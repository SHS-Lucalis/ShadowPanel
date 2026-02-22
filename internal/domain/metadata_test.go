package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadata_Scan(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected Metadata
		wantErr  bool
	}{
		{
			name:     "nil_value",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty_bytes",
			input:    []byte{},
			expected: nil,
		},
		{
			name:     "valid_json_bytes",
			input:    []byte(`{"docker_image":"ghcr.io/gameap/csgo:latest","default_port":27015}`),
			expected: Metadata{"docker_image": "ghcr.io/gameap/csgo:latest", "default_port": float64(27015)},
		},
		{
			name:     "valid_json_string",
			input:    `{"key":"value"}`,
			expected: Metadata{"key": "value"},
		},
		{
			name:     "nested_object",
			input:    []byte(`{"config":{"enabled":true,"ports":[27015,27016]}}`),
			expected: Metadata{"config": map[string]any{"enabled": true, "ports": []any{float64(27015), float64(27016)}}},
		},
		{
			name:     "unsupported_type",
			input:    123,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m Metadata
			err := m.Scan(tt.input)

			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, m)
		})
	}
}

func TestMetadata_Value(t *testing.T) {
	tests := []struct {
		name      string
		metadata  Metadata
		expected  string
		expectNil bool
	}{
		{
			name:      "nil_metadata",
			metadata:  nil,
			expectNil: true,
		},
		{
			name:     "simple_object",
			metadata: Metadata{"key": "value"},
			expected: `{"key":"value"}`,
		},
		{
			name:     "complex_object",
			metadata: Metadata{"docker_image": "test:latest", "port": float64(27015)},
			expected: `{"docker_image":"test:latest","port":27015}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.metadata.Value()
			require.NoError(t, err)

			if tt.expectNil {
				assert.Nil(t, val)

				return
			}

			bytes, ok := val.([]byte)
			require.True(t, ok)
			assert.JSONEq(t, tt.expected, string(bytes))
		})
	}
}

func TestMetadata_String(t *testing.T) {
	tests := []struct {
		name     string
		metadata Metadata
		expected string
	}{
		{
			name:     "nil_metadata",
			metadata: nil,
			expected: "",
		},
		{
			name:     "empty_metadata",
			metadata: Metadata{},
			expected: "{}",
		},
		{
			name:     "simple_object",
			metadata: Metadata{"key": "value"},
			expected: `{"key":"value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.metadata.String()

			if tt.expected == "" {
				assert.Equal(t, "", result)

				return
			}

			assert.JSONEq(t, tt.expected, result)
		})
	}
}
