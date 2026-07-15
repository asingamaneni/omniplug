package adapter

import (
	"io/fs"
	"sort"
	"testing"

	"github.com/asingamaneni/omniplug/internal/model"
)

type fake struct{ name string }

func (f *fake) Name() string                          { return f.name }
func (f *fake) Capabilities() Capabilities            { return Capabilities{} }
func (f *fake) Validate(p *model.Plugin) []Diagnostic { return nil }
func (f *fake) Compile(p *model.Plugin) (Bundle, []Diagnostic, error) {
	return NewBundle(), nil, nil
}
func (f *fake) InstallPlan(p *model.Plugin, s Scope, projectDir string) (InstallPlan, error) {
	return InstallPlan{}, nil
}

func TestRegisterDuplicatePanics(t *testing.T) {
	Register(&fake{name: "dup-test"})
	defer func() {
		if recover() == nil {
			t.Error("expected panic on duplicate Register")
		}
	}()
	Register(&fake{name: "dup-test"})
}

func TestRegistryGetAllNames(t *testing.T) {
	Register(&fake{name: "zz-test"})
	Register(&fake{name: "aa-test"})
	if _, ok := Get("zz-test"); !ok {
		t.Error("Get should find a registered adapter")
	}
	if _, ok := Get("never-registered"); ok {
		t.Error("Get should miss an unregistered name")
	}
	names := Names()
	if !sort.StringsAreSorted(names) {
		t.Errorf("Names must be sorted: %v", names)
	}
	all := All()
	if len(all) != len(names) {
		t.Fatalf("All returned %d adapters, Names %d", len(all), len(names))
	}
	for i, a := range all {
		if a.Name() != names[i] {
			t.Errorf("All not in Names order at %d: %s vs %s", i, a.Name(), names[i])
		}
	}
}

func TestScriptModeSanitizes(t *testing.T) {
	cases := []struct {
		in   fs.FileMode
		want fs.FileMode
	}{
		{0o644, 0o644},
		{0o600, 0o644},
		{0o755, 0o755},
		{0o700, 0o755},
		{0o755 | fs.ModeSetuid, 0o755}, // setuid never propagates
		{0o755 | fs.ModeSetgid, 0o755},
		{0o644 | fs.ModeSticky, 0o644},
	}
	for _, tc := range cases {
		if got := ScriptMode(tc.in); got != tc.want {
			t.Errorf("ScriptMode(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestBundleAddFileSanitizesMode(t *testing.T) {
	b := NewBundle()
	b.AddFile("x.sh", []byte("hi"), 0o755|fs.ModeSetuid)
	if b.Modes["x.sh"] != 0o755 {
		t.Errorf("AddFile mode = %v, want 0755", b.Modes["x.sh"])
	}
	if string(b.Files["x.sh"]) != "hi" {
		t.Errorf("AddFile content lost")
	}
}

func TestHasErrors(t *testing.T) {
	warns := []Diagnostic{Warn("x", "c", "w")}
	if HasErrors(warns) {
		t.Error("warnings alone must not count as errors")
	}
	if !HasErrors(append(warns, Error("x", "c", "e"))) {
		t.Error("an error diagnostic must be detected")
	}
}
