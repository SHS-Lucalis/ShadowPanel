package rcon

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_DispatchesByProtocol(t *testing.T) {
	tests := []struct {
		name      string
		protocol  Protocol
		wantType  string
		wantError string
	}{
		{
			name:     "source_protocol_returns_source_client",
			protocol: ProtocolSource,
			wantType: "*rcon.Source",
		},
		{
			name:     "goldsource_protocol_returns_goldsource_client",
			protocol: ProtocolGoldSrc,
			wantType: "*rcon.GoldSource",
		},
		{
			name:      "unknown_protocol_returns_error",
			protocol:  Protocol("rogue"),
			wantError: ErrUnsupportedProtocol.Error(),
		},
		{
			name:      "empty_protocol_returns_error",
			protocol:  Protocol(""),
			wantError: ErrUnsupportedProtocol.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ARRANGE
			cfg := Config{
				Address:  "127.0.0.1:27015",
				Password: "x",
				Protocol: tt.protocol,
			}

			// ACT
			client, err := NewClient(cfg)

			// ASSERT
			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
				assert.Nil(t, client)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, client)

			switch tt.protocol {
			case ProtocolSource:
				_, ok := client.(*Source)
				assert.True(t, ok, "Source protocol must yield *Source, got %T", client)
			case ProtocolGoldSrc:
				_, ok := client.(*GoldSource)
				assert.True(t, ok, "GoldSrc protocol must yield *GoldSource, got %T", client)
			}
		})
	}
}

func TestIsProtocolSupported(t *testing.T) {
	tests := []struct {
		name     string
		protocol Protocol
		want     bool
	}{
		{name: "source_is_supported", protocol: ProtocolSource, want: true},
		{name: "goldsource_is_supported", protocol: ProtocolGoldSrc, want: true},
		{name: "unknown_is_not_supported", protocol: Protocol("rogue"), want: false},
		{name: "empty_is_not_supported", protocol: Protocol(""), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ACT
			got := IsProtocolSupported(tt.protocol)

			// ASSERT
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsPlayerManagementSupported(t *testing.T) {
	tests := []struct {
		name     string
		gameCode string
		want     bool
	}{
		{name: "cs_is_supported", gameCode: "cs", want: true},
		{name: "minecraft_is_supported", gameCode: "minecraft", want: true},
		{name: "valve_is_supported", gameCode: "valve", want: true},
		{name: "unknown_game_is_not_supported", gameCode: "rust", want: false},
		{name: "empty_game_code_is_not_supported", gameCode: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ACT
			got := IsPlayerManagementSupported(tt.gameCode)

			// ASSERT
			assert.Equal(t, tt.want, got)
		})
	}
}
