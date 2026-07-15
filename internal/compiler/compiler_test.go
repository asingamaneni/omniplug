package compiler

import (
	"os"
	"strings"
	"testing"

	"github.com/asingamaneni/omniplug/internal/adapter"
	"github.com/asingamaneni/omniplug/internal/model"
)

// fakeAdapter is a minimal adapter for exercising the compiler. The registry is
// global to the test binary, so every test passes explicit target names.
type fakeAdapter struct {
	name  string
	files map[string][]byte
	diags []adapter.Diagnostic
}

func (f *fakeAdapter) Name() string                                  { return f.name }
func (f *fakeAdapter) Capabilities() adapter.Capabilities            { return adapter.Capabilities{} }
func (f *fakeAdapter) Validate(_ *model.Plugin) []adapter.Diagnostic { return nil }
func (f *fakeAdapter) Compile(_ *model.Plugin) (adapter.Bundle, []adapter.Diagnostic, error) {
	b := adapter.NewBundle()
	for k, v := range f.files {
		b.Add(k, v)
	}
	return b, f.diags, nil
}
func (f *fakeAdapter) InstallPlan(_ *model.Plugin, _ adapter.Scope, projectDir string) (adapter.InstallPlan, error) {
	return adapter.InstallPlan{Root: projectDir}, nil
}

func TestMain(m *testing.M) {
	adapter.Register(&fakeAdapter{name: "fake-ok", files: map[string][]byte{"a.txt": []byte("x")}})
	adapter.Register(&fakeAdapter{
		name:  "fake-err",
		files: map[string][]byte{"a.txt": []byte("x")},
		diags: []adapter.Diagnostic{adapter.Error("fake-err", "x", "boom")},
	})
	adapter.Register(&fakeAdapter{name: "fake-empty"})
	os.Exit(m.Run())
}

func validPlugin() *model.Plugin {
	return &model.Plugin{Name: "ok", APIVersion: model.APIVersion}
}

func TestResolveTargetsAllAndExplicit(t *testing.T) {
	all, err := ResolveTargets(nil)
	if err != nil {
		t.Fatalf("ResolveTargets(nil): %v", err)
	}
	found := map[string]bool{}
	for _, n := range all {
		found[n] = true
	}
	if !found["fake-ok"] || !found["fake-err"] {
		t.Errorf("nil should expand to all registered adapters, got %v", all)
	}
	viaAll, err := ResolveTargets([]string{"all"})
	if err != nil || len(viaAll) != len(all) {
		t.Errorf(`ResolveTargets(["all"]) = %v, %v; want same as nil`, viaAll, err)
	}
	explicit, err := ResolveTargets([]string{"fake-ok"})
	if err != nil || len(explicit) != 1 || explicit[0] != "fake-ok" {
		t.Errorf("explicit resolve = %v, %v", explicit, err)
	}
}

func TestResolveTargetsUnknownErrors(t *testing.T) {
	if _, err := ResolveTargets([]string{"no-such-target"}); err == nil {
		t.Error("expected error for unknown target")
	} else if !strings.Contains(err.Error(), "no-such-target") {
		t.Errorf("error should name the unknown target: %v", err)
	}
}

func TestSchemaErrorAbortsCompilation(t *testing.T) {
	results, diags, err := Compile(&model.Plugin{}, []string{"fake-ok"})
	if err == nil {
		t.Fatal("expected error for plugin without a name")
	}
	if results != nil {
		t.Errorf("no results should be produced on schema failure, got %v", results)
	}
	if !adapter.HasErrors(diags) {
		t.Errorf("schema diagnostics must be returned: %+v", diags)
	}
}

func TestEmptyBundleWarns(t *testing.T) {
	results, _, err := Compile(validPlugin(), []string{"fake-empty"})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	warned := false
	for _, d := range results[0].Diagnostics {
		if strings.Contains(d.Message, "empty") {
			warned = true
		}
	}
	if !warned {
		t.Errorf("empty bundle should warn: %+v", results[0].Diagnostics)
	}
}

func TestResultHasErrors(t *testing.T) {
	results, _, err := Compile(validPlugin(), []string{"fake-err", "fake-ok"})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	byName := map[string]Result{}
	for _, r := range results {
		byName[r.Target] = r
	}
	if !byName["fake-err"].HasErrors() {
		t.Error("fake-err result should report errors")
	}
	if byName["fake-ok"].HasErrors() {
		t.Error("fake-ok result should not report errors")
	}
}
