package cli

import (
	"fmt"

	"github.com/asingamaneni/omniplug/internal/compiler"
	"github.com/asingamaneni/omniplug/internal/parser"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	var src string
	var targets []string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a canonical plugin source without writing files (schema, adapter checks, and compile diagnostics)",
		RunE: func(_ *cobra.Command, _ []string) error {
			p, err := parser.Load(src)
			if err != nil {
				return err
			}
			// A dry Compile surfaces the same degradation diagnostics that
			// build/install print. Compile is pure; the bundles are discarded
			// and nothing touches disk.
			_, ds, err := compiler.Compile(p, targets)
			if err != nil {
				printDiagnostics(ds)
				return err
			}
			if len(ds) == 0 {
				fmt.Printf("ok: %s validates cleanly\n", p.Name)
				return nil
			}
			errs := printDiagnostics(ds)
			if errs > 0 {
				return fmt.Errorf("%d error(s) found", errs)
			}
			fmt.Printf("ok: %s validates with %s\n", p.Name, countNoun(len(ds), "warning"))
			return nil
		},
	}
	cmd.Flags().StringVarP(&src, "src", "s", ".", "path to the canonical plugin source")
	cmd.Flags().StringSliceVarP(&targets, "target", "t", []string{"all"}, "targets to validate against (all|claude|...)")
	return cmd
}
