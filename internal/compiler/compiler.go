// Package compiler orchestrates compilation across the adapter registry. It has
// no compile-time knowledge of any specific target.
package compiler

import (
	"fmt"

	"github.com/asingamaneni/omniplug/internal/adapter"
	"github.com/asingamaneni/omniplug/internal/model"
	"github.com/asingamaneni/omniplug/internal/schema"
)

// Result is the compiled output for a single target.
type Result struct {
	Target      string
	Bundle      adapter.Bundle
	Diagnostics []adapter.Diagnostic
}

// HasErrors reports whether this target's diagnostics include an error.
// Callers must not write a bundle whose Result has errors.
func (r Result) HasErrors() bool { return adapter.HasErrors(r.Diagnostics) }

// ResolveTargets expands "all" into every registered adapter name, or validates
// the requested names.
func ResolveTargets(requested []string) ([]string, error) {
	if len(requested) == 0 || (len(requested) == 1 && requested[0] == "all") {
		names := adapter.Names()
		if len(names) == 0 {
			return nil, fmt.Errorf("no target adapters registered")
		}
		return names, nil
	}
	for _, t := range requested {
		if _, ok := adapter.Get(t); !ok {
			return nil, fmt.Errorf("unknown target %q (registered: %v)", t, adapter.Names())
		}
	}
	return requested, nil
}

// Compile validates the plugin (schema + per-adapter) and compiles it for each
// target. Schema errors abort before compilation; the returned diagnostics
// always include schema findings.
func Compile(p *model.Plugin, targets []string) ([]Result, []adapter.Diagnostic, error) {
	schemaDiags := schema.Validate(p)
	if adapter.HasErrors(schemaDiags) {
		return nil, schemaDiags, fmt.Errorf("schema validation failed")
	}

	names, err := ResolveTargets(targets)
	if err != nil {
		return nil, schemaDiags, err
	}

	var results []Result
	allDiags := append([]adapter.Diagnostic(nil), schemaDiags...)
	// A manifest-level targets.<name> block for a target no adapter provides
	// would otherwise be silently ignored.
	for key := range p.Targets {
		if _, ok := adapter.Get(key); !ok {
			allDiags = append(allDiags, adapter.Warn("manifest", "targets",
				fmt.Sprintf("targets.%s is not a registered target; ignored", key)))
		}
	}
	for _, n := range names {
		ad, _ := adapter.Get(n)
		var diags []adapter.Diagnostic
		diags = append(diags, ad.Validate(p)...)
		bundle, compileDiags, err := ad.Compile(p)
		if err != nil {
			return nil, allDiags, fmt.Errorf("compiling %s: %w", n, err)
		}
		diags = append(diags, compileDiags...)
		if len(bundle.Files) == 0 {
			diags = append(diags, adapter.Warn(n, "plugin", "compiled bundle is empty (no components to emit)"))
		}
		// A key written by two components silently loses one; make it fatal so
		// the bad bundle is never written (gated by Result.HasErrors downstream).
		for _, path := range bundle.Collisions() {
			diags = append(diags, adapter.Error(n, "bundle",
				fmt.Sprintf("multiple components emit %q; one would be silently dropped", path)))
		}
		results = append(results, Result{Target: n, Bundle: bundle, Diagnostics: diags})
		allDiags = append(allDiags, diags...)
	}
	return results, allDiags, nil
}
