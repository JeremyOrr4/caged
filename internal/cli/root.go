package cli

import (
	"github.com/JeremyOrr4/caged/internal/config"
	"github.com/JeremyOrr4/caged/internal/container"
	"github.com/spf13/cobra"
)

// NewRootCmd creates the root caged command.
func NewRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "caged",
		Short:         "Open a disposable Docker container shell",
		Long:          "Open a disposable Docker container shell",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return container.Run(config.DefaultImage, []string{"bash"})
		},
	}

	cmd.Version = version
	return cmd
}
