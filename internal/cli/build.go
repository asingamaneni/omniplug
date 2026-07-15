package cli

import (
	"fmt"
	"path/filepath"
	"strings"

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
			var failed []string
			for _, r := range results {
				if r.HasErrors() {
					failed = append(failed, r.Target)
					continue
				}
				root := filepath.Join(out, r.Target)
				wr, err := installer.Write(r.Bundle, root, false)
				if err != nil {
					return err
				}
				fmt.Printf("built %s -> %s (%s)\n", r.Target, root, countNoun(len(wr.Written), "file"))
			}
			if len(failed) > 0 {
				return fmt.Errorf("target(s) %s failed with errors; nothing written for them", strings.Join(failed, ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&src, "src", "s", ".", "path to the canonical plugin source")
	cmd.Flags().StringVarP(&out, "out", "o", "dist", "output directory")
	cmd.Flags().StringSliceVarP(&targets, "target", "t", []string{"all"}, "targets to build (all|claude|...)")
	return cmd
}
