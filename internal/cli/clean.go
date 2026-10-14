package cli

import (
	"fmt"

	"github.com/JeremyOrr4/caged/internal/app"
	"github.com/spf13/cobra"
)

func newCleanCmd(service CleanService) *cobra.Command {
	var opts app.CleanOptions
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove caged's Docker images",
		Long: "Remove the runtime image, the extension image, and any older or replaced caged images, so they do not\n" +
			"outlive caged. Run it before uninstalling. An image that a container still uses is kept and\n" +
			"reported. It never touches ~/.config/caged, agent configs, containers, volumes, or the Docker build\n" +
			"cache (docker builder prune clears that).",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			results, err := service.Clean(cmd.Context(), opts)
			out := cmd.OutOrStdout()
			if err == nil && len(results) == 0 {
				_, err = fmt.Fprintln(out, "nothing to clean")
				return err
			}
			for _, result := range results {
				if _, writeErr := fmt.Fprintln(out, cleanLine(result)); writeErr != nil {
					return writeErr
				}
			}
			return err
		},
	}
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "print what would be removed, and remove nothing")
	return cmd
}

// cleanLine formats one result.
func cleanLine(result app.CleanResult) string {
	size := ""
	if result.Size != "" {
		size = " (" + result.Size + ")"
	}
	switch result.Outcome {
	case app.WouldRemove:
		return "would remove " + result.Ref + size
	case app.Kept:
		return "kept " + result.Ref + ": " + result.Reason
	default:
		return "removed " + result.Ref + size
	}
}
