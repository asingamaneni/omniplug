package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/asingamaneni/omniplug/internal/adapter"
	"github.com/asingamaneni/omniplug/internal/compiler"
	"github.com/asingamaneni/omniplug/internal/installer"
	"github.com/asingamaneni/omniplug/internal/parser"
	"github.com/spf13/cobra"
)

func newInstallCmd() *cobra.Command {
	var src, scope, projectDir string
	var targets []string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Compile and install the plugin into a target's real config location",
		RunE: func(_ *cobra.Command, _ []string) error {
			sc := adapter.Scope(scope)
			if sc != adapter.ScopeProject && sc != adapter.ScopeUser {
				return fmt.Errorf("invalid --scope %q (want project|user)", scope)
			}
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
				ad, _ := adapter.Get(r.Target)
				plan, err := ad.InstallPlan(p, sc, projectDir)
				if err != nil {
					return err
				}
				// Cursor bakes project-root-relative script paths into
				// hooks.json/mcp.json at compile time, which don't resolve for a
				// user-scoped install rooted at $HOME. Warn rather than fail.
				if sc == adapter.ScopeUser && r.Target == "cursor" && len(p.HookFiles) > 0 {
					fmt.Fprintln(os.Stderr, "  warning [cursor] hooks: bundled script paths are project-relative and may not resolve for a user-scope install; prefer --scope project")
				}
				wr, err := installer.Write(r.Bundle, plan.Root, dryRun)
				if err != nil {
					return err
				}
				verb := "installed"
				if dryRun {
					verb = "would install"
				}
				fmt.Printf("%s %s -> %s [%s] (%s)\n", verb, r.Target, plan.Root, plan.Description, countNoun(len(wr.Written), "file"))
			}
			if len(failed) > 0 {
				return fmt.Errorf("target(s) %s failed with errors; nothing written for them", strings.Join(failed, ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&src, "src", "s", ".", "path to the canonical plugin source")
	cmd.Flags().StringVar(&scope, "scope", "project", "install scope (project|user)")
	cmd.Flags().StringVar(&projectDir, "project-dir", ".", "project directory for project-scoped installs")
	cmd.Flags().StringSliceVarP(&targets, "target", "t", []string{"all"}, "targets to install (all|claude|...)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report actions without writing files")
	return cmd
}
