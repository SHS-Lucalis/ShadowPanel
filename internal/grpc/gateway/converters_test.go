package gateway

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestDomainServerToProtoWithGameMod(t *testing.T) {
	linuxCmd := "./start.sh -game cstrike"
	windowsCmd := "start.exe -game cstrike"

	tests := []struct {
		name             string
		serverStartCmd   *string
		gameMod          *domain.GameMod
		nodeOS           domain.NodeOS
		wantStartCommand *string
	}{
		{
			name:           "nil_server_command_uses_linux_game_mod_command",
			serverStartCmd: nil,
			gameMod: &domain.GameMod{
				StartCmdLinux:   &linuxCmd,
				StartCmdWindows: &windowsCmd,
			},
			nodeOS:           domain.NodeOSLinux,
			wantStartCommand: &linuxCmd,
		},
		{
			name:           "nil_server_command_uses_windows_game_mod_command",
			serverStartCmd: nil,
			gameMod: &domain.GameMod{
				StartCmdLinux:   &linuxCmd,
				StartCmdWindows: &windowsCmd,
			},
			nodeOS:           domain.NodeOSWindows,
			wantStartCommand: &windowsCmd,
		},
		{
			name:           "non_nil_server_command_preserved",
			serverStartCmd: &linuxCmd,
			gameMod: &domain.GameMod{
				StartCmdLinux: new("other-command"),
			},
			nodeOS:           domain.NodeOSLinux,
			wantStartCommand: &linuxCmd,
		},
		{
			name:           "empty_server_command_uses_game_mod_command",
			serverStartCmd: new(""),
			gameMod: &domain.GameMod{
				StartCmdLinux: &linuxCmd,
			},
			nodeOS:           domain.NodeOSLinux,
			wantStartCommand: &linuxCmd,
		},
		{
			name:             "nil_game_mod_returns_nil_command",
			serverStartCmd:   nil,
			gameMod:          nil,
			nodeOS:           domain.NodeOSLinux,
			wantStartCommand: nil,
		},
		{
			name:           "nil_server_command_and_nil_game_mod_command",
			serverStartCmd: nil,
			gameMod: &domain.GameMod{
				StartCmdLinux: nil,
			},
			nodeOS:           domain.NodeOSLinux,
			wantStartCommand: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &domain.Server{
				ID:           1,
				Name:         "test",
				ServerIP:     "127.0.0.1",
				ServerPort:   27015,
				Dir:          "/srv/server",
				StartCommand: tt.serverStartCmd,
			}

			result := DomainServerToProtoWithGameMod(server, tt.gameMod, tt.nodeOS)

			require.NotNil(t, result)

			if tt.wantStartCommand == nil {
				assert.Nil(t, result.StartCommand)
			} else {
				require.NotNil(t, result.StartCommand)
				assert.Equal(t, *tt.wantStartCommand, *result.StartCommand)
			}
		})
	}
}

func TestDomainServerToProto_minimalServer(t *testing.T) {
	// ARRANGE
	srv := &domain.Server{
		ID:         42,
		UUID:       uuid.MustParse("11111111-2222-3333-4444-555555555555"),
		UUIDShort:  "11111111",
		Enabled:    true,
		Installed:  domain.ServerInstalledStatusInstalled,
		Blocked:    false,
		Name:       "minimal",
		GameID:     "csgo",
		DSID:       7,
		GameModID:  3,
		ServerIP:   "10.0.0.1",
		ServerPort: 27015,
		Dir:        "/srv/csgo",
	}

	// ACT
	got := DomainServerToProto(srv)

	// ASSERT
	require.NotNil(t, got)
	assert.Equal(t, uint64(42), got.Id, "id propagated")
	assert.Equal(t, "11111111-2222-3333-4444-555555555555", got.Uuid)
	assert.Equal(t, "11111111", got.UuidShort)
	assert.True(t, got.Enabled, "enabled flag")
	assert.Equal(t, proto.ServerInstalledStatus_SERVER_INSTALLED_STATUS_INSTALLED, got.Installed)
	assert.False(t, got.Blocked)
	assert.Equal(t, "minimal", got.Name)
	assert.Equal(t, "csgo", got.GameId)
	assert.Equal(t, uint64(7), got.DsId)
	assert.Equal(t, uint64(3), got.GameModId)
	assert.Equal(t, "10.0.0.1", got.ServerIp)
	assert.Equal(t, int32(27015), got.ServerPort)
	assert.Equal(t, "/srv/csgo", got.Dir)
	assert.Nil(t, got.QueryPort, "nil query port stays nil")
	assert.Nil(t, got.RconPort, "nil rcon port stays nil")
	assert.Nil(t, got.CpuLimit, "nil cpu limit stays nil")
	assert.Nil(t, got.RamLimit, "nil ram limit stays nil")
	assert.Nil(t, got.NetLimit, "nil net limit stays nil")
	assert.Nil(t, got.CreatedAt, "nil timestamp stays nil")
	assert.Nil(t, got.UpdatedAt)
	assert.Nil(t, got.DeletedAt)
	assert.Nil(t, got.Expires)
	assert.Nil(t, got.LastProcessCheck)
	assert.Nil(t, got.Vars, "nil server vars stays nil")
}

func TestDomainServerToProto_fullServer(t *testing.T) {
	// ARRANGE
	created := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	updated := time.Date(2024, 1, 3, 3, 4, 5, 0, time.UTC)
	deleted := time.Date(2024, 1, 4, 3, 4, 5, 0, time.UTC)
	expires := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	processCheck := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	srv := &domain.Server{
		ID:               99,
		UUID:             uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"),
		UUIDShort:        "aaaaaaaa",
		Enabled:          true,
		Installed:        domain.ServerInstalledStatusInstallationInProg,
		Blocked:          true,
		Name:             "full server",
		GameID:           "csgo",
		DSID:             5,
		GameModID:        12,
		Expires:          &expires,
		ServerIP:         "192.168.1.1",
		ServerPort:       27015,
		QueryPort:        new(27016),
		RconPort:         new(27017),
		Rcon:             new("rcon-pass"),
		Dir:              "/srv/full",
		SuUser:           new("steam"),
		CPULimit:         new(80),
		RAMLimit:         new(4096),
		NetLimit:         new(1000),
		StartCommand:     new("./start.sh"),
		StopCommand:      new("./stop.sh"),
		ForceStopCommand: new("kill -9"),
		RestartCommand:   new("./restart.sh"),
		ProcessActive:    true,
		LastProcessCheck: &processCheck,
		Vars:             domain.ServerVars{"map": "de_dust2", "tickrate": "128"},
		Metadata:         domain.Metadata{"region": "eu"},
		CreatedAt:        &created,
		UpdatedAt:        &updated,
		DeletedAt:        &deleted,
	}

	// ACT
	got := DomainServerToProto(srv)

	// ASSERT
	require.NotNil(t, got)
	assert.Equal(t, uint64(99), got.Id)
	assert.True(t, got.Blocked)
	assert.Equal(t, proto.ServerInstalledStatus_SERVER_INSTALLED_STATUS_INSTALLATION_IN_PROGRESS, got.Installed)

	require.NotNil(t, got.QueryPort)
	assert.Equal(t, int32(27016), *got.QueryPort)
	require.NotNil(t, got.RconPort)
	assert.Equal(t, int32(27017), *got.RconPort)
	require.NotNil(t, got.Rcon)
	assert.Equal(t, "rcon-pass", *got.Rcon)
	require.NotNil(t, got.SuUser)
	assert.Equal(t, "steam", *got.SuUser)
	require.NotNil(t, got.CpuLimit)
	assert.Equal(t, int32(80), *got.CpuLimit)
	require.NotNil(t, got.RamLimit)
	assert.Equal(t, int32(4096), *got.RamLimit)
	require.NotNil(t, got.NetLimit)
	assert.Equal(t, int32(1000), *got.NetLimit)
	require.NotNil(t, got.StartCommand)
	assert.Equal(t, "./start.sh", *got.StartCommand)
	assert.True(t, got.ProcessActive)

	require.NotNil(t, got.CreatedAt, "created_at propagated")
	assert.Equal(t, created, got.CreatedAt.AsTime())
	require.NotNil(t, got.UpdatedAt)
	assert.Equal(t, updated, got.UpdatedAt.AsTime())
	require.NotNil(t, got.DeletedAt)
	assert.Equal(t, deleted, got.DeletedAt.AsTime())
	require.NotNil(t, got.Expires)
	assert.Equal(t, expires, got.Expires.AsTime())
	require.NotNil(t, got.LastProcessCheck)
	assert.Equal(t, processCheck, got.LastProcessCheck.AsTime())

	require.NotNil(t, got.Vars, "vars must be encoded as JSON string pointer")
	var decoded map[string]string
	require.NoError(t, json.Unmarshal([]byte(*got.Vars), &decoded))
	assert.Equal(t, "de_dust2", decoded["map"])
	assert.Equal(t, "128", decoded["tickrate"])

	require.NotNil(t, got.Metadata)
	regionAny := got.Metadata["region"]
	require.NotNil(t, regionAny)
	wrapped := wrapperspb.String("")
	require.NoError(t, regionAny.UnmarshalTo(wrapped))
	assert.Equal(t, "eu", wrapped.GetValue())
}

func TestDomainServerToProto_installedStatusMapping(t *testing.T) {
	tests := []struct {
		name string
		in   domain.ServerInstalledStatus
		want proto.ServerInstalledStatus
	}{
		{
			name: "not_installed_maps_to_not_installed",
			in:   domain.ServerInstalledStatusNotInstalled,
			want: proto.ServerInstalledStatus_SERVER_INSTALLED_STATUS_NOT_INSTALLED,
		},
		{
			name: "installed_maps_to_installed",
			in:   domain.ServerInstalledStatusInstalled,
			want: proto.ServerInstalledStatus_SERVER_INSTALLED_STATUS_INSTALLED,
		},
		{
			name: "in_progress_maps_to_in_progress",
			in:   domain.ServerInstalledStatusInstallationInProg,
			want: proto.ServerInstalledStatus_SERVER_INSTALLED_STATUS_INSTALLATION_IN_PROGRESS,
		},
		{
			name: "unknown_value_falls_back_to_not_installed",
			in:   domain.ServerInstalledStatus(99),
			want: proto.ServerInstalledStatus_SERVER_INSTALLED_STATUS_NOT_INSTALLED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := &domain.Server{ID: 1, Installed: tt.in, ServerIP: "1.1.1.1", ServerPort: 1}

			got := DomainServerToProto(srv)

			require.NotNil(t, got)
			assert.Equal(t, tt.want, got.Installed)
		})
	}
}

func TestDomainGameToProto(t *testing.T) {
	tests := []struct {
		name      string
		in        *domain.Game
		assertion func(*testing.T, *proto.Game)
	}{
		{
			name: "minimal_game_with_zero_values",
			in: &domain.Game{
				Code:    "cs",
				Name:    "Counter-Strike",
				Engine:  "GoldSrc",
				Enabled: 1,
			},
			assertion: func(t *testing.T, p *proto.Game) {
				t.Helper()
				assert.Equal(t, "cs", p.Code)
				assert.Equal(t, "Counter-Strike", p.Name)
				assert.Equal(t, "GoldSrc", p.Engine)
				assert.True(t, p.Enabled, "Enabled=1 must produce true")
				assert.Nil(t, p.SteamAppIdLinux)
				assert.Nil(t, p.SteamAppIdWindows)
				assert.Nil(t, p.Metadata)
			},
		},
		{
			name: "disabled_game_propagates_false",
			in: &domain.Game{
				Code:    "cs",
				Enabled: 0,
			},
			assertion: func(t *testing.T, p *proto.Game) {
				t.Helper()
				assert.False(t, p.Enabled, "Enabled=0 must produce false")
			},
		},
		{
			name: "steam_app_ids_clamped_and_propagated",
			in: &domain.Game{
				Code:              "csgo",
				SteamAppIDLinux:   new(uint(740)),
				SteamAppIDWindows: new(uint(741)),
			},
			assertion: func(t *testing.T, p *proto.Game) {
				t.Helper()
				require.NotNil(t, p.SteamAppIdLinux)
				assert.Equal(t, uint32(740), *p.SteamAppIdLinux)
				require.NotNil(t, p.SteamAppIdWindows)
				assert.Equal(t, uint32(741), *p.SteamAppIdWindows)
			},
		},
		{
			name: "all_optional_fields_set",
			in: &domain.Game{
				Code:                    "csgo",
				Name:                    "CS:GO",
				Engine:                  "Source",
				EngineVersion:           "v2",
				SteamAppSetConfig:       new("config"),
				RemoteRepositoryLinux:   new("rrl"),
				RemoteRepositoryWindows: new("rrw"),
				LocalRepositoryLinux:    new("lrl"),
				LocalRepositoryWindows:  new("lrw"),
				Metadata:                domain.Metadata{"k": "v"},
			},
			assertion: func(t *testing.T, p *proto.Game) {
				t.Helper()
				require.NotNil(t, p.SteamAppSetConfig)
				assert.Equal(t, "config", *p.SteamAppSetConfig)
				require.NotNil(t, p.RemoteRepositoryLinux)
				assert.Equal(t, "rrl", *p.RemoteRepositoryLinux)
				require.NotNil(t, p.RemoteRepositoryWindows)
				assert.Equal(t, "rrw", *p.RemoteRepositoryWindows)
				require.NotNil(t, p.LocalRepositoryLinux)
				assert.Equal(t, "lrl", *p.LocalRepositoryLinux)
				require.NotNil(t, p.LocalRepositoryWindows)
				assert.Equal(t, "lrw", *p.LocalRepositoryWindows)
				require.NotNil(t, p.Metadata)
				require.Contains(t, p.Metadata, "k")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DomainGameToProto(tt.in)

			require.NotNil(t, got)
			tt.assertion(t, got)
		})
	}
}

func TestDomainGameModToProto(t *testing.T) {
	t.Run("minimal_game_mod_yields_empty_collections", func(t *testing.T) {
		// ARRANGE
		gm := &domain.GameMod{
			ID:       11,
			GameCode: "cs",
			Name:     "Classic",
		}

		// ACT
		got := DomainGameModToProto(gm)

		// ASSERT
		require.NotNil(t, got)
		assert.Equal(t, uint64(11), got.Id)
		assert.Equal(t, "cs", got.GameCode)
		assert.Equal(t, "Classic", got.Name)
		require.NotNil(t, got.FastRcon, "FastRcon must be non-nil empty slice")
		assert.Empty(t, got.FastRcon)
		require.NotNil(t, got.Vars, "Vars must be non-nil empty slice")
		assert.Empty(t, got.Vars)
		assert.Nil(t, got.StartCmdLinux)
	})

	t.Run("populated_game_mod_translates_all_fields", func(t *testing.T) {
		// ARRANGE
		gm := &domain.GameMod{
			ID:                      22,
			GameCode:                "cs",
			Name:                    "Mod",
			StartCmdLinux:           new("./start"),
			StartCmdWindows:         new("start.exe"),
			KickCmd:                 new("kick {user}"),
			BanCmd:                  new("ban {user}"),
			ChnameCmd:               new("name {x}"),
			SrestartCmd:             new("restart"),
			ChmapCmd:                new("map {map}"),
			SendmsgCmd:              new("msg {msg}"),
			PasswdCmd:               new("pass {p}"),
			RemoteRepositoryLinux:   new("rrl"),
			RemoteRepositoryWindows: new("rrw"),
			LocalRepositoryLinux:    new("lrl"),
			LocalRepositoryWindows:  new("lrw"),
			FastRcon: domain.GameModFastRconList{
				{Info: "kill all", Command: "killall"},
				{Info: "swap", Command: "swap"},
			},
			Vars: domain.GameModVarList{
				{Var: "tickrate", Default: domain.GameModVarDefault("128"), Info: "tick", AdminVar: false},
				{Var: "rcon_password", Default: domain.GameModVarDefault("changeme"), Info: "pwd", AdminVar: true},
			},
			Metadata: domain.Metadata{"key": "value"},
		}

		// ACT
		got := DomainGameModToProto(gm)

		// ASSERT
		require.NotNil(t, got)
		require.Len(t, got.FastRcon, 2, "FastRcon must keep order and length")
		assert.Equal(t, "killall", got.FastRcon[0].Command)
		assert.Equal(t, "swap", got.FastRcon[1].Command)

		require.Len(t, got.Vars, 2, "Vars must keep order and length")
		assert.Equal(t, "tickrate", got.Vars[0].Var)
		assert.Equal(t, "128", got.Vars[0].Default)
		assert.False(t, got.Vars[0].AdminVar)
		assert.Equal(t, "rcon_password", got.Vars[1].Var)
		assert.True(t, got.Vars[1].AdminVar)

		require.NotNil(t, got.StartCmdLinux)
		assert.Equal(t, "./start", *got.StartCmdLinux)
		require.NotNil(t, got.PasswdCmd)
		assert.Equal(t, "pass {p}", *got.PasswdCmd)

		require.NotNil(t, got.Metadata)
		require.Contains(t, got.Metadata, "key")
	})
}

func TestDomainMetadataToProto(t *testing.T) {
	t.Run("nil_metadata_returns_nil", func(t *testing.T) {
		assert.Nil(t, domainMetadataToProto(nil))
	})

	t.Run("scalars_stringified_into_anypb_wrappers", func(t *testing.T) {
		// ARRANGE
		m := domain.Metadata{
			"int_val":  42,
			"str_val":  "hello",
			"bool_val": true,
		}

		// ACT
		got := domainMetadataToProto(m)

		// ASSERT
		require.NotNil(t, got)
		require.Len(t, got, 3, "must contain three converted entries")

		for k, expected := range map[string]string{
			"int_val":  "42",
			"str_val":  "hello",
			"bool_val": "true",
		} {
			anyVal := got[k]
			require.NotNil(t, anyVal, "key %s must be present", k)
			wrapped := wrapperspb.String("")
			require.NoError(t, anyVal.UnmarshalTo(wrapped))
			assert.Equal(t, expected, wrapped.GetValue(), "value for %s must round-trip", k)
		}
	})
}

func TestDomainDaemonTaskToProto(t *testing.T) {
	created := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	updated := time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)
	serverID := uint(7)
	runAfterID := uint(3)
	cmd := "/usr/bin/start"
	data := `{"foo":"bar"}`
	output := "captured stdout"

	tests := []struct {
		name      string
		task      *domain.DaemonTask
		assertion func(*testing.T, *proto.DaemonTask)
	}{
		{
			name: "fully_populated_task_translates_all_fields",
			task: &domain.DaemonTask{
				ID:                10,
				RunAftID:          &runAfterID,
				CreatedAt:         &created,
				UpdatedAt:         &updated,
				DedicatedServerID: 5,
				ServerID:          &serverID,
				Task:              domain.DaemonTaskTypeServerInstall,
				Data:              &data,
				Cmd:               &cmd,
				Output:            &output,
				Status:            domain.DaemonTaskStatusWorking,
			},
			assertion: func(t *testing.T, p *proto.DaemonTask) {
				t.Helper()
				assert.Equal(t, uint64(10), p.Id)
				require.NotNil(t, p.RunAfterId)
				assert.Equal(t, uint64(3), *p.RunAfterId)
				assert.Equal(t, uint64(5), p.NodeId)
				require.NotNil(t, p.ServerId)
				assert.Equal(t, uint64(7), *p.ServerId)
				assert.Equal(t, proto.DaemonTaskType_DAEMON_TASK_TYPE_SERVER_INSTALL, p.TaskType)
				assert.Equal(t, proto.DaemonTaskStatus_DAEMON_TASK_STATUS_WORKING, p.Status)
				require.NotNil(t, p.Data)
				assert.Equal(t, data, *p.Data)
				require.NotNil(t, p.Cmd)
				assert.Equal(t, cmd, *p.Cmd)
				require.NotNil(t, p.Output)
				assert.Equal(t, output, *p.Output)
				require.NotNil(t, p.CreatedAt)
				assert.Equal(t, created, p.CreatedAt.AsTime())
				require.NotNil(t, p.UpdatedAt)
				assert.Equal(t, updated, p.UpdatedAt.AsTime())
			},
		},
		{
			name: "nil_optional_fields_stay_nil",
			task: &domain.DaemonTask{
				ID:                1,
				DedicatedServerID: 9,
				Task:              domain.DaemonTaskTypeServerStart,
				Status:            domain.DaemonTaskStatusWaiting,
			},
			assertion: func(t *testing.T, p *proto.DaemonTask) {
				t.Helper()
				assert.Nil(t, p.RunAfterId)
				assert.Nil(t, p.ServerId)
				assert.Nil(t, p.CreatedAt)
				assert.Nil(t, p.UpdatedAt)
				assert.Equal(t, proto.DaemonTaskType_DAEMON_TASK_TYPE_SERVER_START, p.TaskType)
				assert.Equal(t, proto.DaemonTaskStatus_DAEMON_TASK_STATUS_WAITING, p.Status)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DomainDaemonTaskToProto(tt.task)

			require.NotNil(t, got)
			tt.assertion(t, got)
		})
	}
}

func TestDomainTaskTypeToProto(t *testing.T) {
	tests := []struct {
		name string
		in   domain.DaemonTaskType
		want proto.DaemonTaskType
	}{
		{name: "server_start", in: domain.DaemonTaskTypeServerStart, want: proto.DaemonTaskType_DAEMON_TASK_TYPE_SERVER_START},
		{name: "server_stop", in: domain.DaemonTaskTypeServerStop, want: proto.DaemonTaskType_DAEMON_TASK_TYPE_SERVER_STOP},
		{name: "server_restart", in: domain.DaemonTaskTypeServerRestart, want: proto.DaemonTaskType_DAEMON_TASK_TYPE_SERVER_RESTART},
		{name: "server_update", in: domain.DaemonTaskTypeServerUpdate, want: proto.DaemonTaskType_DAEMON_TASK_TYPE_SERVER_UPDATE},
		{name: "server_install", in: domain.DaemonTaskTypeServerInstall, want: proto.DaemonTaskType_DAEMON_TASK_TYPE_SERVER_INSTALL},
		{name: "server_delete", in: domain.DaemonTaskTypeServerDelete, want: proto.DaemonTaskType_DAEMON_TASK_TYPE_SERVER_DELETE},
		{name: "server_move", in: domain.DaemonTaskTypeServerMove, want: proto.DaemonTaskType_DAEMON_TASK_TYPE_SERVER_MOVE},
		{name: "cmd_exec", in: domain.DaemonTaskTypeCmdExec, want: proto.DaemonTaskType_DAEMON_TASK_TYPE_CMD_EXEC},
		{name: "unknown_falls_back_to_unspecified", in: domain.DaemonTaskType("garbage"), want: proto.DaemonTaskType_DAEMON_TASK_TYPE_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domainTaskTypeToProto(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDomainTaskStatusToProto(t *testing.T) {
	tests := []struct {
		name string
		in   domain.DaemonTaskStatus
		want proto.DaemonTaskStatus
	}{
		{name: "waiting", in: domain.DaemonTaskStatusWaiting, want: proto.DaemonTaskStatus_DAEMON_TASK_STATUS_WAITING},
		{name: "working", in: domain.DaemonTaskStatusWorking, want: proto.DaemonTaskStatus_DAEMON_TASK_STATUS_WORKING},
		{name: "error", in: domain.DaemonTaskStatusError, want: proto.DaemonTaskStatus_DAEMON_TASK_STATUS_ERROR},
		{name: "success", in: domain.DaemonTaskStatusSuccess, want: proto.DaemonTaskStatus_DAEMON_TASK_STATUS_SUCCESS},
		{name: "canceled", in: domain.DaemonTaskStatusCanceled, want: proto.DaemonTaskStatus_DAEMON_TASK_STATUS_CANCELED},
		{name: "unknown_falls_back_to_unspecified", in: domain.DaemonTaskStatus("nope"), want: proto.DaemonTaskStatus_DAEMON_TASK_STATUS_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domainTaskStatusToProto(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestProtoTaskStatusToDomain(t *testing.T) {
	tests := []struct {
		name string
		in   proto.DaemonTaskStatus
		want domain.DaemonTaskStatus
	}{
		{name: "waiting", in: proto.DaemonTaskStatus_DAEMON_TASK_STATUS_WAITING, want: domain.DaemonTaskStatusWaiting},
		{name: "working", in: proto.DaemonTaskStatus_DAEMON_TASK_STATUS_WORKING, want: domain.DaemonTaskStatusWorking},
		{name: "error", in: proto.DaemonTaskStatus_DAEMON_TASK_STATUS_ERROR, want: domain.DaemonTaskStatusError},
		{name: "success", in: proto.DaemonTaskStatus_DAEMON_TASK_STATUS_SUCCESS, want: domain.DaemonTaskStatusSuccess},
		{name: "canceled", in: proto.DaemonTaskStatus_DAEMON_TASK_STATUS_CANCELED, want: domain.DaemonTaskStatusCanceled},
		{name: "unspecified_falls_back_to_waiting", in: proto.DaemonTaskStatus_DAEMON_TASK_STATUS_UNSPECIFIED, want: domain.DaemonTaskStatusWaiting},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProtoTaskStatusToDomain(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDomainServerSettingsToProto(t *testing.T) {
	t.Run("empty_input_returns_empty_slice", func(t *testing.T) {
		got := DomainServerSettingsToProto(nil)
		require.NotNil(t, got, "must return non-nil even for nil input")
		assert.Empty(t, got)
	})

	t.Run("string_int_bool_values_serialised_via_String", func(t *testing.T) {
		// ARRANGE
		settings := []domain.ServerSetting{
			{ID: 1, ServerID: 10, Name: "map", Value: domain.NewServerSettingValue("de_dust2")},
			{ID: 2, ServerID: 10, Name: "tickrate", Value: domain.NewServerSettingValue(128)},
			{ID: 3, ServerID: 11, Name: "private", Value: domain.NewServerSettingValue(true)},
		}

		// ACT
		got := DomainServerSettingsToProto(settings)

		// ASSERT
		require.Len(t, got, 3)
		assert.Equal(t, uint64(1), got[0].Id)
		assert.Equal(t, uint64(10), got[0].ServerId)
		assert.Equal(t, "map", got[0].Name)
		assert.Equal(t, "de_dust2", got[0].Value)

		assert.Equal(t, "128", got[1].Value, "int values must be stringified")
		assert.Equal(t, "true", got[2].Value, "bool values must be stringified")
	})
}

func TestClampToInt32(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int32
	}{
		{name: "zero_passthrough", in: 0, want: 0},
		{name: "negative_passthrough", in: -1, want: -1},
		{name: "small_positive_passthrough", in: 27015, want: 27015},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampToInt32(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClampToUint32(t *testing.T) {
	tests := []struct {
		name string
		in   uint
		want uint32
	}{
		{name: "zero_passthrough", in: 0, want: 0},
		{name: "small_positive_passthrough", in: 740, want: 740},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampToUint32(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}
