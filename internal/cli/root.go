package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JeremyOrr4/caged/internal/config"
	"github.com/JeremyOrr4/caged/internal/container"
	"github.com/spf13/cobra"
)

type runTarget struct {
	kind  string
	agent config.Agent
}

// NewRootCmd creates the root caged command.
func NewRootCmd(version string) *cobra.Command {
	var noMount bool
	var noConfig bool
	var withAuth bool
	var verbose bool
	var allowBroadMount bool
	var noPersist bool
	var envVars []string

	cmd := &cobra.Command{
		Use:           "caged [agent]",
		Short:         "Run AI coding agents in a disposable Docker container",
		Long:          "Run a bash shell or a supported AI coding agent in a disposable Docker container",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return config.Names(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolveTarget(args)
			if err != nil {
				return err
			}

			for _, envName := range envVars {
				if err := config.ValidateEnvName(envName); err != nil {
					return err
				}
			}

			switch target.kind {
			case "shell":
				return runContainer(config.RuntimeImage, []string{"bash"}, config.Agent{}, noMount, noConfig, withAuth, verbose, allowBroadMount, envVars, noPersist)
			case "agent":
				return runContainer(config.RuntimeImage, target.agent.Command, target.agent, noMount, noConfig, withAuth, verbose, allowBroadMount, envVars, noPersist)
			default:
				return fmt.Errorf("unsupported execution target")
			}
		},
	}

	cmd.AddCommand(newBuildCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newResetCmd())
	cmd.Flags().BoolVar(&noMount, "no-mount", false, "do not mount the current working directory")
	cmd.Flags().BoolVar(&allowBroadMount, "allow-broad-mount", false, "allow mounting the filesystem root or home directory")
	cmd.Flags().BoolVar(&noConfig, "no-config", false, "do not mount any host AI config into the container")
	cmd.Flags().BoolVar(&withAuth, "with-auth", false, "also pass credential files to the agent; this exposes host credentials to the container")
	cmd.Flags().BoolVar(&noPersist, "no-persist", false, "do not mount a persistent home volume for this agent")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "print which config files are mounted, skipped, or excluded")
	cmd.Flags().StringArrayVar(&envVars, "env", nil, "pass a host environment variable into the container by NAME only")
	cmd.Version = version
	return cmd
}

func resolveTarget(args []string) (runTarget, error) {
	if len(args) == 0 {
		return runTarget{kind: "shell"}, nil
	}

	agent, err := config.Lookup(args[0])
	if err != nil {
		return runTarget{}, err
	}
	return runTarget{kind: "agent", agent: agent}, nil
}

func newBuildCmd() *cobra.Command {
	var noCache bool
	cmd := &cobra.Command{
		Use:          "build",
		Short:        "Build the runtime image",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := container.BuildRuntimeImage(noCache); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "built runtime image", config.RuntimeImage)
			return err
		},
	}
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "build without using the Docker cache")
	return cmd
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List supported AI agents",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, name := range config.Names() {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), name); err != nil {
					return err
				}
			}
			return nil
		},
	}
	return cmd
}

func newResetCmd() *cobra.Command {
	var all bool
	var yes bool
	cmd := &cobra.Command{
		Use:          "reset [agent]",
		Short:        "Reset persisted agent state",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if all && len(args) > 0 {
				return fmt.Errorf("--all cannot be combined with an agent name")
			}
			if !all && len(args) == 0 {
				return fmt.Errorf("reset requires an agent name or --all")
			}

			if !yes {
				fmt.Fprintf(cmd.ErrOrStderr(), "This will delete persisted agent volumes. Continue? [y/N]: ")
				var response string
				if _, err := fmt.Scanln(&response); err != nil && err.Error() != "unexpected newline" {
					return nil
				}
				if !strings.EqualFold(response, "y") && !strings.EqualFold(response, "yes") {
					fmt.Fprintln(cmd.ErrOrStderr(), "aborted")
					return nil
				}
			}

			if all {
				return container.ResetVolumes(true, yes)
			}
			agent, err := config.Lookup(args[0])
			if err != nil {
				return err
			}
			return container.ResetVolumesForAgent(agent.Name, yes)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "remove every caged-* state volume")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation")
	return cmd
}

func runContainer(image string, command []string, agent config.Agent, noMount bool, noConfig bool, withAuth bool, verbose bool, allowBroadMount bool, envVars []string, noPersist bool) error {
	if !container.ImageExists(image) {
		fmt.Fprintf(os.Stderr, "Runtime image %s not found; building it first...\n", image)
		if err := container.BuildRuntimeImage(false); err != nil {
			return err
		}
	}

	if withAuth {
		fmt.Fprintln(os.Stderr, "WARNING: --with-auth includes credential files and makes them readable inside the container.")
	}

	options := container.Options{
		Image:   image,
		Command: command,
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

	localAgentConfig := false
	if agent.Name != "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("find home directory: %w", err)
		}
		localAgentConfig = hasLocalAgentConfig(agent, home)
		if !noConfig {
			resolvedHome, err := filepath.EvalSymlinks(home)
			if err != nil {
				return fmt.Errorf("resolve home directory: %w", err)
			}
			for _, configPath := range agent.ConfigPaths {
				hostPath := filepath.Join(home, filepath.Clean(configPath.HostPath))
				if !withAuth && configPath.IsAuth {
					if verbose {
						fmt.Fprintf(os.Stderr, "config: excluded auth path %s\n", hostPath)
					}
					continue
				}
				if _, statErr := os.Stat(hostPath); statErr != nil {
					if verbose {
						fmt.Fprintf(os.Stderr, "config: missing %s\n", hostPath)
					}
					continue
				}
				resolvedHost, err := filepath.EvalSymlinks(hostPath)
				if err != nil {
					if verbose {
						fmt.Fprintf(os.Stderr, "config: refused %s (unresolvable symlink)\n", hostPath)
					}
					continue
				}
				if !config.IsWithinHome(resolvedHost, resolvedHome) {
					if verbose {
						fmt.Fprintf(os.Stderr, "config: refused %s (resolves outside home)\n", hostPath)
					}
					continue
				}
				if verbose {
					fmt.Fprintf(os.Stderr, "config: pass %s -> %s\n", hostPath, filepath.Join("/home/caged", filepath.Clean(configPath.ContainerPath)))
				}
			}
			options.ConfigMounts = config.ResolveConfigs(agent, home, func(path string) bool {
				_, err := os.Stat(path)
				return err == nil
			}, withAuth)
		}
	}

	if agent.Name != "" && localAgentConfig && !noMount && !noPersist {
		for _, statePath := range agent.StatePaths {
			options.VolumeMounts = append(options.VolumeMounts, container.VolumeMount{
				Name:          config.StateVolumeName(agent.Name),
				ContainerPath: filepath.ToSlash(filepath.Join("/home/caged", filepath.Clean(statePath))),
			})
		}
	}

	resolvedEnv := config.ResolveEnvVars(agent, envVars, func(name string) bool {
		_, ok := os.LookupEnv(name)
		return ok
	})
	if len(resolvedEnv) > 0 {
		if verbose {
			for _, envName := range resolvedEnv {
				fmt.Fprintf(os.Stderr, "env: pass %s\n", envName)
			}
			for _, envName := range envVars {
				if !contains(resolvedEnv, envName) && strings.TrimSpace(envName) != "" {
					fmt.Fprintf(os.Stderr, "env: skipped %s (unset or disallowed)\n", envName)
				}
			}
		}
		options.EnvVars = resolvedEnv
	} else if len(envVars) > 0 && verbose {
		for _, envName := range envVars {
			if strings.TrimSpace(envName) != "" {
				fmt.Fprintf(os.Stderr, "env: skipped %s (unset or disallowed)\n", envName)
			}
		}
	}

	return container.Run(options)
}

func hasLocalAgentConfig(agent config.Agent, home string) bool {
	mounts := config.ResolveConfigs(agent, home, func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}, true)
	for _, mount := range mounts {
		if mount.Name != ".gitconfig" {
			return true
		}
	}
	return false
}

func contains(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
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

func formatAgentList(names []string) string {
	return strings.Join(names, ", ")
}
