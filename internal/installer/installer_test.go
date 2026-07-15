package installer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/asingamaneni/omniplug/internal/adapter"
)

func TestRejectsTraversalPath(t *testing.T) {
	b := adapter.NewBundle()
	b.Add("../escape.txt", []byte("x"))
	if _, err := Write(b, t.TempDir(), false); err == nil {
		t.Fatal("expected Write to reject a path escaping the install root")
	}
}

func TestRejectsAbsolutePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("absolute path form differs on windows")
	}
	b := adapter.NewBundle()
	b.Add("/etc/evil", []byte("x"))
	if _, err := Write(b, t.TempDir(), false); err == nil {
		t.Fatal("expected Write to reject an absolute bundle path")
	}
}

func TestWritesFileModes(t *testing.T) {
	b := adapter.NewBundle()
	b.AddFile("scripts/run.sh", []byte("#!/bin/sh\n"), 0o755)
	b.Add("README.md", []byte("hi"))
	root := t.TempDir()
	if _, err := Write(b, root, false); err != nil {
		t.Fatalf("Write: %v", err)
	}
	script, err := os.Stat(filepath.Join(root, "scripts", "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && script.Mode().Perm() != 0o755 {
		t.Errorf("script mode = %v, want 0755", script.Mode().Perm())
	}
	readme, err := os.Stat(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && readme.Mode().Perm() != 0o644 {
		t.Errorf("readme mode = %v, want 0644", readme.Mode().Perm())
	}
}
