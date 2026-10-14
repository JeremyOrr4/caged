package cli

import (
	"fmt"

	"github.com/JeremyOrr4/caged/internal/app"
	"github.com/spf13/cobra"
)

func newBuildCmd(service BuildService) *cobra.Command {
	var noCache bool
	cmd := &cobra.Command{
		Use:          "build",
		Short:        "Build the runtime image",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			built, err := service.Build(cmd.Context(), app.BuildOptions{NoCache: noCache})
			if err != nil {
				return err
			}
			for _, image := range built {
				line := fmt.Sprintf("built %s image %s", image.Role, image.Ref)
				if image.Summary != "" {
					line += " (" + image.Summary + ")"
				}
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "build without using the Docker cache")
	return cmd
}
