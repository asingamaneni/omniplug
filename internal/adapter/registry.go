package adapter

import (
	"fmt"
	"sort"
)

// registry holds all self-registered adapters, keyed by Name().
var registry = map[string]Adapter{}

// Register adds an adapter to the registry. Call from an adapter package's
// init(). Panics on duplicate names to catch programming errors early.
func Register(a Adapter) {
	name := a.Name()
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("adapter %q already registered", name))
	}
	registry[name] = a
}

// Get returns the adapter with the given name.
func Get(name string) (Adapter, bool) {
	a, ok := registry[name]
	return a, ok
}

// All returns every registered adapter, sorted by name for deterministic output.
func All() []Adapter {
	out := make([]Adapter, 0, len(registry))
	for _, n := range Names() {
		out = append(out, registry[n])
	}
	return out
}

// Names returns the sorted list of registered adapter names.
func Names() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
