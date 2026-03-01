package gamesimport

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPelicanEggConfig_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantConfig PelicanEggConfig
		wantError  string
	}{
		{
			name: "config_fields_as_strings",
			input: `{
				"files": "{}",
				"startup": "{\"done\": \"Bot logged in as \"}",
				"logs": "{}",
				"stop": "^C"
			}`,
			wantConfig: PelicanEggConfig{
				Stop: "^C",
				Startup: PelicanEggConfigStartup{
					Done: "Bot logged in as ",
				},
				Files: map[string]PelicanEggConfigFile{},
			},
		},
		{
			name: "config_fields_as_objects",
			input: `{
				"files": {},
				"startup": {"done": "Server started"},
				"logs": {},
				"stop": "stop"
			}`,
			wantConfig: PelicanEggConfig{
				Stop: "stop",
				Startup: PelicanEggConfigStartup{
					Done: "Server started",
				},
				Files: map[string]PelicanEggConfigFile{},
			},
		},
		{
			name: "config_with_complex_files_as_string",
			input: `{
				"files": "{\"config.json\": {\"parser\": \"json\", \"find\": {\"server.port\": \"{{server.build.default.port}}\"}}}"
			}`,
			wantConfig: PelicanEggConfig{
				Files: map[string]PelicanEggConfigFile{
					"config.json": {
						Parser: "json",
						Find: map[string]any{
							"server.port": "{{server.build.default.port}}",
						},
					},
				},
			},
		},
		{
			name: "config_with_windows_newlines",
			input: `{
				"files": "{}",
				"startup": "{\r\n    \"done\": \"Bot logged in as \"\r\n}",
				"logs": "{}",
				"stop": "^C"
			}`,
			wantConfig: PelicanEggConfig{
				Stop: "^C",
				Startup: PelicanEggConfigStartup{
					Done: "Bot logged in as ",
				},
				Files: map[string]PelicanEggConfigFile{},
			},
		},
		{
			name: "config_with_complex_files_as_object",
			input: `{
				"files": {
					"server.properties": {
						"parser": "properties",
						"find": {
							"server-port": "{{server.build.default.port}}"
						}
					}
				},
				"startup": {"done": "Done"},
				"stop": "stop"
			}`,
			wantConfig: PelicanEggConfig{
				Stop: "stop",
				Startup: PelicanEggConfigStartup{
					Done: "Done",
				},
				Files: map[string]PelicanEggConfigFile{
					"server.properties": {
						Parser: "properties",
						Find: map[string]any{
							"server-port": "{{server.build.default.port}}",
						},
					},
				},
			},
		},
		{
			name: "config_with_empty_strings",
			input: `{
				"files": "",
				"startup": "",
				"logs": "",
				"stop": "quit"
			}`,
			wantConfig: PelicanEggConfig{
				Stop:  "quit",
				Files: map[string]PelicanEggConfigFile{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config PelicanEggConfig
			err := json.Unmarshal([]byte(tt.input), &config)

			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantConfig.Stop, config.Stop)
			assert.Equal(t, tt.wantConfig.Startup.Done, config.Startup.Done)
			assert.Equal(t, len(tt.wantConfig.Files), len(config.Files))

			for key, wantFile := range tt.wantConfig.Files {
				gotFile, ok := config.Files[key]
				require.True(t, ok, "expected file %s to exist", key)
				assert.Equal(t, wantFile.Parser, gotFile.Parser)
			}
		})
	}
}

func TestParsePelicanEgg_WithStringConfigFields(t *testing.T) {
	input := `{
		"meta": {
			"version": "1.0",
			"update_url": "https://example.com",
			"exported_at": "2024-01-01"
		},
		"uuid": "test-uuid",
		"author": "test@example.com",
		"name": "Test Game",
		"description": "Test description",
		"features": [],
		"docker_images": {
			"ghcr.io/test/image:latest": "ghcr.io/test/image:latest"
		},
		"file_denylist": [],
		"startup": "./start.sh",
		"config": {
			"files": "{}",
			"startup": "{\r\n    \"done\": \"Server started\"\r\n}",
			"logs": "{}",
			"stop": "^C"
		},
		"scripts": {
			"installation": {
				"script": "#!/bin/bash",
				"container": "alpine",
				"entrypoint": "bash"
			}
		},
		"variables": []
	}`

	egg, err := ParsePelicanEgg([]byte(input))
	require.NoError(t, err)

	assert.Equal(t, "test-uuid", egg.UUID)
	assert.Equal(t, "Test Game", egg.Name)
	assert.Equal(t, "^C", egg.Config.Stop)
	assert.Equal(t, "Server started", egg.Config.Startup.Done)

	require.NotNil(t, egg.Raw)
	assert.Equal(t, "test-uuid", egg.Raw["uuid"])
	assert.Equal(t, "Test Game", egg.Raw["name"])
}

func TestParsePelicanEgg_RawPreservesUnknownFields(t *testing.T) {
	input := `{
		"_comment": "DO NOT EDIT: FILE GENERATED AUTOMATICALLY BY PTERODACTYL PANEL",
		"meta": {
			"version": "PTDL_v2",
			"update_url": null
		},
		"exported_at": "2024-04-02T14:13:31+02:00",
		"uuid": "preserve-uuid",
		"name": "Test Preserve",
		"author": "test@example.com",
		"description": "Test description",
		"features": ["feature1"],
		"docker_images": {
			"Nodejs 18": "ghcr.io/parkervcp/yolks:nodejs_18"
		},
		"file_denylist": ["secret.txt"],
		"startup": "./start.sh",
		"config": {},
		"scripts": {
			"installation": {
				"script": "#!/bin/bash",
				"container": "alpine",
				"entrypoint": "bash"
			}
		},
		"variables": []
	}`

	egg, err := ParsePelicanEgg([]byte(input))
	require.NoError(t, err)

	require.NotNil(t, egg.Raw)
	assert.Equal(t, "DO NOT EDIT: FILE GENERATED AUTOMATICALLY BY PTERODACTYL PANEL", egg.Raw["_comment"])
	assert.Equal(t, "2024-04-02T14:13:31+02:00", egg.Raw["exported_at"])
	assert.Equal(t, "preserve-uuid", egg.Raw["uuid"])

	meta, ok := egg.Raw["meta"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "PTDL_v2", meta["version"])

	dockerImages, ok := egg.Raw["docker_images"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ghcr.io/parkervcp/yolks:nodejs_18", dockerImages["Nodejs 18"])

	features, ok := egg.Raw["features"].([]any)
	require.True(t, ok)
	require.Len(t, features, 1)
	assert.Equal(t, "feature1", features[0])

	fileDenylist, ok := egg.Raw["file_denylist"].([]any)
	require.True(t, ok)
	require.Len(t, fileDenylist, 1)
	assert.Equal(t, "secret.txt", fileDenylist[0])
}

func TestParsePelicanEgg_WithObjectConfigFields(t *testing.T) {
	input := `{
		"meta": {
			"version": "1.0",
			"update_url": "https://example.com",
			"exported_at": "2024-01-01"
		},
		"uuid": "test-uuid-2",
		"author": "test@example.com",
		"name": "Test Game 2",
		"description": "Test description",
		"features": [],
		"docker_images": {},
		"file_denylist": [],
		"startup": "./start.sh",
		"config": {
			"files": {
				"config.yml": {
					"parser": "yaml",
					"find": {
						"port": "{{server.build.default.port}}"
					}
				}
			},
			"startup": {
				"done": "Ready!"
			},
			"logs": {},
			"stop": "stop"
		},
		"scripts": {
			"installation": {
				"script": "#!/bin/bash",
				"container": "alpine",
				"entrypoint": "bash"
			}
		},
		"variables": []
	}`

	egg, err := ParsePelicanEgg([]byte(input))
	require.NoError(t, err)

	assert.Equal(t, "test-uuid-2", egg.UUID)
	assert.Equal(t, "stop", egg.Config.Stop)
	assert.Equal(t, "Ready!", egg.Config.Startup.Done)
	require.Len(t, egg.Config.Files, 1)
	assert.Equal(t, "yaml", egg.Config.Files["config.yml"].Parser)
}
