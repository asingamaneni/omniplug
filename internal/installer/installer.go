// Package installer writes a compiled Bundle to disk, supporting dry-run.
//
// All writes are confined under the install root: a bundle path that would
// escape the root (via `..` or an absolute path) is rejected. This is the last
// line of defense against a malicious plugin source attempting a path-traversal
// (zip-slip) write outside the intended output directory.
package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/asingamaneni/omniplug/internal/adapter"
)

// WriteResult records what a write operation did (or would do, when dry-run).
type WriteResult struct {
	Root    string
	Written []string // relative paths, sorted
	DryRun  bool
}

// Write writes every file in the bundle under root. When dryRun is true, it only
// reports the paths it would create without touching disk.
func Write(b adapter.Bundle, root string, dryRun bool) (WriteResult, error) {
	res := WriteResult{Root: root, DryRun: dryRun}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return res, err
	}

	paths := make([]string, 0, len(b.Files))
	for p := range b.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, rel := range paths {
		dest, err := safeJoin(absRoot, rel)
		if err != nil {
			return res, err
		}
		res.Written = append(res.Written, rel)
		if dryRun {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return res, fmt.Errorf("creating dir for %s: %w", rel, err)
		}
		mode := os.FileMode(0o644)
		if m, ok := b.Modes[rel]; ok {
			mode = m
		}
		if err := os.WriteFile(dest, b.Files[rel], mode); err != nil {
			return res, fmt.Errorf("writing %s: %w", rel, err)
		}
	}
	return res, nil
}

// safeJoin joins rel under absRoot and verifies the result stays within absRoot.
// It rejects absolute paths and any path that escapes via `..`.
func safeJoin(absRoot, rel string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("refusing absolute bundle path %q", rel)
	}
	dest := filepath.Join(absRoot, clean)
	within, err := filepath.Rel(absRoot, dest)
	if err != nil {
		return "", err
	}
	if within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("refusing to write outside install root: %q", rel)
	}
	return dest, nil
}
