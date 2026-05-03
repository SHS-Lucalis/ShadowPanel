package archiver_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/gameap/gameap/internal/daemon"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/services/filemanager/archiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type contentLister struct {
	files map[string]string
	stat  *daemon.FileDetails
}

func (l *contentLister) ReadDirRecursive(_ context.Context, _ *domain.Node, _ string) ([]*daemon.FileInfo, error) {
	out := make([]*daemon.FileInfo, 0, len(l.files))
	for path, content := range l.files {
		out = append(out, &daemon.FileInfo{
			Path: path,
			Size: uint64(len(content)),
			Type: daemon.FileTypeFile,
			Perm: 0o644,
		})
	}

	return out, nil
}

func (l *contentLister) GetFileInfo(_ context.Context, _ *domain.Node, _ string) (*daemon.FileDetails, error) {
	return l.stat, nil
}

type contentStreamer struct {
	files map[string]string
}

func (s *contentStreamer) DownloadStream(_ context.Context, _ *domain.Node, filePath string) (io.ReadCloser, error) {
	rel := strings.TrimPrefix(filePath, "/work/")
	body, ok := s.files[rel]
	if !ok {
		return io.NopCloser(strings.NewReader("")), nil
	}

	return io.NopCloser(strings.NewReader(body)), nil
}

func TestArchiver_WriteArchive(t *testing.T) {
	t.Parallel()

	node := &domain.Node{ID: 1, WorkPath: "/work"}

	t.Run("writes_zip_with_correct_paths_and_content", func(t *testing.T) {
		t.Parallel()

		files := map[string]string{
			"server-1/data/a.txt":     "alpha",
			"server-1/data/sub/b.txt": "bravo",
		}
		lister := &contentLister{files: files, stat: &daemon.FileDetails{Type: daemon.FileTypeDir}}
		streamer := &contentStreamer{files: files}

		a := archiver.NewArchiver(lister, streamer, nil)
		manifest, err := a.BuildManifest(context.Background(), node, "/work/server-1/data", archiver.Limits{})
		require.NoError(t, err)

		var buf bytes.Buffer
		result, err := a.WriteArchive(context.Background(), &buf, node, manifest, archiver.Options{})
		require.NoError(t, err)
		assert.Greater(t, result.BytesWritten, uint64(0))

		zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		require.NoError(t, err)

		got := make(map[string]string)
		for _, f := range zr.File {
			rc, openErr := f.Open()
			require.NoError(t, openErr)
			body, readErr := io.ReadAll(rc)
			_ = rc.Close()
			require.NoError(t, readErr)
			got[f.Name] = string(body)
		}

		assert.Equal(t, "alpha", got["data/a.txt"])
		assert.Equal(t, "bravo", got["data/sub/b.txt"])
	})

	t.Run("writes_skipped_report_when_special_files_present", func(t *testing.T) {
		t.Parallel()

		lister := &mixedLister{
			stat: &daemon.FileDetails{Type: daemon.FileTypeDir},
			items: []*daemon.FileInfo{
				{Path: "server-1/data/regular.txt", Size: 5, Type: daemon.FileTypeFile, Perm: 0o644},
				{Path: "server-1/data/socket", Type: daemon.FileTypeSocket},
			},
		}
		streamer := &contentStreamer{files: map[string]string{"server-1/data/regular.txt": "hello"}}

		a := archiver.NewArchiver(lister, streamer, nil)
		manifest, err := a.BuildManifest(context.Background(), node, "/work/server-1/data", archiver.Limits{})
		require.NoError(t, err)

		var buf bytes.Buffer
		_, err = a.WriteArchive(context.Background(), &buf, node, manifest, archiver.Options{})
		require.NoError(t, err)

		zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		require.NoError(t, err)

		var foundSkipped bool
		for _, f := range zr.File {
			if f.Name == "_SKIPPED.txt" {
				foundSkipped = true
				rc, _ := f.Open()
				body, _ := io.ReadAll(rc)
				_ = rc.Close()
				assert.Contains(t, string(body), "data/socket")
			}
		}
		assert.True(t, foundSkipped, "_SKIPPED.txt entry expected")
	})

	t.Run("preserves_symlinks_as_zip_symlink_entries", func(t *testing.T) {
		t.Parallel()

		lister := &mixedLister{
			stat: &daemon.FileDetails{Type: daemon.FileTypeDir},
			items: []*daemon.FileInfo{
				{Path: "server-1/data/link", Type: daemon.FileTypeSymlink, SymlinkTarget: "../target"},
			},
		}
		streamer := &contentStreamer{files: map[string]string{}}

		a := archiver.NewArchiver(lister, streamer, nil)
		manifest, err := a.BuildManifest(context.Background(), node, "/work/server-1/data", archiver.Limits{})
		require.NoError(t, err)

		var buf bytes.Buffer
		_, err = a.WriteArchive(context.Background(), &buf, node, manifest, archiver.Options{})
		require.NoError(t, err)

		zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		require.NoError(t, err)

		require.Len(t, zr.File, 1)
		entry := zr.File[0]
		assert.Equal(t, "data/link", entry.Name)
		assert.NotZero(t, entry.Mode()&os.ModeSymlink, "symlink mode bit must be set")

		rc, _ := entry.Open()
		body, _ := io.ReadAll(rc)
		_ = rc.Close()
		assert.Equal(t, "../target", string(body))
	})
}

type mixedLister struct {
	stat  *daemon.FileDetails
	items []*daemon.FileInfo
}

func (m *mixedLister) ReadDirRecursive(_ context.Context, _ *domain.Node, _ string) ([]*daemon.FileInfo, error) {
	return m.items, nil
}

func (m *mixedLister) GetFileInfo(_ context.Context, _ *domain.Node, _ string) (*daemon.FileDetails, error) {
	return m.stat, nil
}
