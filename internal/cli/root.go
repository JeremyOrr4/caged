package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/JeremyOrr4/caged/internal/config"
	"github.com/JeremyOrr4/caged/internal/container"
	"github.com/spf13/cobra"
)

// NewRootCmd creates the root caged command.
func NewRootCmd(version string) *cobra.Command {
	var noMount bool
	var allowBroadMount bool

	cmd := &cobra.Command{
		Use:           "caged",
		Short:         "Open a disposable Docker container shell",
		Long:          "Open a disposable Docker container shell",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			options := container.Options{
				Image:   config.DefaultImage,
				Command: []string{"bash"},
			}

			if !noMount {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("get current working directory: %w", err)
				}
				cwd, err = filepath.Abs(cwd)
				if err != nil {
					return fmt.Errorf("resolve current working directory: %w", err)
				}
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("find home directory: %w", err)
				}

				mount, err := newMount(cwd, home, allowBroadMount)
				if err != nil {
					return err
				}
				options.Mount = mount
			}

			return container.Run(options)
		},
	}

	cmd.Flags().BoolVar(&noMount, "no-mount", false, "do not mount the current working directory")
	cmd.Flags().BoolVar(&allowBroadMount, "allow-broad-mount", false, "allow mounting the filesystem root or home directory")
	cmd.Version = version
	return cmd
}

func newMount(cwd, home string, allowBroadMount bool) (*container.Mount, error) {
	cwd = filepath.Clean(cwd)
	home = filepath.Clean(home)

	if !allowBroadMount && cwd == string(filepath.Separator) {
		return nil, fmt.Errorf("refusing to mount the filesystem root; use --allow-broad-mount to override")
	}
	if !allowBroadMount && cwd == home {
		return nil, fmt.Errorf("refusing to mount your entire home directory; use --allow-broad-mount to override")
	}

	return &container.Mount{HostPath: cwd, ContainerPath: "/workspace"}, nil
}
