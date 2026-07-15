package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/asingamaneni/omniplug/internal/adapter"
	"github.com/spf13/cobra"
)

func newListTargetsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-targets",
		Short: "List registered target adapters and their capability matrix",
		RunE: func(_ *cobra.Command, _ []string) error {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "TARGET\tSKILLS\tMCP\tCOMMANDS\tAGENTS\tHOOKS\tGUIDANCE")
			for _, ad := range adapter.All() {
				c := ad.Capabilities()
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					ad.Name(), yn(c.Skills), yn(c.MCP), string(c.Commands),
					yn(c.Agents), yn(c.Hooks), yn(c.Guidance))
			}
			return w.Flush()
		},
	}
}

func yn(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
