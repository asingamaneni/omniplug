package adapter

import (
	"fmt"
	"io/fs"
	"sort"
	"sync/atomic"
	"testing"

	"github.com/asingamaneni/omniplug/internal/model"
)

type fake struct{ name string }

func (f *fake) Name() string                          { return f.name }
func (f *fake) Capabilities() Capabilities            { return Capabilities{} }
func (f *fake) Validate(_ *model.Plugin) []Diagnostic { return nil }
func (f *fake) Compile(_ *model.Plugin) (Bundle, []Diagnostic, error) {
	return NewBundle(), nil, nil
}
func (f *fake) InstallPlan(_ *model.Plugin, _ Scope, _ string) (InstallPlan, error) {
	return InstallPlan{}, nil
}

// nameSeq gives each registration a process-unique name so tests remain
// idempotent under `go test -count=N` (the registry is process-global and
// panics on duplicate names, with no reset).
var nameSeq atomic.Int64

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, nameSeq.Add(1))
}

func TestRegisterDuplicatePanics(t *testing.T) {
	dup := uniqueName("dup")
	Register(&fake{name: dup})
	defer func() {
		if recover() == nil {
			t.Error("expected panic on duplicate Register")
		}
	}()
	Register(&fake{name: dup})
}

func TestRegistryGetAllNames(t *testing.T) {
	a, b := uniqueName("zz"), uniqueName("aa")
	Register(&fake{name: a})
	Register(&fake{name: b})
	if _, ok := Get(a); !ok {
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
	for i, ad := range all {
		if ad.Name() != names[i] {
			t.Errorf("All not in Names order at %d: %s vs %s", i, ad.Name(), names[i])
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

func TestBundleCollisions(t *testing.T) {
	b := NewBundle()
	b.Add("a.txt", []byte("1"))
	b.Add("b.txt", []byte("1"))
	if len(b.Collisions()) != 0 {
		t.Errorf("no collisions expected yet, got %v", b.Collisions())
	}
	b.Add("a.txt", []byte("2"))            // overwrite via Add
	b.AddFile("b.txt", []byte("2"), 0o644) // overwrite via AddFile
	got := b.Collisions()
	if len(got) != 2 || got[0] != "a.txt" || got[1] != "b.txt" {
		t.Errorf("Collisions() = %v, want sorted [a.txt b.txt]", got)
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
