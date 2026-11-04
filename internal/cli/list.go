package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/JeremyOrr4/caged/internal/app"
	"github.com/spf13/cobra"
)

func newListCmd(service ListService) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List supported AI agents",
		Long: "List the supported AI agents, the host config that a run would mount for each, and which of\n" +
			"its credential environment variables are set. Only names are shown, never values.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			statuses, err := service.List()
			if err != nil {
				return err
			}
			return writeAgentTable(cmd.OutOrStdout(), statuses)
		},
	}
}

// writeAgentTable prints the agents as aligned columns. An agent with no
// config shows "not found", and an empty list of variables shows "-". The
// paths and names go through app.PrintableLine, since a config path can hold
// a tab or a newline; agent names come from the registry.
func writeAgentTable(out io.Writer, statuses []app.AgentStatus) error {
	table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "AGENT\tCONFIG\tAUTH ENV SET"); err != nil {
		return err
	}
	for _, status := range statuses {
		config, env := "not found", "-"
		if len(status.Config) > 0 {
			config = strings.Join(status.Config, ", ")
		}
		if len(status.EnvSet) > 0 {
			env = strings.Join(status.EnvSet, ", ")
		}
		if _, err := fmt.Fprintf(table, "%s\t%s\t%s\n", status.Name, app.PrintableLine(config), app.PrintableLine(env)); err != nil {
			return err
		}
	}
	return table.Flush()
}
