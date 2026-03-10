package gamesimport

import (
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportOptions_Validate(t *testing.T) {
	tests := []struct {
		name      string
		opts      *Options
		wantError string
	}{
		{
			name: "nil_options_is_valid",
			opts: nil,
		},
		{
			name: "empty_options_is_valid",
			opts: &Options{},
		},
		{
			name: "valid_code_only",
			opts: &Options{
				Code: lo.ToPtr("test_game"),
			},
		},
		{
			name: "valid_name_only",
			opts: &Options{
				Name: lo.ToPtr("Test Game"),
			},
		},
		{
			name: "valid_code_and_name",
			opts: &Options{
				Code: lo.ToPtr("test"),
				Name: lo.ToPtr("Test Game"),
			},
		},
		{
			name: "valid_code_min_length",
			opts: &Options{
				Code: lo.ToPtr("ab"),
			},
		},
		{
			name: "valid_code_max_length",
			opts: &Options{
				Code: lo.ToPtr("1234567890123456"),
			},
		},
		{
			name: "valid_name_min_length",
			opts: &Options{
				Name: lo.ToPtr("AB"),
			},
		},
		{
			name: "valid_code_with_underscores",
			opts: &Options{
				Code: lo.ToPtr("my_game_code"),
			},
		},
		{
			name: "valid_code_with_hyphens",
			opts: &Options{
				Code: lo.ToPtr("my-game-code"),
			},
		},
		{
			name: "valid_code_with_numbers",
			opts: &Options{
				Code: lo.ToPtr("game123"),
			},
		},
		{
			name:      "code_too_short",
			opts:      &Options{Code: lo.ToPtr("a")},
			wantError: "code must be between 2 and 16 characters",
		},
		{
			name:      "code_too_long",
			opts:      &Options{Code: lo.ToPtr("12345678901234567")},
			wantError: "code must be between 2 and 16 characters",
		},
		{
			name:      "code_with_uppercase",
			opts:      &Options{Code: lo.ToPtr("Test")},
			wantError: "code must match pattern",
		},
		{
			name:      "code_with_spaces",
			opts:      &Options{Code: lo.ToPtr("test game")},
			wantError: "code must match pattern",
		},
		{
			name:      "code_with_special_chars",
			opts:      &Options{Code: lo.ToPtr("test@game")},
			wantError: "code must match pattern",
		},
		{
			name:      "name_too_short",
			opts:      &Options{Name: lo.ToPtr("A")},
			wantError: "name must be between 2 and 128 characters",
		},
		{
			name:      "name_too_long",
			opts:      &Options{Name: lo.ToPtr(strings.Repeat("a", 129))},
			wantError: "name must be between 2 and 128 characters",
		},
		{
			name: "valid_code_with_invalid_name_returns_error",
			opts: &Options{
				Code: lo.ToPtr("valid"),
				Name: lo.ToPtr("A"),
			},
			wantError: "name must be between 2 and 128 characters",
		},
		{
			name: "invalid_code_with_valid_name_returns_error",
			opts: &Options{
				Code: lo.ToPtr("a"),
				Name: lo.ToPtr("Valid Name"),
			},
			wantError: "code must be between 2 and 16 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()

			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestImportOptions_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		opts     *Options
		expected bool
	}{
		{
			name:     "nil_options",
			opts:     nil,
			expected: true,
		},
		{
			name:     "empty_options",
			opts:     &Options{},
			expected: true,
		},
		{
			name: "only_code_set",
			opts: &Options{
				Code: lo.ToPtr("test"),
			},
			expected: false,
		},
		{
			name: "only_name_set",
			opts: &Options{
				Name: lo.ToPtr("Test"),
			},
			expected: false,
		},
		{
			name: "both_set",
			opts: &Options{
				Code: lo.ToPtr("test"),
				Name: lo.ToPtr("Test"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.opts.IsEmpty()
			assert.Equal(t, tt.expected, result)
		})
	}
}
