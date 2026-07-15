package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "init [name]",
		Short: "Scaffold a new canonical plugin source",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := "my-plugin"
			if len(args) == 1 {
				name = args[0]
			}
			root := filepath.Join(dir, name)
			files := scaffold(name)
			for rel, content := range files {
				dest := filepath.Join(root, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
					return err
				}
			}
			fmt.Printf("scaffolded plugin %q at %s\n", name, root)
			return nil
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "parent directory for the new plugin")
	return cmd
}

func scaffold(name string) map[string]string {
	return map[string]string{
		"plugin.yaml": fmt.Sprintf(`apiVersion: omniplug/v1
name: %s
version: 0.1.0
description: A new omniplug plugin.
author:
  name: Your Name
`, name),
		"skills/hello/SKILL.md": `---
name: hello
description: Greet the user. Use when the user says hello or asks for a greeting.
---

Greet the user warmly and ask how you can help.
`,
	}
}
