// Package cli wires the omniplug command-line interface (cobra).
package cli

import (
	"fmt"
	"os"

	"github.com/asingamaneni/omniplug/internal/adapter"
	"github.com/spf13/cobra"
)

// version is overridable at build time via -ldflags.
var version = "0.1.0-dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "omniplug",
		Short:         "Compile a tool-neutral AI plugin into Claude/Cursor/Codex layouts",
		Long:          "omniplug authors an AI agent plugin once in a canonical format and compiles or installs it into target-specific layouts.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		newInitCmd(),
		newValidateCmd(),
		newBuildCmd(),
		newInstallCmd(),
		newListTargetsCmd(),
		newGenDocsCmd(),
	)
	return root
}

// Execute runs the root command and exits non-zero on error.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// printDiagnostics writes diagnostics to stderr and returns the error count.
func printDiagnostics(ds []adapter.Diagnostic) int {
	errs := 0
	for _, d := range ds {
		if d.Severity == adapter.SeverityError {
			errs++
		}
		fmt.Fprintf(os.Stderr, "  %-7s [%s] %s: %s\n", d.Severity, d.Source, d.Component, d.Message)
	}
	return errs
}
