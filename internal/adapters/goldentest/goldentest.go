// Package goldentest compares a compiled Bundle against a directory of golden
// files, byte for byte in both directions. It is shared by the adapter test
// suites and is itself test-only (no production code imports it).
package goldentest

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/asingamaneni/omniplug/internal/adapter"
)

// intendedMode returns the on-disk mode the installer would apply to rel: the
// bundle's explicit mode when set, else the default 0644. Goldens are stored at
// this mode so the exec-bit half of the Bundle contract is pinned too.
func intendedMode(b adapter.Bundle, rel string) os.FileMode {
	if m, ok := b.Modes[rel]; ok {
		return m
	}
	return 0o644
}

// Compare asserts that the bundle and the golden directory hold the same file
// set with identical contents and modes. With update=true it rewrites the
// golden directory from the bundle instead.
func Compare(t *testing.T, b adapter.Bundle, goldenDir string, update bool) {
	t.Helper()
	if update {
		if err := os.RemoveAll(goldenDir); err != nil {
			t.Fatalf("clearing golden dir: %v", err)
		}
		for rel, content := range b.Files {
			dest := filepath.Join(goldenDir, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				t.Fatalf("mkdir for %s: %v", rel, err)
			}
			mode := intendedMode(b, rel)
			if err := os.WriteFile(dest, content, mode); err != nil {
				t.Fatalf("writing golden %s: %v", rel, err)
			}
			// WriteFile only sets mode on create; chmod so regenerated goldens
			// carry the intended exec bit even when overwritten.
			if err := os.Chmod(dest, mode); err != nil {
				t.Fatalf("chmod golden %s: %v", rel, err)
			}
		}
		t.Logf("regenerated %d golden files under %s", len(b.Files), goldenDir)
		return
	}

	// Bundle -> golden: every compiled file must match its golden twin in both
	// content and mode.
	for rel, content := range b.Files {
		goldenPath := filepath.Join(goldenDir, filepath.FromSlash(rel))
		info, err := os.Stat(goldenPath)
		if err != nil {
			t.Errorf("missing golden for %s (run with -update to regenerate): %v", rel, err)
			continue
		}
		want, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Errorf("reading golden for %s: %v", rel, err)
			continue
		}
		if !bytes.Equal(content, want) {
			t.Errorf("output differs from golden for %s:\n--- golden ---\n%s\n--- got ---\n%s",
				rel, want, content)
		}
		if got, wantMode := info.Mode().Perm(), intendedMode(b, rel).Perm(); got != wantMode {
			t.Errorf("mode differs from golden for %s: got %o, want %o (run with -update)", rel, got, wantMode)
		}
	}

	// Golden -> bundle: no stale golden files may linger.
	err := filepath.Walk(goldenDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(goldenDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if _, ok := b.Files[rel]; !ok {
			t.Errorf("stale golden file %s not produced by Compile (run with -update)", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking golden dir %s: %v (run with -update to create it)", goldenDir, err)
	}
}
