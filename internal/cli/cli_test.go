package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asingamaneni/omniplug/internal/adapter"
	"github.com/asingamaneni/omniplug/internal/model"

	// The real adapters register here the same way cmd/omniplug wires them.
	_ "github.com/asingamaneni/omniplug/internal/adapters/claude"
	_ "github.com/asingamaneni/omniplug/internal/adapters/cursor"
)

const examplePath = "../../examples/hello-plugin"

// fakeErrAdapter emits an error-severity diagnostic so tests can exercise the
// build/install error gating. It pollutes target "all" for this test binary,
// so every test passes explicit -t claude,cursor.
type fakeErrAdapter struct{}

func (f *fakeErrAdapter) Name() string                                  { return "fake-err" }
func (f *fakeErrAdapter) Capabilities() adapter.Capabilities            { return adapter.Capabilities{} }
func (f *fakeErrAdapter) Validate(_ *model.Plugin) []adapter.Diagnostic { return nil }
func (f *fakeErrAdapter) Compile(_ *model.Plugin) (adapter.Bundle, []adapter.Diagnostic, error) {
	b := adapter.NewBundle()
	b.Add("boom.txt", []byte("x"))
	return b, []adapter.Diagnostic{adapter.Error("fake-err", "plugin", "boom")}, nil
}
func (f *fakeErrAdapter) InstallPlan(_ *model.Plugin, _ adapter.Scope, projectDir string) (adapter.InstallPlan, error) {
	return adapter.InstallPlan{Root: filepath.Join(projectDir, "fake-err")}, nil
}

func TestMain(m *testing.M) {
	adapter.Register(&fakeErrAdapter{})
	os.Exit(m.Run())
}

// run executes the CLI in-process, capturing os.Stdout/os.Stderr (the commands
// print with fmt.Printf, not through cobra's writers).
func run(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout, os.Stderr = wOut, wErr
	defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()

	root := newRootCmd()
	root.SetOut(wOut)
	root.SetErr(wErr)
	root.SetArgs(args)
	err = root.Execute()

	_ = wOut.Close()
	_ = wErr.Close()
	outB, _ := io.ReadAll(rOut)
	errB, _ := io.ReadAll(rErr)
	return string(outB), string(errB), err
}

func TestInitRefusesOverwriteUnlessForced(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := run(t, "init", "demo", "-d", dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	for _, f := range []string{"plugin.yaml", "skills/hello/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(dir, "demo", f)); err != nil {
			t.Errorf("scaffold missing %s: %v", f, err)
		}
	}
	if _, _, err := run(t, "init", "demo", "-d", dir); err == nil {
		t.Error("re-init must refuse to overwrite")
	} else if !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Errorf("unexpected refusal message: %v", err)
	}
	if _, _, err := run(t, "init", "demo", "-d", dir, "--force"); err != nil {
		t.Errorf("init --force should overwrite: %v", err)
	}
}

func TestValidateShowsDegradationWarnings(t *testing.T) {
	_, stderr, err := run(t, "validate", "-s", examplePath, "-t", "claude,cursor")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !strings.Contains(stderr, "warning") || !strings.Contains(stderr, "cursor") {
		t.Errorf("validate must print the degradation warnings build prints:\n%s", stderr)
	}
}

func TestValidateMissingSourceFriendlyError(t *testing.T) {
	_, _, err := run(t, "validate", "-s", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "no plugin.yaml found") {
		t.Errorf("expected friendly missing-manifest error, got: %v", err)
	}
}

func TestBuildWritesSelectedTargets(t *testing.T) {
	out := filepath.Join(t.TempDir(), "dist")
	stdout, _, err := run(t, "build", "-s", examplePath, "-o", out, "-t", "claude,cursor")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	for _, f := range []string{
		"claude/.claude-plugin/plugin.json",
		"claude/hooks/hooks.json",
		"cursor/.cursor/hooks.json",
		"cursor/.cursor/agents/code-reviewer.md",
	} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(f))); err != nil {
			t.Errorf("built output missing %s: %v", f, err)
		}
	}
	// The example compiles to multiple files per target, so the output must use
	// the pluralized countNoun form ("N files"), confirming it is wired in.
	if !strings.Contains(stdout, "files)") || !strings.Contains(stdout, "built claude") {
		t.Errorf("build output missing pluralized file count:\n%s", stdout)
	}
}

func TestBuildUnknownTargetErrors(t *testing.T) {
	_, _, err := run(t, "build", "-s", examplePath, "-o", t.TempDir(), "-t", "no-such")
	if err == nil || !strings.Contains(err.Error(), "unknown target") {
		t.Errorf("expected unknown-target error, got: %v", err)
	}
}

func TestBuildGatesOnErrorDiagnostics(t *testing.T) {
	out := filepath.Join(t.TempDir(), "dist")
	_, _, err := run(t, "build", "-s", examplePath, "-o", out, "-t", "fake-err")
	if err == nil || !strings.Contains(err.Error(), "failed with errors") {
		t.Fatalf("expected error-gating failure, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(out, "fake-err")); !os.IsNotExist(statErr) {
		t.Error("nothing must be written for a target with error diagnostics")
	}
}

func TestInstallDryRunWritesNothing(t *testing.T) {
	proj := filepath.Join(t.TempDir(), "proj")
	_, _, err := run(t, "install", "-s", examplePath, "--scope", "project",
		"--project-dir", proj, "--dry-run", "-t", "claude,cursor")
	if err != nil {
		t.Fatalf("install --dry-run: %v", err)
	}
	if _, statErr := os.Stat(proj); !os.IsNotExist(statErr) {
		t.Error("dry-run must not create the project directory")
	}
}

func TestInstallInvalidScopeErrors(t *testing.T) {
	_, _, err := run(t, "install", "-s", examplePath, "--scope", "banana", "-t", "claude")
	if err == nil || !strings.Contains(err.Error(), "invalid --scope") {
		t.Errorf("expected invalid-scope error, got: %v", err)
	}
}

func TestVersionFlag(t *testing.T) {
	stdout, _, err := run(t, "--version")
	if err != nil {
		t.Fatalf("--version: %v", err)
	}
	if !strings.Contains(stdout, "omniplug version") {
		t.Errorf("unexpected --version output: %q", stdout)
	}
}

func TestCountNoun(t *testing.T) {
	if got := countNoun(1, "file"); got != "1 file" {
		t.Errorf("countNoun(1) = %q", got)
	}
	if got := countNoun(3, "file"); got != "3 files" {
		t.Errorf("countNoun(3) = %q", got)
	}
}

// TestBuildSingularFileCount exercises the n==1 wiring end-to-end: a
// guidance-only plugin compiles to exactly one Cursor file, so the output must
// read "1 file" (not "1 files").
func TestBuildSingularFileCount(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "plugin.yaml"), []byte("name: solo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "guidance"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "guidance", "AGENTS.md"), []byte("Be careful.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := run(t, "build", "-s", src, "-o", filepath.Join(t.TempDir(), "dist"), "-t", "cursor")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.Contains(stdout, "(1 file)") {
		t.Errorf("single-file build should read '1 file':\n%s", stdout)
	}
}

// TestEndToEnd drives the full pipeline: init -> validate -> build -> install --dry-run.
func TestEndToEnd(t *testing.T) {
	tmp := t.TempDir()
	if _, _, err := run(t, "init", "e2e-plugin", "-d", tmp); err != nil {
		t.Fatalf("init: %v", err)
	}
	src := filepath.Join(tmp, "e2e-plugin")
	if _, _, err := run(t, "validate", "-s", src, "-t", "claude,cursor"); err != nil {
		t.Fatalf("validate: %v", err)
	}
	dist := filepath.Join(tmp, "dist")
	if _, _, err := run(t, "build", "-s", src, "-o", dist, "-t", "claude,cursor"); err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dist, "claude", "skills", "hello", "SKILL.md")); err != nil {
		t.Errorf("built claude skill missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dist, "cursor", ".cursor", "skills", "hello", "SKILL.md")); err != nil {
		t.Errorf("built cursor skill missing: %v", err)
	}
	proj := filepath.Join(tmp, "proj")
	if _, _, err := run(t, "install", "-s", src, "--scope", "project",
		"--project-dir", proj, "--dry-run", "-t", "claude,cursor"); err != nil {
		t.Fatalf("install --dry-run: %v", err)
	}
	if _, statErr := os.Stat(proj); !os.IsNotExist(statErr) {
		t.Error("dry-run must not create the project dir")
	}
}
