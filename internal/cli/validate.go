package cli

import (
	"fmt"

	"github.com/asingamaneni/omniplug/internal/adapter"
	"github.com/asingamaneni/omniplug/internal/parser"
	"github.com/asingamaneni/omniplug/internal/schema"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	var src string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a canonical plugin source (schema + per-adapter checks)",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := parser.Load(src)
			if err != nil {
				return err
			}
			var ds []adapter.Diagnostic
			ds = append(ds, schema.Validate(p)...)
			for _, ad := range adapter.All() {
				ds = append(ds, ad.Validate(p)...)
			}
			if len(ds) == 0 {
				fmt.Printf("ok: %s validates cleanly\n", p.Name)
				return nil
			}
			errs := printDiagnostics(ds)
			if errs > 0 {
				return fmt.Errorf("%d error(s) found", errs)
			}
			fmt.Printf("ok with %d warning(s)\n", len(ds))
			return nil
		},
	}
	cmd.Flags().StringVarP(&src, "src", "s", ".", "path to the canonical plugin source")
	return cmd
}
