package archiver

import (
	"context"
	"path"
	"sort"
	"strings"

	"github.com/gameap/gameap/internal/daemon"
	"github.com/gameap/gameap/internal/domain"
	"github.com/pkg/errors"
)

var (
	ErrTooLarge      = errors.New("archive total size exceeds limit")
	ErrTooManyFiles  = errors.New("archive total file count exceeds limit")
	ErrEmptyManifest = errors.New("nothing to archive")
	ErrNotADirectory = errors.New("requested path is not a directory")
)

type Entry struct {
	AbsPath       string
	RelPath       string
	Size          uint64
	Mode          uint32
	ModTime       uint64
	Type          daemon.FileType
	SymlinkTarget string
}

type Manifest struct {
	Entries    []Entry
	Skipped    []string
	TotalSize  uint64
	TotalFiles uint32
	RootName   string
}

type Limits struct {
	MaxTotalBytes uint64
	MaxFiles      uint32
}

func BuildManifest(
	ctx context.Context,
	lister FileLister,
	node *domain.Node,
	rootAbsPath string,
	limits Limits,
) (*Manifest, error) {
	rootInfo, err := lister.GetFileInfo(ctx, node, rootAbsPath)
	if err != nil {
		return nil, errors.WithMessage(err, "stat root path")
	}
	if rootInfo.Type != daemon.FileTypeDir {
		return nil, ErrNotADirectory
	}

	rootRel := stripWorkPath(node.WorkPath, rootAbsPath)
	rootBase := path.Base(rootRel)
	if rootBase == "" || rootBase == "." || rootBase == "/" {
		rootBase = "archive"
	}

	files, err := lister.ReadDirRecursive(ctx, node, rootAbsPath)
	if err != nil {
		return nil, errors.WithMessage(err, "list directory recursive")
	}

	manifest := &Manifest{RootName: rootBase}

	for _, f := range files {
		relInArchive := relativeArchivePath(rootRel, rootBase, f.Path)
		if relInArchive == "" {
			continue
		}

		entry := Entry{
			AbsPath:       path.Join(node.WorkPath, f.Path),
			RelPath:       relInArchive,
			Size:          f.Size,
			Mode:          f.Perm,
			ModTime:       f.TimeModified,
			Type:          f.Type,
			SymlinkTarget: f.SymlinkTarget,
		}

		switch f.Type {
		case daemon.FileTypeFile:
			manifest.Entries = append(manifest.Entries, entry)
			manifest.TotalSize += f.Size
			manifest.TotalFiles++
		case daemon.FileTypeDir:
			entry.RelPath = ensureTrailingSlash(entry.RelPath)
			manifest.Entries = append(manifest.Entries, entry)
		case daemon.FileTypeSymlink:
			manifest.Entries = append(manifest.Entries, entry)
			manifest.TotalFiles++
		default:
			manifest.Skipped = append(manifest.Skipped, relInArchive)
		}
	}

	if len(manifest.Entries) == 0 {
		return nil, ErrEmptyManifest
	}

	if limits.MaxTotalBytes > 0 && manifest.TotalSize > limits.MaxTotalBytes {
		return nil, errors.Wrapf(
			ErrTooLarge,
			"%d bytes > limit %d", manifest.TotalSize, limits.MaxTotalBytes,
		)
	}

	if limits.MaxFiles > 0 && manifest.TotalFiles > limits.MaxFiles {
		return nil, errors.Wrapf(
			ErrTooManyFiles,
			"%d files > limit %d", manifest.TotalFiles, limits.MaxFiles,
		)
	}

	sort.Slice(manifest.Entries, func(i, j int) bool {
		return manifest.Entries[i].RelPath < manifest.Entries[j].RelPath
	})

	return manifest, nil
}

func relativeArchivePath(rootRel, rootBase, fullRel string) string {
	cleanFull := strings.TrimPrefix(fullRel, "/")
	cleanRoot := strings.TrimPrefix(rootRel, "/")

	if cleanFull == "" || cleanFull == cleanRoot {
		return ""
	}

	rest := strings.TrimPrefix(cleanFull, cleanRoot)
	rest = strings.TrimPrefix(rest, "/")
	if rest == "" {
		return ""
	}

	return path.Join(rootBase, rest)
}

func ensureTrailingSlash(p string) string {
	if strings.HasSuffix(p, "/") {
		return p
	}

	return p + "/"
}

func stripWorkPath(workPath, fullPath string) string {
	rel := strings.TrimPrefix(fullPath, workPath)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		return "."
	}

	return rel
}
