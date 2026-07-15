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

// Compare asserts that the bundle and the golden directory hold the same file
// set with identical contents. With update=true it rewrites the golden
// directory from the bundle instead.
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
			if err := os.WriteFile(dest, content, 0o644); err != nil {
				t.Fatalf("writing golden %s: %v", rel, err)
			}
		}
		t.Logf("regenerated %d golden files under %s", len(b.Files), goldenDir)
		return
	}

	// Bundle -> golden: every compiled file must match its golden twin.
	for rel, content := range b.Files {
		goldenPath := filepath.Join(goldenDir, filepath.FromSlash(rel))
		want, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Errorf("missing golden for %s (run with -update to regenerate): %v", rel, err)
			continue
		}
		if !bytes.Equal(content, want) {
			t.Errorf("output differs from golden for %s:\n--- golden ---\n%s\n--- got ---\n%s",
				rel, want, content)
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
