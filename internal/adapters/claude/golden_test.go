package claude

import (
	"flag"
	"testing"

	"github.com/asingamaneni/omniplug/internal/adapters/goldentest"
	"github.com/asingamaneni/omniplug/internal/parser"
)

var update = flag.Bool("update", false, "rewrite golden files")

// TestGoldenHelloPlugin pins every byte of the compiled example bundle.
// Regenerate with: go test ./internal/adapters/claude -run Golden -update
func TestGoldenHelloPlugin(t *testing.T) {
	p, err := parser.Load("../../../examples/hello-plugin")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	b, _, err := (&Adapter{}).Compile(p)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	goldentest.Compare(t, b, "testdata/golden/hello-plugin", *update)
}
