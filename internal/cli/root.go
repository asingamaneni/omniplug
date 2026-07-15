// Package cli wires the omniplug command-line interface (cobra).
package cli

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/asingamaneni/omniplug/internal/adapter"
	"github.com/spf13/cobra"
)

// devVersion is the version reported by unlinked dev builds.
const devVersion = "0.1.0-dev"

// version is overridable at build time via -ldflags.
var version = devVersion

// resolveVersion prefers the ldflags-injected version, then the module version
// recorded by `go install module@version`, then the dev default.
func resolveVersion() string {
	if version != devVersion {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return version
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "omniplug",
		Short:         "Compile a tool-neutral AI plugin into Claude/Cursor/Codex layouts",
		Long:          "omniplug authors an AI agent plugin once in a canonical format and compiles or installs it into target-specific layouts.",
		Version:       resolveVersion(),
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

// countNoun formats a count with a naively pluralized noun ("1 file", "3 files").
func countNoun(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
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
