package importgameap

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/repositories/inmemory"
	"github.com/gameap/gameap/internal/services"
	"github.com/gameap/gameap/internal/services/gameapimporter"
	"github.com/gameap/gameap/pkg/api"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_ServeHTTP(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		setupGame      func(*inmemory.GameRepository)
		setupGameMod   func(*inmemory.GameModRepository)
		expectedStatus int
		wantError      string
		validate       func(t *testing.T, response Response, gameRepo *inmemory.GameRepository, gameModRepo *inmemory.GameModRepository)
	}{
		{
			name: "successful_import_new_game_and_mods",
			requestBody: `
schema_version: "1.0"
game:
  code: "cstrike"
  name: "Counter-Strike 1.6"
  engine: "GoldSource"
  engine_version: "1.0"
  steam_app_id_linux: 90
  steam_app_id_windows: 90
mods:
  - name: "Classic"
    start_cmd_linux: "./hlds_run -game cstrike +port {port}"
    kick_cmd: "kick {name}"
    fast_rcon:
      - info: "Restart"
        command: "changelevel {map}"
    vars:
      - var: "maxplayers"
        default: "32"
        info: "Max players"
  - name: "Deathmatch"
    start_cmd_linux: "./hlds_run -game cstrike_dm +port {port}"
`,
			setupGame:      func(_ *inmemory.GameRepository) {},
			setupGameMod:   func(_ *inmemory.GameModRepository) {},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, response Response, gameRepo *inmemory.GameRepository, gameModRepo *inmemory.GameModRepository) {
				t.Helper()

				assert.Equal(t, "cstrike", response.GameCode)
				assert.Equal(t, "Counter-Strike 1.6", response.GameName)
				assert.Equal(t, 2, response.ModsImported)
				require.Len(t, response.ModsCreated, 2)
				assert.Contains(t, response.ModsCreated, "Classic")
				assert.Contains(t, response.ModsCreated, "Deathmatch")
				require.Len(t, response.ModsUpdated, 0)

				games, err := gameRepo.FindAll(context.Background(), nil, nil)
				require.NoError(t, err)
				require.Len(t, games, 1)
				assert.Equal(t, "Counter-Strike 1.6", games[0].Name)
				assert.Equal(t, "GoldSource", games[0].Engine)

				gameMods, err := gameModRepo.FindAll(context.Background(), nil, nil)
				require.NoError(t, err)
				require.Len(t, gameMods, 2)
			},
		},
		{
			name: "successful_import_updates_existing_game",
			requestBody: `
schema_version: "1.0"
game:
  code: "existing"
  name: "Updated Name"
  engine: "New Engine"
mods:
  - name: "Default"
    start_cmd_linux: "./new_command"
`,
			setupGame: func(repo *inmemory.GameRepository) {
				_ = repo.Save(context.Background(), &domain.Game{
					Code:    "existing",
					Name:    "Old Name",
					Engine:  "Old Engine",
					Enabled: 1,
				})
			},
			setupGameMod: func(repo *inmemory.GameModRepository) {
				_ = repo.Save(context.Background(), &domain.GameMod{
					GameCode:      "existing",
					Name:          "Default",
					StartCmdLinux: lo.ToPtr("./old_command"),
				})
			},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, response Response, gameRepo *inmemory.GameRepository, _ *inmemory.GameModRepository) {
				t.Helper()

				assert.Equal(t, "existing", response.GameCode)
				assert.Equal(t, "Updated Name", response.GameName)
				require.Len(t, response.ModsCreated, 0)
				require.Len(t, response.ModsUpdated, 1)

				games, err := gameRepo.FindAll(context.Background(), nil, nil)
				require.NoError(t, err)
				require.Len(t, games, 1)
				assert.Equal(t, "Updated Name", games[0].Name)
				assert.Equal(t, "New Engine", games[0].Engine)
			},
		},
		{
			name: "import_game_without_mods",
			requestBody: `
schema_version: "1.0"
game:
  code: "nomods"
  name: "No Mods Game"
  engine: "Test"
`,
			setupGame:      func(_ *inmemory.GameRepository) {},
			setupGameMod:   func(_ *inmemory.GameModRepository) {},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, response Response, _ *inmemory.GameRepository, gameModRepo *inmemory.GameModRepository) {
				t.Helper()

				assert.Equal(t, "nomods", response.GameCode)
				assert.Equal(t, 0, response.ModsImported)
				require.Len(t, response.ModsCreated, 0)
				require.Len(t, response.ModsUpdated, 0)

				gameMods, err := gameModRepo.FindAll(context.Background(), nil, nil)
				require.NoError(t, err)
				require.Len(t, gameMods, 0)
			},
		},
		{
			name:           "invalid_yaml",
			requestBody:    `schema_version: "1.0"\ngame: {`,
			setupGame:      func(_ *inmemory.GameRepository) {},
			setupGameMod:   func(_ *inmemory.GameModRepository) {},
			expectedStatus: http.StatusBadRequest,
			wantError:      "failed to parse GameAP YAML",
		},
		{
			name: "missing_schema_version",
			requestBody: `
game:
  code: "test"
  name: "Test"
  engine: "Test"
`,
			setupGame:      func(_ *inmemory.GameRepository) {},
			setupGameMod:   func(_ *inmemory.GameModRepository) {},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "invalid_game_code",
			requestBody: `
schema_version: "1.0"
game:
  code: "Invalid Code"
  name: "Test"
  engine: "Test"
`,
			setupGame:      func(_ *inmemory.GameRepository) {},
			setupGameMod:   func(_ *inmemory.GameModRepository) {},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "duplicate_mod_names",
			requestBody: `
schema_version: "1.0"
game:
  code: "test"
  name: "Test"
  engine: "Test"
mods:
  - name: "Same"
  - name: "Same"
`,
			setupGame:      func(_ *inmemory.GameRepository) {},
			setupGameMod:   func(_ *inmemory.GameModRepository) {},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "unsupported_schema_version",
			requestBody: `
schema_version: "2.0"
game:
  code: "test"
  name: "Test"
  engine: "Test"
`,
			setupGame:      func(_ *inmemory.GameRepository) {},
			setupGameMod:   func(_ *inmemory.GameModRepository) {},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gameRepo := inmemory.NewGameRepository()
			gameModRepo := inmemory.NewGameModRepository()
			responder := api.NewResponder()

			tt.setupGame(gameRepo)
			tt.setupGameMod(gameModRepo)

			importer := gameapimporter.NewImporter(
				gameRepo,
				gameModRepo,
				services.NewNilTransactionManager(),
			)

			handler := NewHandler(importer, responder)

			req := httptest.NewRequest(http.MethodPost, "/api/games/import/gameap", bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/x-yaml")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			switch {
			case tt.wantError != "":
				var errorResp map[string]any
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errorResp))
				assert.Equal(t, "error", errorResp["status"])
				errorMsg, ok := errorResp["error"].(string)
				require.True(t, ok)
				assert.True(t, strings.Contains(errorMsg, tt.wantError),
					"Expected error containing '%s', got: %s", tt.wantError, errorMsg)
			case w.Code >= http.StatusBadRequest:
				var errorResp map[string]any
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errorResp))
				assert.Equal(t, "error", errorResp["status"])
			case tt.validate != nil:
				var response Response
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
				tt.validate(t, response, gameRepo, gameModRepo)
			}
		})
	}
}

func TestHandler_EmptyBody(t *testing.T) {
	gameRepo := inmemory.NewGameRepository()
	gameModRepo := inmemory.NewGameModRepository()
	responder := api.NewResponder()

	importer := gameapimporter.NewImporter(
		gameRepo,
		gameModRepo,
		services.NewNilTransactionManager(),
	)

	handler := NewHandler(importer, responder)

	req := httptest.NewRequest(http.MethodPost, "/api/games/import/gameap", nil)
	req.Header.Set("Content-Type", "application/x-yaml")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "error", response["status"])
	errorMsg, ok := response["error"].(string)
	require.True(t, ok)
	assert.Contains(t, errorMsg, "request body is empty")
}

func TestHandler_BodyTooLarge(t *testing.T) {
	gameRepo := inmemory.NewGameRepository()
	gameModRepo := inmemory.NewGameModRepository()
	responder := api.NewResponder()

	importer := gameapimporter.NewImporter(
		gameRepo,
		gameModRepo,
		services.NewNilTransactionManager(),
	)

	handler := NewHandler(importer, responder)

	largeBody := strings.Repeat("a", 2*1024*1024)

	req := httptest.NewRequest(http.MethodPost, "/api/games/import/gameap", bytes.NewBufferString(largeBody))
	req.Header.Set("Content-Type", "application/x-yaml")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "error", response["status"])
	errorMsg, ok := response["error"].(string)
	require.True(t, ok)
	assert.Contains(t, errorMsg, "maximum 1 MB")
}

func TestHandler_ResponseStructure(t *testing.T) {
	gameRepo := inmemory.NewGameRepository()
	gameModRepo := inmemory.NewGameModRepository()
	responder := api.NewResponder()

	importer := gameapimporter.NewImporter(
		gameRepo,
		gameModRepo,
		services.NewNilTransactionManager(),
	)

	handler := NewHandler(importer, responder)

	requestBody := `
schema_version: "1.0"
game:
  code: "test"
  name: "Test Game"
  engine: "Test Engine"
mods:
  - name: "Default"
    start_cmd_linux: "./server"
`

	req := httptest.NewRequest(http.MethodPost, "/api/games/import/gameap", bytes.NewBufferString(requestBody))
	req.Header.Set("Content-Type", "application/x-yaml")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "test", response.GameCode)
	assert.Equal(t, "Test Game", response.GameName)
	assert.Equal(t, 1, response.ModsImported)
	require.Len(t, response.ModsCreated, 1)
	assert.Equal(t, "Default", response.ModsCreated[0])
	require.Len(t, response.ModsUpdated, 0)
}
