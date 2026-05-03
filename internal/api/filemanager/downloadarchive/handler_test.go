package downloadarchive

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/rbac"
	"github.com/gameap/gameap/internal/repositories/inmemory"
	"github.com/gameap/gameap/internal/services"
	"github.com/gameap/gameap/internal/services/filemanager/archiver"
	"github.com/gameap/gameap/pkg/api"
	"github.com/gameap/gameap/pkg/auth"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errDaemonUnavailable = errors.New("daemon unavailable")

var testUser = domain.User{
	ID:    1,
	Login: "tester",
	Email: "t@example.com",
}

var testNode = domain.Node{
	ID:                  1,
	Enabled:             true,
	Name:                "node",
	OS:                  "linux",
	Location:            "loc",
	GdaemonHost:         "127.0.0.1",
	GdaemonPort:         31717,
	GdaemonAPIKey:       "k",
	WorkPath:            "/srv",
	GdaemonServerCert:   "c",
	ClientCertificateID: 1,
}

func authedCtx() context.Context {
	return auth.ContextWithSession(context.Background(), &auth.Session{
		Login: testUser.Login,
		Email: testUser.Email,
		User:  &testUser,
	})
}

func saveServerWithFilesAbility(
	t *testing.T,
	serverRepo *inmemory.ServerRepository,
	nodeRepo *inmemory.NodeRepository,
	rbacRepo *inmemory.RBACRepository,
) {
	t.Helper()

	now := time.Now()
	server := &domain.Server{
		ID:        1,
		UID:       uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		UUIDShort: "short1",
		Enabled:   true,
		Installed: 1,
		Name:      "S1",
		GameID:    "g",
		DSID:      1,
		GameModID: 1,
		ServerIP:  "127.0.0.1",
		Dir:       "servers/s1",
		CreatedAt: &now,
		UpdatedAt: &now,
	}
	require.NoError(t, serverRepo.Save(context.Background(), server))
	serverRepo.AddUserServer(testUser.ID, server.ID)

	ability := &domain.Ability{
		Name:       domain.AbilityNameGameServerFiles,
		EntityType: lo.ToPtr(domain.EntityTypeServer),
		EntityID:   new(server.ID),
	}
	require.NoError(t, rbacRepo.SaveAbility(context.Background(), ability))
	require.NoError(t, rbacRepo.SavePermission(context.Background(), &domain.Permission{
		AbilityID:  ability.ID,
		EntityID:   new(testUser.ID),
		EntityType: lo.ToPtr(domain.EntityTypeUser),
		Forbidden:  false,
	}))

	node := testNode
	require.NoError(t, nodeRepo.Save(context.Background(), &node))
}

type stubArchiver struct {
	manifest      *archiver.Manifest
	manifestErr   error
	writeErr      error
	writeContent  []byte
	writeRecorded *archiver.Manifest
}

func (s *stubArchiver) BuildManifest(
	_ context.Context, _ *domain.Node, _ string, _ archiver.Limits,
) (*archiver.Manifest, error) {
	return s.manifest, s.manifestErr
}

func (s *stubArchiver) WriteArchive(
	_ context.Context, w io.Writer, _ *domain.Node, m *archiver.Manifest, _ archiver.Options,
) (*archiver.Result, error) {
	s.writeRecorded = m
	if s.writeErr != nil {
		return nil, s.writeErr
	}
	if len(s.writeContent) > 0 {
		n, err := w.Write(s.writeContent)
		if err != nil {
			return nil, err
		}

		return &archiver.Result{BytesWritten: uint64(n)}, nil
	}

	zw := zip.NewWriter(w)
	fw, _ := zw.Create("data/a.txt")
	_, _ = fw.Write([]byte("alpha"))
	if err := zw.Close(); err != nil {
		return nil, err
	}

	return &archiver.Result{BytesWritten: 0}, nil
}

type fakeGuard struct {
	acquireErr error
	released   bool
}

func (f *fakeGuard) Acquire(_ context.Context, _ uint) (func(), error) {
	if f.acquireErr != nil {
		return nil, f.acquireErr
	}

	return func() { f.released = true }, nil
}

func TestHandler_ServeHTTP(t *testing.T) {
	tests := []struct {
		name           string
		serverID       string
		queryDisk      string
		queryPath      string
		queryCompress  string
		setupCtx       func() context.Context
		setupRepo      func(*testing.T, *inmemory.ServerRepository, *inmemory.NodeRepository, *inmemory.RBACRepository)
		setupArchiver  func() *stubArchiver
		setupGuard     func() *fakeGuard
		limits         Limits
		expectedStatus int
		wantError      string
		validate       func(*testing.T, *httptest.ResponseRecorder, *stubArchiver, *fakeGuard)
	}{
		{
			name:      "success_streams_zip_with_headers",
			serverID:  "1",
			queryDisk: "server",
			queryPath: "data",
			setupCtx:  authedCtx,
			setupRepo: saveServerWithFilesAbility,
			setupArchiver: func() *stubArchiver {
				return &stubArchiver{
					manifest: &archiver.Manifest{
						RootName:   "data",
						TotalSize:  500,
						TotalFiles: 2,
						Skipped:    []string{"data/socket"},
						Entries: []archiver.Entry{
							{RelPath: "data/a.txt", Size: 250},
							{RelPath: "data/b.txt", Size: 250},
						},
					},
				}
			},
			setupGuard:     func() *fakeGuard { return &fakeGuard{} },
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, w *httptest.ResponseRecorder, _ *stubArchiver, g *fakeGuard) {
				t.Helper()
				assert.Equal(t, "application/zip", w.Header().Get("Content-Type"))
				assert.Contains(t, w.Header().Get("Content-Disposition"), "data.zip")
				assert.Equal(t, "500", w.Header().Get("X-Archive-Total-Bytes"))
				assert.Equal(t, "2", w.Header().Get("X-Archive-Total-Files"))
				assert.Equal(t, "1", w.Header().Get("X-Archive-Skipped-Count"))
				assert.True(t, g.released, "concurrency slot must be released")

				zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
				require.NoError(t, err)
				require.Len(t, zr.File, 1)
				assert.Equal(t, "data/a.txt", zr.File[0].Name)
			},
		},
		{
			name:      "error_unauthenticated",
			serverID:  "1",
			queryDisk: "server",
			queryPath: "data",
			setupCtx:  context.Background,
			setupRepo: func(_ *testing.T, _ *inmemory.ServerRepository, _ *inmemory.NodeRepository, _ *inmemory.RBACRepository) {
			},
			setupArchiver:  func() *stubArchiver { return &stubArchiver{} },
			setupGuard:     func() *fakeGuard { return &fakeGuard{} },
			expectedStatus: http.StatusUnauthorized,
			wantError:      "user not authenticated",
		},
		{
			name:      "error_disk_required",
			serverID:  "1",
			queryDisk: "",
			queryPath: "data",
			setupCtx:  authedCtx,
			setupRepo: saveServerWithFilesAbility,
			setupArchiver: func() *stubArchiver {
				return &stubArchiver{}
			},
			setupGuard:     func() *fakeGuard { return &fakeGuard{} },
			expectedStatus: http.StatusBadRequest,
			wantError:      "disk parameter is required",
		},
		{
			name:      "error_unsupported_disk",
			serverID:  "1",
			queryDisk: "local",
			queryPath: "data",
			setupCtx:  authedCtx,
			setupRepo: saveServerWithFilesAbility,
			setupArchiver: func() *stubArchiver {
				return &stubArchiver{}
			},
			setupGuard:     func() *fakeGuard { return &fakeGuard{} },
			expectedStatus: http.StatusBadRequest,
			wantError:      "unsupported disk",
		},
		{
			name:      "error_path_required",
			serverID:  "1",
			queryDisk: "server",
			queryPath: "",
			setupCtx:  authedCtx,
			setupRepo: saveServerWithFilesAbility,
			setupArchiver: func() *stubArchiver {
				return &stubArchiver{}
			},
			setupGuard:     func() *fakeGuard { return &fakeGuard{} },
			expectedStatus: http.StatusBadRequest,
			wantError:      "path parameter is required",
		},
		{
			name:      "error_path_traversal",
			serverID:  "1",
			queryDisk: "server",
			queryPath: "../../etc/passwd",
			setupCtx:  authedCtx,
			setupRepo: saveServerWithFilesAbility,
			setupArchiver: func() *stubArchiver {
				return &stubArchiver{}
			},
			setupGuard:     func() *fakeGuard { return &fakeGuard{} },
			expectedStatus: http.StatusBadRequest,
			wantError:      "path contains invalid directory traversal",
		},
		{
			name:          "error_invalid_compress",
			serverID:      "1",
			queryDisk:     "server",
			queryPath:     "data",
			queryCompress: "12",
			setupCtx:      authedCtx,
			setupRepo:     saveServerWithFilesAbility,
			setupArchiver: func() *stubArchiver {
				return &stubArchiver{}
			},
			setupGuard:     func() *fakeGuard { return &fakeGuard{} },
			expectedStatus: http.StatusBadRequest,
			wantError:      "compress must be an integer between 0 and 9",
		},
		{
			name:      "error_total_size_too_large",
			serverID:  "1",
			queryDisk: "server",
			queryPath: "data",
			setupCtx:  authedCtx,
			setupRepo: saveServerWithFilesAbility,
			setupArchiver: func() *stubArchiver {
				return &stubArchiver{
					manifestErr: archiver.ErrTooLarge,
				}
			},
			setupGuard:     func() *fakeGuard { return &fakeGuard{} },
			expectedStatus: http.StatusRequestEntityTooLarge,
			wantError:      "archive total size exceeds limit",
		},
		{
			name:      "error_too_many_files",
			serverID:  "1",
			queryDisk: "server",
			queryPath: "data",
			setupCtx:  authedCtx,
			setupRepo: saveServerWithFilesAbility,
			setupArchiver: func() *stubArchiver {
				return &stubArchiver{
					manifestErr: archiver.ErrTooManyFiles,
				}
			},
			setupGuard:     func() *fakeGuard { return &fakeGuard{} },
			expectedStatus: http.StatusRequestEntityTooLarge,
			wantError:      "archive total file count exceeds limit",
		},
		{
			name:      "error_empty_directory_returns_404",
			serverID:  "1",
			queryDisk: "server",
			queryPath: "data",
			setupCtx:  authedCtx,
			setupRepo: saveServerWithFilesAbility,
			setupArchiver: func() *stubArchiver {
				return &stubArchiver{
					manifestErr: archiver.ErrEmptyManifest,
				}
			},
			setupGuard:     func() *fakeGuard { return &fakeGuard{} },
			expectedStatus: http.StatusNotFound,
			wantError:      "nothing to archive",
		},
		{
			name:      "error_path_is_not_directory",
			serverID:  "1",
			queryDisk: "server",
			queryPath: "file.txt",
			setupCtx:  authedCtx,
			setupRepo: saveServerWithFilesAbility,
			setupArchiver: func() *stubArchiver {
				return &stubArchiver{
					manifestErr: archiver.ErrNotADirectory,
				}
			},
			setupGuard:     func() *fakeGuard { return &fakeGuard{} },
			expectedStatus: http.StatusBadRequest,
			wantError:      "requested path is not a directory",
		},
		{
			name:      "error_too_many_concurrent_returns_429",
			serverID:  "1",
			queryDisk: "server",
			queryPath: "data",
			setupCtx:  authedCtx,
			setupRepo: saveServerWithFilesAbility,
			setupArchiver: func() *stubArchiver {
				return &stubArchiver{
					manifest: &archiver.Manifest{
						RootName:   "data",
						TotalSize:  10,
						TotalFiles: 1,
						Entries: []archiver.Entry{
							{RelPath: "data/a.txt"},
						},
					},
				}
			},
			setupGuard: func() *fakeGuard {
				return &fakeGuard{acquireErr: archiver.ErrTooManyConcurrent}
			},
			expectedStatus: http.StatusTooManyRequests,
			wantError:      "too many concurrent archive downloads",
		},
		{
			name:      "error_user_without_files_permission",
			serverID:  "1",
			queryDisk: "server",
			queryPath: "data",
			setupCtx:  authedCtx,
			setupRepo: func(t *testing.T, serverRepo *inmemory.ServerRepository, nodeRepo *inmemory.NodeRepository, _ *inmemory.RBACRepository) {
				t.Helper()
				now := time.Now()
				server := &domain.Server{
					ID: 1, UID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
					UUIDShort: "short", Enabled: true, Installed: 1, Name: "S1",
					GameID: "g", DSID: 1, GameModID: 1, ServerIP: "127.0.0.1",
					Dir: "servers/s1", CreatedAt: &now, UpdatedAt: &now,
				}
				require.NoError(t, serverRepo.Save(context.Background(), server))
				serverRepo.AddUserServer(testUser.ID, server.ID)

				node := testNode
				require.NoError(t, nodeRepo.Save(context.Background(), &node))
			},
			setupArchiver:  func() *stubArchiver { return &stubArchiver{} },
			setupGuard:     func() *fakeGuard { return &fakeGuard{} },
			expectedStatus: http.StatusForbidden,
			wantError:      "user does not have required permissions",
		},
		{
			name:      "error_archiver_internal_failure",
			serverID:  "1",
			queryDisk: "server",
			queryPath: "data",
			setupCtx:  authedCtx,
			setupRepo: saveServerWithFilesAbility,
			setupArchiver: func() *stubArchiver {
				return &stubArchiver{
					manifestErr: errDaemonUnavailable,
				}
			},
			setupGuard:     func() *fakeGuard { return &fakeGuard{} },
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverRepo := inmemory.NewServerRepository()
			nodeRepo := inmemory.NewNodeRepository()
			rbacRepo := inmemory.NewRBACRepository()
			rbacService := rbac.NewRBAC(services.NewNilTransactionManager(), rbacRepo, 0)
			responder := api.NewResponder()

			arch := tt.setupArchiver()
			guard := tt.setupGuard()
			handler := NewHandler(serverRepo, nodeRepo, rbacService, arch, guard, tt.limits, responder)

			tt.setupRepo(t, serverRepo, nodeRepo, rbacRepo)

			query := url.Values{}
			if tt.queryDisk != "" {
				query.Add("disk", tt.queryDisk)
			}
			if tt.queryPath != "" {
				query.Add("path", tt.queryPath)
			}
			if tt.queryCompress != "" {
				query.Add("compress", tt.queryCompress)
			}

			fullURL := "/api/file-manager/" + tt.serverID + "/download-archive"
			if len(query) > 0 {
				fullURL += "?" + query.Encode()
			}

			req := httptest.NewRequest(http.MethodGet, fullURL, nil)
			req = req.WithContext(tt.setupCtx())
			req = mux.SetURLVars(req, map[string]string{"server": tt.serverID})
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.wantError != "" {
				assert.Contains(t, w.Body.String(), tt.wantError)
			}

			if tt.validate != nil {
				tt.validate(t, w, arch, guard)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "valid_relative", path: "data/sub", wantErr: false},
		{name: "valid_single", path: "logs", wantErr: false},
		{name: "invalid_traversal", path: "../etc", wantErr: true},
		{name: "invalid_double_dots_inside", path: "a/../b", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePath(tt.path)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestArchiveFilename(t *testing.T) {
	tests := []struct {
		name     string
		rootName string
		path     string
		want     string
	}{
		{name: "uses_root_name", rootName: "data", path: "any", want: "data.zip"},
		{name: "falls_back_to_path_basename", rootName: "", path: "logs/sub", want: "sub.zip"},
		{name: "falls_back_to_archive", rootName: ".", path: "/", want: "archive.zip"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := archiveFilename(tt.rootName, tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestContentDispositionHeader_RFC5987(t *testing.T) {
	header := contentDispositionHeader("кириллица.zip")
	assert.True(t, strings.HasPrefix(header, "attachment;"))
	assert.Contains(t, header, "filename*=UTF-8''")
}
