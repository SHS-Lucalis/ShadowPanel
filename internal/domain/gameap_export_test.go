package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGameExport(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantExport *GameExport
		wantError  string
	}{
		{
			name: "valid_minimal_export",
			input: `
schema_version: "1.0"
game:
  code: "cstrike"
  name: "Counter-Strike 1.6"
  engine: "GoldSource"
`,
			wantExport: &GameExport{
				SchemaVersion: "1.0",
				Game: GameExportGame{
					Code:   "cstrike",
					Name:   "Counter-Strike 1.6",
					Engine: "GoldSource",
				},
			},
		},
		{
			name: "valid_full_export",
			input: `
schema_version: "1.0"
exported_at: "2024-01-15T10:30:00Z"
exported_by: "GameAP v3.0.0"
game:
  code: "cstrike"
  name: "Counter-Strike 1.6"
  engine: "GoldSource"
  engine_version: "1.0"
  steam_app_id_linux: 90
  steam_app_id_windows: 90
  steam_app_set_config: "mod cstrike"
  remote_repository_linux: "http://example.com/linux"
  remote_repository_windows: "http://example.com/windows"
  metadata:
    custom_key: "custom_value"
mods:
  - name: "Classic"
    start_cmd_linux: "./hlds_run -game cstrike +port {port}"
    start_cmd_windows: "hlds.exe -game cstrike +port {port}"
    kick_cmd: "kick {name}"
    ban_cmd: "banid 0 {name} kick"
    srestart_cmd: "restart"
    chmap_cmd: "changelevel {map}"
    sendmsg_cmd: "say {msg}"
    passwd_cmd: "sv_password {password}"
    fast_rcon:
      - info: "Restart Map"
        command: "changelevel {map}"
    vars:
      - var: "maxplayers"
        default: "32"
        info: "Maximum players"
        admin_var: false
`,
			wantExport: &GameExport{
				SchemaVersion: "1.0",
				ExportedAt:    "2024-01-15T10:30:00Z",
				ExportedBy:    "GameAP v3.0.0",
				Game: GameExportGame{
					Code:                    "cstrike",
					Name:                    "Counter-Strike 1.6",
					Engine:                  "GoldSource",
					EngineVersion:           "1.0",
					SteamAppIDLinux:         new(uint(90)),
					SteamAppIDWindows:       new(uint(90)),
					SteamAppSetConfig:       new("mod cstrike"),
					RemoteRepositoryLinux:   new("http://example.com/linux"),
					RemoteRepositoryWindows: new("http://example.com/windows"),
					Metadata: Metadata{
						"custom_key": "custom_value",
					},
				},
				Mods: []GameExportMod{
					{
						Name:            "Classic",
						StartCmdLinux:   new("./hlds_run -game cstrike +port {port}"),
						StartCmdWindows: new("hlds.exe -game cstrike +port {port}"),
						KickCmd:         new("kick {name}"),
						BanCmd:          new("banid 0 {name} kick"),
						SrestartCmd:     new("restart"),
						ChmapCmd:        new("changelevel {map}"),
						SendmsgCmd:      new("say {msg}"),
						PasswdCmd:       new("sv_password {password}"),
						FastRcon: []GameExportModFastRcon{
							{Info: "Restart Map", Command: "changelevel {map}"},
						},
						Vars: []GameExportModVar{
							{Var: "maxplayers", Default: "32", Info: "Maximum players", AdminVar: false},
						},
					},
				},
			},
		},
		{
			name: "valid_export_without_mods",
			input: `
schema_version: "1.0"
game:
  code: "test"
  name: "Test Game"
  engine: "Test Engine"
mods: []
`,
			wantExport: &GameExport{
				SchemaVersion: "1.0",
				Game: GameExportGame{
					Code:   "test",
					Name:   "Test Game",
					Engine: "Test Engine",
				},
				Mods: []GameExportMod{},
			},
		},
		{
			name: "invalid_yaml",
			input: `
schema_version: "1.0"
game:
  code: "test
  name: "Test Game"
`,
			wantError: "failed to parse GameAP YAML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			export, err := ParseGameExport([]byte(tt.input))

			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantExport.SchemaVersion, export.SchemaVersion)
			assert.Equal(t, tt.wantExport.Game.Code, export.Game.Code)
			assert.Equal(t, tt.wantExport.Game.Name, export.Game.Name)
			assert.Equal(t, tt.wantExport.Game.Engine, export.Game.Engine)
			require.Len(t, export.Mods, len(tt.wantExport.Mods))
		})
	}
}

func TestGameExport_Validate(t *testing.T) {
	tests := []struct {
		name      string
		export    *GameExport
		wantError string
	}{
		{
			name: "valid_minimal",
			export: &GameExport{
				SchemaVersion: "1.0",
				Game: GameExportGame{
					Code:   "test",
					Name:   "Test Game",
					Engine: "Test Engine",
				},
			},
		},
		{
			name: "valid_with_mods",
			export: &GameExport{
				SchemaVersion: "1.0",
				Game: GameExportGame{
					Code:   "test",
					Name:   "Test Game",
					Engine: "Test Engine",
				},
				Mods: []GameExportMod{
					{Name: "Mod1"},
					{Name: "Mod2"},
				},
			},
		},
		{
			name: "missing_schema_version",
			export: &GameExport{
				Game: GameExportGame{
					Code:   "test",
					Name:   "Test Game",
					Engine: "Test Engine",
				},
			},
			wantError: "schema_version is required",
		},
		{
			name: "unsupported_schema_version",
			export: &GameExport{
				SchemaVersion: "2.0",
				Game: GameExportGame{
					Code:   "test",
					Name:   "Test Game",
					Engine: "Test Engine",
				},
			},
			wantError: "unsupported schema version",
		},
		{
			name: "missing_game_code",
			export: &GameExport{
				SchemaVersion: "1.0",
				Game: GameExportGame{
					Name:   "Test Game",
					Engine: "Test Engine",
				},
			},
			wantError: "game.code is required",
		},
		{
			name: "game_code_too_short",
			export: &GameExport{
				SchemaVersion: "1.0",
				Game: GameExportGame{
					Code:   "t",
					Name:   "Test Game",
					Engine: "Test Engine",
				},
			},
			wantError: "game.code must be between 2 and 16 characters",
		},
		{
			name: "game_code_too_long",
			export: &GameExport{
				SchemaVersion: "1.0",
				Game: GameExportGame{
					Code:   "this_code_is_way_too_long",
					Name:   "Test Game",
					Engine: "Test Engine",
				},
			},
			wantError: "game.code must be between 2 and 16 characters",
		},
		{
			name: "game_code_invalid_format",
			export: &GameExport{
				SchemaVersion: "1.0",
				Game: GameExportGame{
					Code:   "Test Game",
					Name:   "Test Game",
					Engine: "Test Engine",
				},
			},
			wantError: "game.code must match pattern",
		},
		{
			name: "game_code_with_uppercase",
			export: &GameExport{
				SchemaVersion: "1.0",
				Game: GameExportGame{
					Code:   "TestGame",
					Name:   "Test Game",
					Engine: "Test Engine",
				},
			},
			wantError: "game.code must match pattern",
		},
		{
			name: "missing_game_name",
			export: &GameExport{
				SchemaVersion: "1.0",
				Game: GameExportGame{
					Code:   "test",
					Engine: "Test Engine",
				},
			},
			wantError: "game.name is required",
		},
		{
			name: "game_name_too_short",
			export: &GameExport{
				SchemaVersion: "1.0",
				Game: GameExportGame{
					Code:   "test",
					Name:   "T",
					Engine: "Test Engine",
				},
			},
			wantError: "game.name must be between 2 and 128 characters",
		},
		{
			name: "missing_game_engine",
			export: &GameExport{
				SchemaVersion: "1.0",
				Game: GameExportGame{
					Code: "test",
					Name: "Test Game",
				},
			},
			wantError: "game.engine is required",
		},
		{
			name: "missing_mod_name",
			export: &GameExport{
				SchemaVersion: "1.0",
				Game: GameExportGame{
					Code:   "test",
					Name:   "Test Game",
					Engine: "Test Engine",
				},
				Mods: []GameExportMod{
					{Name: ""},
				},
			},
			wantError: "mods[0].name is required",
		},
		{
			name: "duplicate_mod_names",
			export: &GameExport{
				SchemaVersion: "1.0",
				Game: GameExportGame{
					Code:   "test",
					Name:   "Test Game",
					Engine: "Test Engine",
				},
				Mods: []GameExportMod{
					{Name: "Classic"},
					{Name: "Classic"},
				},
			},
			wantError: "duplicate mod name: Classic",
		},
		{
			name: "start_cmd_linux_too_long",
			export: &GameExport{
				SchemaVersion: "1.0",
				Game: GameExportGame{
					Code:   "test",
					Name:   "Test Game",
					Engine: "Test Engine",
				},
				Mods: []GameExportMod{
					{
						Name:          "Mod1",
						StartCmdLinux: new(string(make([]byte, 1001))),
					},
				},
			},
			wantError: "mods[0].start_cmd_linux must be at most 1000 characters",
		},
		{
			name: "kick_cmd_too_long",
			export: &GameExport{
				SchemaVersion: "1.0",
				Game: GameExportGame{
					Code:   "test",
					Name:   "Test Game",
					Engine: "Test Engine",
				},
				Mods: []GameExportMod{
					{
						Name:    "Mod1",
						KickCmd: new(string(make([]byte, 201))),
					},
				},
			},
			wantError: "mods[0].kick_cmd must be at most 200 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.export.Validate()

			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestGameExportGame_ToDomainGame(t *testing.T) {
	exportGame := &GameExportGame{
		Code:                    "cstrike",
		Name:                    "Counter-Strike 1.6",
		Engine:                  "GoldSource",
		EngineVersion:           "1.0",
		SteamAppIDLinux:         new(uint(90)),
		SteamAppIDWindows:       new(uint(90)),
		SteamAppSetConfig:       new("mod cstrike"),
		RemoteRepositoryLinux:   new("http://example.com/linux"),
		RemoteRepositoryWindows: new("http://example.com/windows"),
		LocalRepositoryLinux:    new("/local/linux"),
		LocalRepositoryWindows:  new("C:\\local\\windows"),
		Metadata: Metadata{
			"custom": "value",
		},
	}

	game := exportGame.ToDomainGame()

	assert.Equal(t, "cstrike", game.Code)
	assert.Equal(t, "Counter-Strike 1.6", game.Name)
	assert.Equal(t, "GoldSource", game.Engine)
	assert.Equal(t, "1.0", game.EngineVersion)
	assert.Equal(t, uint(90), *game.SteamAppIDLinux)
	assert.Equal(t, uint(90), *game.SteamAppIDWindows)
	assert.Equal(t, "mod cstrike", *game.SteamAppSetConfig)
	assert.Equal(t, "http://example.com/linux", *game.RemoteRepositoryLinux)
	assert.Equal(t, "http://example.com/windows", *game.RemoteRepositoryWindows)
	assert.Equal(t, "/local/linux", *game.LocalRepositoryLinux)
	assert.Equal(t, "C:\\local\\windows", *game.LocalRepositoryWindows)
	assert.Equal(t, 1, game.Enabled)
	assert.Equal(t, "value", game.Metadata["custom"])
}

func TestGameExportMod_ToDomainGameMod(t *testing.T) {
	exportMod := &GameExportMod{
		Name: "Classic",
		FastRcon: []GameExportModFastRcon{
			{Info: "Restart", Command: "restart"},
		},
		Vars: []GameExportModVar{
			{Var: "maxplayers", Default: "32", Info: "Max players", AdminVar: false},
		},
		RemoteRepositoryLinux:   new("http://mod.com/linux"),
		RemoteRepositoryWindows: new("http://mod.com/windows"),
		LocalRepositoryLinux:    new("/mod/linux"),
		LocalRepositoryWindows:  new("C:\\mod\\windows"),
		StartCmdLinux:           new("./start.sh"),
		StartCmdWindows:         new("start.exe"),
		KickCmd:                 new("kick {name}"),
		BanCmd:                  new("ban {name}"),
		ChnameCmd:               new("rename {name}"),
		SrestartCmd:             new("restart"),
		ChmapCmd:                new("changelevel {map}"),
		SendmsgCmd:              new("say {msg}"),
		PasswdCmd:               new("password {pass}"),
		Metadata: Metadata{
			"mod_key": "mod_value",
		},
	}

	gameMod := exportMod.ToDomainGameMod("cstrike")

	assert.Equal(t, "cstrike", gameMod.GameCode)
	assert.Equal(t, "Classic", gameMod.Name)
	require.Len(t, gameMod.FastRcon, 1)
	assert.Equal(t, "Restart", gameMod.FastRcon[0].Info)
	assert.Equal(t, "restart", gameMod.FastRcon[0].Command)
	require.Len(t, gameMod.Vars, 1)
	assert.Equal(t, "maxplayers", gameMod.Vars[0].Var)
	assert.Equal(t, GameModVarDefault("32"), gameMod.Vars[0].Default)
	assert.Equal(t, "http://mod.com/linux", *gameMod.RemoteRepositoryLinux)
	assert.Equal(t, "./start.sh", *gameMod.StartCmdLinux)
	assert.Equal(t, "kick {name}", *gameMod.KickCmd)
	assert.Equal(t, "mod_value", gameMod.Metadata["mod_key"])
}

func TestNewGameExportFromDomain(t *testing.T) {
	game := &Game{
		Code:                    "cstrike",
		Name:                    "Counter-Strike 1.6",
		Engine:                  "GoldSource",
		EngineVersion:           "1.0",
		SteamAppIDLinux:         new(uint(90)),
		SteamAppIDWindows:       new(uint(90)),
		SteamAppSetConfig:       new("mod cstrike"),
		RemoteRepositoryLinux:   new("http://example.com/linux"),
		RemoteRepositoryWindows: new("http://example.com/windows"),
		Enabled:                 1,
		Metadata: Metadata{
			"custom":      "value",
			"pelican_egg": map[string]any{"should": "be excluded"},
		},
	}

	mods := []GameMod{
		{
			ID:       1,
			GameCode: "cstrike",
			Name:     "Classic",
			FastRcon: GameModFastRconList{
				{Info: "Restart", Command: "restart"},
			},
			Vars: GameModVarList{
				{Var: "maxplayers", Default: "32", Info: "Max players"},
			},
			StartCmdLinux: new("./hlds_run"),
			KickCmd:       new("kick {name}"),
			Metadata: Metadata{
				"pelican_egg": map[string]any{"should": "be excluded"},
			},
		},
	}

	export := NewGameExportFromDomain(game, mods, "v3.0.0")

	assert.Equal(t, CurrentSchemaVersion, export.SchemaVersion)
	assert.NotEmpty(t, export.ExportedAt)
	assert.Equal(t, "GameAP v3.0.0", export.ExportedBy)

	assert.Equal(t, "cstrike", export.Game.Code)
	assert.Equal(t, "Counter-Strike 1.6", export.Game.Name)
	assert.Equal(t, "GoldSource", export.Game.Engine)
	assert.Equal(t, uint(90), *export.Game.SteamAppIDLinux)
	assert.Equal(t, "value", export.Game.Metadata["custom"])
	assert.NotNil(t, export.Game.Metadata["pelican_egg"])
	require.Len(t, export.Mods, 1)
	assert.Equal(t, "Classic", export.Mods[0].Name)
	assert.Equal(t, "./hlds_run", *export.Mods[0].StartCmdLinux)
	require.Len(t, export.Mods[0].FastRcon, 1)
	require.Len(t, export.Mods[0].Vars, 1)
	assert.NotNil(t, export.Mods[0].Metadata["pelican_egg"])
}

func TestGameExport_ToYAML(t *testing.T) {
	export := &GameExport{
		SchemaVersion: "1.0",
		ExportedAt:    "2024-01-15T10:30:00Z",
		ExportedBy:    "GameAP v3.0.0",
		Game: GameExportGame{
			Code:   "test",
			Name:   "Test Game",
			Engine: "Test Engine",
		},
		Mods: []GameExportMod{
			{
				Name:          "Default",
				StartCmdLinux: new("./start.sh"),
			},
		},
	}

	yamlData, err := export.ToYAML()
	require.NoError(t, err)

	parsed, err := ParseGameExport(yamlData)
	require.NoError(t, err)

	assert.Equal(t, export.SchemaVersion, parsed.SchemaVersion)
	assert.Equal(t, export.Game.Code, parsed.Game.Code)
	assert.Equal(t, export.Game.Name, parsed.Game.Name)
	require.Len(t, parsed.Mods, 1)
	assert.Equal(t, "Default", parsed.Mods[0].Name)
	assert.Equal(t, "./start.sh", *parsed.Mods[0].StartCmdLinux)
}
