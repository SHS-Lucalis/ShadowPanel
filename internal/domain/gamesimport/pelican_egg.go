package gamesimport

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// PelicanEgg represents a Pelican/Pterodactyl egg configuration.
// Eggs define how game servers are installed, configured, and run within the panel.
type PelicanEgg struct {
	Meta         PelicanEggMeta       `json:"meta"`
	UUID         string               `json:"uuid"`
	Author       string               `json:"author"`
	Name         string               `json:"name"`
	Description  string               `json:"description"`
	Features     []string             `json:"features"`
	DockerImages map[string]string    `json:"docker_images"`
	FileDenylist []string             `json:"file_denylist"`
	Startup      string               `json:"startup"`
	Config       PelicanEggConfig     `json:"config"`
	Scripts      PelicanEggScripts    `json:"scripts"`
	Variables    []PelicanEggVariable `json:"variables"`
}

type PelicanEggMeta struct {
	Version    string `json:"version"`
	UpdateURL  string `json:"update_url"`
	ExportedAt string `json:"exported_at"`
}

type PelicanEggConfig struct {
	Files   map[string]PelicanEggConfigFile `json:"files"`
	Startup PelicanEggConfigStartup         `json:"startup"`
	Stop    string                          `json:"stop"`
	Logs    any                             `json:"logs"`
}

type PelicanEggConfigFile struct {
	Parser string         `json:"parser"`
	Find   map[string]any `json:"find"`
}

type PelicanEggConfigStartup struct {
	Done            string   `json:"done"`
	UserInteraction []string `json:"userInteraction"`
}

// UnmarshalJSON handles Pelican Egg JSON where files, startup, and logs
// can be either JSON strings containing serialized JSON or direct JSON objects.
func (c *PelicanEggConfig) UnmarshalJSON(data []byte) error {
	type rawConfig struct {
		Files   json.RawMessage `json:"files"`
		Startup json.RawMessage `json:"startup"`
		Stop    string          `json:"stop"`
		Logs    json.RawMessage `json:"logs"`
	}

	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return errors.Wrap(err, "failed to unmarshal PelicanEggConfig")
	}

	c.Stop = raw.Stop

	if len(raw.Files) > 0 {
		c.Files = make(map[string]PelicanEggConfigFile)
		if err := unmarshalFlexibleJSON(raw.Files, &c.Files); err != nil {
			return errors.Wrap(err, "failed to unmarshal config.files")
		}
	}

	if len(raw.Startup) > 0 {
		if err := unmarshalFlexibleJSON(raw.Startup, &c.Startup); err != nil {
			return errors.Wrap(err, "failed to unmarshal config.startup")
		}
	}

	if len(raw.Logs) > 0 {
		if err := unmarshalFlexibleJSON(raw.Logs, &c.Logs); err != nil {
			return errors.Wrap(err, "failed to unmarshal config.logs")
		}
	}

	return nil
}

// unmarshalFlexibleJSON handles JSON fields that can be either a string
// containing JSON or a direct JSON object.
func unmarshalFlexibleJSON(data []byte, target any) error {
	if len(data) == 0 {
		return nil
	}

	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		if str == "" || str == "{}" {
			return nil
		}

		return json.Unmarshal([]byte(str), target)
	}

	return json.Unmarshal(data, target)
}

type PelicanEggScripts struct {
	Installation PelicanEggInstallationScript `json:"installation"`
}

type PelicanEggInstallationScript struct {
	Script     string `json:"script"`
	Container  string `json:"container"`
	Entrypoint string `json:"entrypoint"`
}

type PelicanEggVariable struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	EnvVariable  string `json:"env_variable"`
	DefaultValue string `json:"default_value"`
	UserViewable bool   `json:"user_viewable"`
	UserEditable bool   `json:"user_editable"`
	Rules        string `json:"rules"`
	FieldType    string `json:"field_type"`
}

// ParsePelicanEgg parses JSON data into a PelicanEgg struct.
func ParsePelicanEgg(data []byte) (*PelicanEgg, error) {
	var egg PelicanEgg
	if err := json.Unmarshal(data, &egg); err != nil {
		return nil, errors.Wrap(err, "failed to parse pelican egg JSON")
	}

	return &egg, nil
}

// FirstDockerImage returns the first docker image key from the egg configuration.
func (e *PelicanEgg) FirstDockerImage() string {
	for key := range e.DockerImages {
		return key
	}

	return ""
}
