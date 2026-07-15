package cli

import (
	"fmt"
	"path/filepath"

	"github.com/asingamaneni/omniplug/internal/compiler"
	"github.com/asingamaneni/omniplug/internal/installer"
	"github.com/asingamaneni/omniplug/internal/parser"
	"github.com/spf13/cobra"
)

func newBuildCmd() *cobra.Command {
	var src, out string
	var targets []string
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Compile the plugin to dist/<target>/ for one or more targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := parser.Load(src)
			if err != nil {
				return err
			}
			results, diags, err := compiler.Compile(p, targets)
			if err != nil {
				printDiagnostics(diags)
				return err
			}
			if len(diags) > 0 {
				printDiagnostics(diags)
			}
			for _, r := range results {
				root := filepath.Join(out, r.Target)
				wr, err := installer.Write(r.Bundle, root, false)
				if err != nil {
					return err
				}
				fmt.Printf("built %s -> %s (%d files)\n", r.Target, root, len(wr.Written))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&src, "src", "s", ".", "path to the canonical plugin source")
	cmd.Flags().StringVarP(&out, "out", "o", "dist", "output directory")
	cmd.Flags().StringSliceVarP(&targets, "target", "t", []string{"all"}, "targets to build (all|claude|...)")
	return cmd
}
