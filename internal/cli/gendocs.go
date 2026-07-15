package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// newGenDocsCmd generates the CLI command reference as Markdown for the docs
// site, so the reference always matches the actual command tree. Hidden because
// it is a maintainer/build tool, not an end-user command.
func newGenDocsCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:    "gen-docs",
		Short:  "Generate the CLI command reference (Markdown) for the docs site",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			index := "---\ntitle: CLI reference\nweight: 50\n---\n\n" +
				"Generated from the command tree with `omniplug gen-docs`.\n"
			if err := os.WriteFile(filepath.Join(dir, "_index.md"), []byte(index), 0o644); err != nil {
				return err
			}
			prepend := func(filename string) string {
				base := strings.TrimSuffix(filepath.Base(filename), ".md")
				title := strings.ReplaceAll(base, "_", " ")
				return fmt.Sprintf("---\ntitle: %s\n---\n\n", title)
			}
			link := func(name string) string { return name }

			root := cmd.Root()
			root.DisableAutoGenTag = true
			if err := doc.GenMarkdownTreeCustom(root, dir, prepend, link); err != nil {
				return err
			}
			fmt.Printf("wrote CLI reference to %s\n", dir)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "site/content/docs/reference", "output directory for the reference pages")
	return cmd
}
