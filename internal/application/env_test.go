package application

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadEnvFile(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		setupPath func(t *testing.T, content string) string
		preset    map[string]string
		wantEnv   map[string]string
		wantError string
	}{
		{
			name: "empty_path_is_noop",
			setupPath: func(t *testing.T, _ string) string {
				t.Helper()

				return ""
			},
		},
		{
			name: "missing_file_returns_open_error",
			setupPath: func(t *testing.T, _ string) string {
				t.Helper()

				return filepath.Join(t.TempDir(), "missing.env")
			},
			wantError: "failed to open env file",
		},
		{
			name:    "single_key_value_is_set",
			content: "FOO=bar\n",
			wantEnv: map[string]string{"FOO": "bar"},
		},
		{
			name:    "multiple_variables_in_one_file",
			content: "ALPHA=one\nBRAVO=two\nCHARLIE=three\n",
			wantEnv: map[string]string{
				"ALPHA":   "one",
				"BRAVO":   "two",
				"CHARLIE": "three",
			},
		},
		{
			name:    "blank_lines_skipped",
			content: "\n\nA=1\n\nB=2\n\n",
			wantEnv: map[string]string{"A": "1", "B": "2"},
		},
		{
			name:    "comment_lines_skipped",
			content: "# this is a comment\nKEY=value\n# trailing comment\n",
			wantEnv: map[string]string{"KEY": "value"},
		},
		{
			name:    "double_quoted_value_unquoted",
			content: `KEY="hello world"` + "\n",
			wantEnv: map[string]string{"KEY": "hello world"},
		},
		{
			name:    "single_quoted_value_unquoted",
			content: "KEY='hello world'\n",
			wantEnv: map[string]string{"KEY": "hello world"},
		},
		{
			name:    "whitespace_around_key_and_value_trimmed",
			content: "  KEY  =   value with spaces   \n",
			wantEnv: map[string]string{"KEY": "value with spaces"},
		},
		{
			name:    "value_with_equals_sign_preserved",
			content: "KEY=base64:abc=def==\n",
			wantEnv: map[string]string{"KEY": "base64:abc=def=="},
		},
		{
			name:    "leading_hash_after_whitespace_treated_as_comment",
			content: "  # indented comment\nKEY=value\n",
			wantEnv: map[string]string{"KEY": "value"},
		},
		{
			name:      "malformed_line_without_equals_returns_error",
			content:   "VALID=ok\nINVALID_LINE\n",
			wantError: "invalid env file format at line 2",
		},
		{
			name:    "empty_value_after_equals_is_set_empty",
			content: "EMPTY=\n",
			wantEnv: map[string]string{"EMPTY": ""},
		},
		{
			name:    "key_with_existing_value_is_overwritten",
			content: "EXISTING=new\n",
			preset:  map[string]string{"EXISTING": "old"},
			wantEnv: map[string]string{"EXISTING": "new"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ARRANGE
			for k, v := range tt.preset {
				t.Setenv(k, v)
			}

			for k := range tt.wantEnv {
				if _, exists := tt.preset[k]; !exists {
					t.Setenv(k, "")
				}
			}

			var path string
			if tt.setupPath != nil {
				path = tt.setupPath(t, tt.content)
			} else {
				path = filepath.Join(t.TempDir(), "test.env")
				require.NoError(t, os.WriteFile(path, []byte(tt.content), 0o600))
			}

			// ACT
			err := loadEnvFile(path)

			// ASSERT
			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)

				return
			}

			require.NoError(t, err)
			for k, want := range tt.wantEnv {
				assert.Equal(t, want, os.Getenv(k), "env var %q", k)
			}
		})
	}
}
