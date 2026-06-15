// Command shslint enforces SHS Panel core boundary rules.
//
// Currently it scans the core panel source tree for forbidden game-name
// substrings (defined in .shs/lint/forbidden-strings.txt). Game-specific
// names belong only under plugins/ and templates/. See
// docs/architecture/00-shs-panel-architecture-plan.md §8.2.
//
// This file lives under .shs/tools/ so the Go toolchain's "./..."
// expansion (which ignores directories beginning with '.') does not
// include it in panel builds.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Roots that are considered "core panel" for boundary checks. Game-specific
// names must not appear inside any of these.
var coreRoots = []string{
	"cmd",
	"internal/api",
	"internal/app",
	"internal/application",
	"internal/domain",
	"internal/repositories",
	"internal/services",
	"internal/ws",
	"migrations",
	"openapi",
	"web/frontend",
}

// pkgExceptions: subtrees under pkg/ that are SHS-owned and may legitimately
// reference plugin ids (e.g. in doc comments, hook examples).
var pkgExceptions = []string{
	"pkg/shspluginsdk",
}

// File extensions worth scanning. Code and structured-config files only.
// Asset files (CSS, SVG, fonts, images) and lockfiles are skipped — game
// names legitimately appear in icon-font glyph definitions and similar
// generic infrastructure.
var scanExts = map[string]bool{
	".go":   true,
	".ts":   true,
	".tsx":  true,
	".js":   true,
	".jsx":  true,
	".vue":  true,
	".sql":  true,
	".yaml": true,
	".yml":  true,
}

func main() {
	configPath := flag.String("config", ".shs/lint/forbidden-strings.txt", "path to forbidden-strings list")
	flag.Parse()

	terms, err := loadTerms(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shslint: %v\n", err)
		os.Exit(2)
	}
	if len(terms) == 0 {
		fmt.Fprintln(os.Stderr, "shslint: no forbidden terms configured; nothing to check")
		return
	}

	roots := append([]string(nil), coreRoots...)
	roots = append(roots, pkgRootsExcluding(pkgExceptions)...)

	violations := 0
	for _, root := range roots {
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				name := d.Name()
				if name == "node_modules" || name == "vendor" || name == "dist" || name == ".git" {
					return fs.SkipDir
				}
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if !scanExts[ext] {
				return nil
			}
			n, err := scanFile(path, terms)
			if err != nil {
				return err
			}
			violations += n
			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "shslint: walk %s: %v\n", root, err)
			os.Exit(2)
		}
	}

	if violations > 0 {
		fmt.Fprintf(os.Stderr, "\nshslint: %d violation(s) found. Game-specific names belong under plugins/ or templates/.\n", violations)
		os.Exit(1)
	}
	fmt.Println("[shs-lint] core boundary ok")
}

func loadTerms(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var terms []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		terms = append(terms, strings.ToLower(line))
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return terms, nil
}

func pkgRootsExcluding(excluded []string) []string {
	pkgRoot := "pkg"
	if _, err := os.Stat(pkgRoot); os.IsNotExist(err) {
		return nil
	}
	excludedSet := make(map[string]bool, len(excluded))
	for _, e := range excluded {
		excludedSet[filepath.ToSlash(e)] = true
	}

	entries, err := os.ReadDir(pkgRoot)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		full := filepath.ToSlash(filepath.Join(pkgRoot, e.Name()))
		if excludedSet[full] {
			continue
		}
		out = append(out, full)
	}
	return out
}

func scanFile(path string, terms []string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	violations := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		lower := strings.ToLower(sc.Text())
		for _, term := range terms {
			if strings.Contains(lower, term) {
				rel, _ := filepath.Rel(".", path)
				fmt.Fprintf(os.Stderr, "%s:%d: forbidden term %q\n", filepath.ToSlash(rel), lineNo, term)
				violations++
				break
			}
		}
	}
	return violations, sc.Err()
}
