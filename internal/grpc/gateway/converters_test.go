package gateway

import (
	"testing"

	"github.com/gameap/gameap/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
