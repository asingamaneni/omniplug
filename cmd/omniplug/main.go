// Command omniplug compiles a tool-neutral AI plugin source into target-specific
// layouts (Claude Code today; Cursor/Codex/others via additional adapters).
package main

import (
	"github.com/asingamaneni/omniplug/internal/cli"

	// Register target adapters via blank imports. Adding a new target is a
	// matter of implementing the adapter.Adapter interface and adding its
	// blank import here — no changes to the compiler or CLI.
	_ "github.com/asingamaneni/omniplug/internal/adapters/claude"
	_ "github.com/asingamaneni/omniplug/internal/adapters/cursor"
)

func main() {
	cli.Execute()
}
