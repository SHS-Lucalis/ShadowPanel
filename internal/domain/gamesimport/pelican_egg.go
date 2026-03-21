package gamesimport

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/goccy/go-yaml"
	"github.com/pkg/errors"
)

// PelicanEgg represents a Pelican/Pterodactyl egg configuration.
// Eggs define how game servers are installed, configured, and run within the panel.
type PelicanEgg struct {
	Meta            PelicanEggMeta       `json:"meta" yaml:"meta"`
	UUID            string               `json:"uuid" yaml:"uuid"`
	Author          string               `json:"author" yaml:"author"`
	Name            string               `json:"name" yaml:"name"`
	Description     string               `json:"description" yaml:"description"`
	Features        []string             `json:"features" yaml:"features"`
	DockerImages    map[string]string    `json:"docker_images" yaml:"docker_images"`
	FileDenylist    FlexibleStringSlice  `json:"file_denylist" yaml:"file_denylist"`
	Startup         string               `json:"startup" yaml:"startup"`
	StartupCommands map[string]string    `json:"startup_commands" yaml:"startup_commands"`
	Config          PelicanEggConfig     `json:"config" yaml:"config"`
	Scripts         PelicanEggScripts    `json:"scripts" yaml:"scripts"`
	Variables       []PelicanEggVariable `json:"variables" yaml:"variables"`

	// Raw contains the original data as a map for metadata storage.
	// This preserves all fields including unknown ones like _comment.
	Raw map[string]any `json:"-" yaml:"-"`
}

// GetStartupCommand returns the startup command, preferring startup_commands["Default"]
// (PLCN_v3 format) over the legacy startup field.
func (e *PelicanEgg) GetStartupCommand() string {
	if cmd, ok := e.StartupCommands["Default"]; ok && cmd != "" {
		return cmd
	}

	return e.Startup
}

type PelicanEggMeta struct {
	Version    string `json:"version" yaml:"version"`
	UpdateURL  string `json:"update_url" yaml:"update_url"`
	ExportedAt string `json:"exported_at" yaml:"exported_at"`
}

type PelicanEggConfig struct {
	Files   map[string]PelicanEggConfigFile `json:"files" yaml:"files"`
	Startup PelicanEggConfigStartup         `json:"startup" yaml:"startup"`
	Stop    string                          `json:"stop" yaml:"stop"`
	Logs    any                             `json:"logs" yaml:"logs"`
}

type PelicanEggConfigFile struct {
	Parser string         `json:"parser" yaml:"parser"`
	Find   map[string]any `json:"find" yaml:"find"`
}

type PelicanEggConfigStartup struct {
	Done            string   `json:"done" yaml:"done"`
	UserInteraction []string `json:"userInteraction" yaml:"userInteraction"`
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

// UnmarshalYAML handles YAML format where files, startup, and logs are always direct objects.
func (c *PelicanEggConfig) UnmarshalYAML(unmarshal func(any) error) error {
	type rawConfig struct {
		Files   map[string]PelicanEggConfigFile `yaml:"files"`
		Startup PelicanEggConfigStartup         `yaml:"startup"`
		Stop    string                          `yaml:"stop"`
		Logs    any                             `yaml:"logs"`
	}

	var raw rawConfig
	if err := unmarshal(&raw); err != nil {
		return errors.Wrap(err, "failed to unmarshal PelicanEggConfig")
	}

	c.Files = raw.Files
	c.Startup = raw.Startup
	c.Stop = raw.Stop
	c.Logs = raw.Logs

	return nil
}

type PelicanEggScripts struct {
	Installation PelicanEggInstallationScript `json:"installation" yaml:"installation"`
}

type PelicanEggInstallationScript struct {
	Script     string `json:"script" yaml:"script"`
	Container  string `json:"container" yaml:"container"`
	Entrypoint string `json:"entrypoint" yaml:"entrypoint"`
}

type PelicanEggVariable struct {
	Name         string        `json:"name" yaml:"name"`
	Description  string        `json:"description" yaml:"description"`
	EnvVariable  string        `json:"env_variable" yaml:"env_variable"`
	DefaultValue string        `json:"default_value" yaml:"default_value"`
	UserViewable bool          `json:"user_viewable" yaml:"user_viewable"`
	UserEditable bool          `json:"user_editable" yaml:"user_editable"`
	Rules        FlexibleRules `json:"rules" yaml:"rules"`
	FieldType    string        `json:"field_type" yaml:"field_type"`
}

// FlexibleRules handles both string and array formats for rules field.
// Legacy format: "required|string|max:64".
// PLCN_v3 format: ["required", "string", "max:64"].
type FlexibleRules []string

func (r *FlexibleRules) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		if str == "" {
			*r = []string{}
		} else {
			*r = []string{str}
		}

		return nil
	}

	var arr []string
	if err := json.Unmarshal(data, &arr); err != nil {
		return errors.Wrap(err, "rules must be string or array of strings")
	}

	*r = arr

	return nil
}

func (r *FlexibleRules) UnmarshalYAML(unmarshal func(any) error) error {
	var arr []string
	if err := unmarshal(&arr); err == nil {
		*r = arr

		return nil
	}

	var str string
	if err := unmarshal(&str); err != nil {
		return errors.Wrap(err, "rules must be string or array of strings")
	}

	if str == "" {
		*r = []string{}
	} else {
		*r = []string{str}
	}

	return nil
}

// FlexibleStringSlice handles YAML fields that can be either an array or an empty object {}.
// In YAML format, file_denylist can be: [] (array) or {} (empty object).
type FlexibleStringSlice []string

func (f *FlexibleStringSlice) UnmarshalJSON(data []byte) error {
	var arr []string
	if err := json.Unmarshal(data, &arr); err != nil {
		return errors.Wrap(err, "file_denylist must be an array of strings")
	}

	*f = arr

	return nil
}

func (f *FlexibleStringSlice) UnmarshalYAML(unmarshal func(any) error) error {
	var arr []string
	if err := unmarshal(&arr); err == nil {
		*f = arr

		return nil
	}

	var obj map[string]any
	if err := unmarshal(&obj); err != nil {
		return errors.Wrap(err, "file_denylist must be an array of strings or empty object")
	}

	*f = []string{}

	return nil
}

// detectFormat determines if data is JSON or YAML based on content.
// JSON starts with '{', otherwise it's treated as YAML.
func detectFormat(data []byte) string {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		return "json"
	}

	return "yaml"
}

// ParsePelicanEgg parses JSON or YAML data into a PelicanEgg struct.
// The format is auto-detected based on content.
func ParsePelicanEgg(data []byte) (*PelicanEgg, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.Wrap(io.EOF, "empty input data")
	}

	switch detectFormat(data) {
	case "json":
		return parsePelicanEggJSON(data)
	default:
		return parsePelicanEggYAML(data)
	}
}

func parsePelicanEggJSON(data []byte) (*PelicanEgg, error) {
	var egg PelicanEgg
	if err := json.Unmarshal(data, &egg); err != nil {
		return nil, errors.Wrap(err, "failed to parse pelican egg JSON")
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, errors.Wrap(err, "failed to parse pelican egg raw JSON")
	}
	egg.Raw = raw

	return &egg, nil
}

func parsePelicanEggYAML(data []byte) (*PelicanEgg, error) {
	var egg PelicanEgg
	if err := yaml.Unmarshal(data, &egg); err != nil {
		return nil, errors.Wrap(err, "failed to parse pelican egg YAML")
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, errors.Wrap(err, "failed to parse pelican egg raw YAML")
	}
	egg.Raw = raw

	return &egg, nil
}

// FirstDockerImage returns the first docker image key from the egg configuration.
func (e *PelicanEgg) FirstDockerImage() string {
	for _, val := range e.DockerImages {
		return val
	}

	return ""
}
