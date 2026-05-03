package archiver_test

import (
	"context"
	"testing"

	"github.com/gameap/gameap/internal/daemon"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/services/filemanager/archiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubFileLister struct {
	listResult []*daemon.FileInfo
	listErr    error
	statResult *daemon.FileDetails
	statErr    error
}

func (s *stubFileLister) ReadDirRecursive(_ context.Context, _ *domain.Node, _ string) ([]*daemon.FileInfo, error) {
	return s.listResult, s.listErr
}

func (s *stubFileLister) GetFileInfo(_ context.Context, _ *domain.Node, _ string) (*daemon.FileDetails, error) {
	return s.statResult, s.statErr
}

func TestBuildManifest(t *testing.T) {
	t.Parallel()

	node := &domain.Node{ID: 1, WorkPath: "/work"}

	tests := []struct {
		name      string
		setupRepo func() *stubFileLister
		rootPath  string
		limits    archiver.Limits
		wantError string
		assert    func(t *testing.T, m *archiver.Manifest)
	}{
		{
			name: "success_directory_with_files",
			setupRepo: func() *stubFileLister {
				return &stubFileLister{
					statResult: &daemon.FileDetails{Type: daemon.FileTypeDir},
					listResult: []*daemon.FileInfo{
						{Path: "server-1/data/a.txt", Size: 100, Type: daemon.FileTypeFile, Perm: 0o644},
						{Path: "server-1/data/sub/b.txt", Size: 200, Type: daemon.FileTypeFile, Perm: 0o644},
						{Path: "server-1/data/sub", Type: daemon.FileTypeDir, Perm: 0o755},
					},
				}
			},
			rootPath: "/work/server-1/data",
			assert: func(t *testing.T, m *archiver.Manifest) {
				t.Helper()
				assert.Equal(t, "data", m.RootName)
				assert.Equal(t, uint64(300), m.TotalSize)
				assert.Equal(t, uint32(2), m.TotalFiles)
				require.Len(t, m.Entries, 3)
				assert.Equal(t, "data/a.txt", m.Entries[0].RelPath)
				assert.Equal(t, "data/sub/", m.Entries[1].RelPath)
				assert.Equal(t, "data/sub/b.txt", m.Entries[2].RelPath)
			},
		},
		{
			name: "success_with_symlink",
			setupRepo: func() *stubFileLister {
				return &stubFileLister{
					statResult: &daemon.FileDetails{Type: daemon.FileTypeDir},
					listResult: []*daemon.FileInfo{
						{Path: "server-1/data/link", Type: daemon.FileTypeSymlink, SymlinkTarget: "../target", Perm: 0o777},
					},
				}
			},
			rootPath: "/work/server-1/data",
			assert: func(t *testing.T, m *archiver.Manifest) {
				t.Helper()
				require.Len(t, m.Entries, 1)
				assert.Equal(t, daemon.FileTypeSymlink, m.Entries[0].Type)
				assert.Equal(t, "../target", m.Entries[0].SymlinkTarget)
				assert.Equal(t, uint32(1), m.TotalFiles)
			},
		},
		{
			name: "success_skips_special_files",
			setupRepo: func() *stubFileLister {
				return &stubFileLister{
					statResult: &daemon.FileDetails{Type: daemon.FileTypeDir},
					listResult: []*daemon.FileInfo{
						{Path: "server-1/data/regular.txt", Size: 10, Type: daemon.FileTypeFile, Perm: 0o644},
						{Path: "server-1/data/socket", Type: daemon.FileTypeSocket},
						{Path: "server-1/data/fifo", Type: daemon.FileTypeNamedPipe},
					},
				}
			},
			rootPath: "/work/server-1/data",
			assert: func(t *testing.T, m *archiver.Manifest) {
				t.Helper()
				require.Len(t, m.Entries, 1)
				require.Len(t, m.Skipped, 2)
				assert.Contains(t, m.Skipped, "data/socket")
				assert.Contains(t, m.Skipped, "data/fifo")
			},
		},
		{
			name: "error_total_size_exceeds_limit",
			setupRepo: func() *stubFileLister {
				return &stubFileLister{
					statResult: &daemon.FileDetails{Type: daemon.FileTypeDir},
					listResult: []*daemon.FileInfo{
						{Path: "server-1/data/big.bin", Size: 1000, Type: daemon.FileTypeFile, Perm: 0o644},
					},
				}
			},
			rootPath:  "/work/server-1/data",
			limits:    archiver.Limits{MaxTotalBytes: 500},
			wantError: "archive total size exceeds limit",
		},
		{
			name: "error_total_files_exceeds_limit",
			setupRepo: func() *stubFileLister {
				files := make([]*daemon.FileInfo, 6)
				for i := range files {
					files[i] = &daemon.FileInfo{
						Path: "server-1/data/f.txt",
						Size: 1,
						Type: daemon.FileTypeFile,
						Perm: 0o644,
					}
				}
				files[0].Path = "server-1/data/f1.txt"
				files[1].Path = "server-1/data/f2.txt"
				files[2].Path = "server-1/data/f3.txt"
				files[3].Path = "server-1/data/f4.txt"
				files[4].Path = "server-1/data/f5.txt"
				files[5].Path = "server-1/data/f6.txt"

				return &stubFileLister{
					statResult: &daemon.FileDetails{Type: daemon.FileTypeDir},
					listResult: files,
				}
			},
			rootPath:  "/work/server-1/data",
			limits:    archiver.Limits{MaxFiles: 3},
			wantError: "archive total file count exceeds limit",
		},
		{
			name: "error_not_a_directory",
			setupRepo: func() *stubFileLister {
				return &stubFileLister{
					statResult: &daemon.FileDetails{Type: daemon.FileTypeFile},
				}
			},
			rootPath:  "/work/server-1/data/file.txt",
			wantError: "requested path is not a directory",
		},
		{
			name: "error_empty_directory",
			setupRepo: func() *stubFileLister {
				return &stubFileLister{
					statResult: &daemon.FileDetails{Type: daemon.FileTypeDir},
					listResult: []*daemon.FileInfo{},
				}
			},
			rootPath:  "/work/server-1/data",
			wantError: "nothing to archive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lister := tt.setupRepo()
			m, err := archiver.BuildManifest(context.Background(), lister, node, tt.rootPath, tt.limits)

			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, m)
			if tt.assert != nil {
				tt.assert(t, m)
			}
		})
	}
}
