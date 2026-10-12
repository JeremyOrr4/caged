package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/JeremyOrr4/caged/internal/config"
	"github.com/JeremyOrr4/caged/internal/container"
	"github.com/spf13/cobra"
)

type runTarget struct {
	kind  string
	agent config.Agent
}

type runFlags struct {
	noMount         bool
	noConfig        bool
	noAuth          bool
	verbose         bool
	allowBroadMount bool
	envVars         []string
}

// NewRootCmd creates the root caged command.
func NewRootCmd(version string) *cobra.Command {
	var noMount bool
	var noConfig bool
	var noAuth bool
	var verbose bool
	var allowBroadMount bool
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
			flags := runFlags{
				noMount:         noMount,
				noConfig:        noConfig,
				noAuth:          noAuth,
				verbose:         verbose,
				allowBroadMount: allowBroadMount,
				envVars:         envVars,
			}
			return runContainer(target, flags)
		},
	}

	cmd.AddCommand(newBuildCmd())
	cmd.AddCommand(newListCmd())
	cmd.Flags().BoolVar(&noMount, "no-mount", false, "do not mount the current working directory")
	cmd.Flags().BoolVar(&allowBroadMount, "allow-broad-mount", false, "allow mounting the filesystem root or home directory")
	cmd.Flags().BoolVar(&noConfig, "no-config", false, "do not mount any host AI config into the container")
	cmd.Flags().BoolVar(&noAuth, "no-auth", false, "exclude allowlisted agent credentials from the container")
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

func agentCommand(agent config.Agent) []string {
	return append([]string(nil), agent.Command...)
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

func runContainer(target runTarget, flags runFlags) error {
	if !container.ImageExists(config.RuntimeImage) {
		fmt.Fprintf(os.Stderr, "Runtime image %s not found; building it first...\n", config.RuntimeImage)
		if err := container.BuildRuntimeImage(false); err != nil {
			return err
		}
	}

	if !flags.noAuth && !flags.noConfig {
		fmt.Fprintln(os.Stderr, "WARNING: allowlisted agent credentials are available inside the container.")
	}

	options := container.Options{
		Image:         config.RuntimeImage,
		Command:       []string{"bash"},
		ReturnToShell: target.kind == "agent",
	}
	if target.kind == "agent" {
		options.Command = agentCommand(target.agent)
	}

	if err := configureMounts(&options, flags); err != nil {
		return err
	}

	options.EnvVars = config.ResolveEnvVars(target.agent, flags.envVars, func(name string) bool {
		_, ok := os.LookupEnv(name)
		return ok
	})
	if flags.verbose {
		logEnvResolution(target.agent, flags.envVars, options.EnvVars)
	}
	return container.Run(options)
}

func configureMounts(options *container.Options, flags runFlags) error {
	if !flags.noMount {
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
		options.Mount, err = newMount(cwd, home, flags.allowBroadMount)
		if err != nil {
			return err
		}
	}

	if flags.noConfig {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}
	report := config.ResolveAllConfigsReport(home, nil, flags.noAuth)
	options.ConfigMounts = report.Mounts
	if flags.verbose {
		logConfigReport(report)
	}
	return nil
}

func logConfigReport(report config.ConfigReport) {
	for _, mount := range report.Mounts {
		fmt.Fprintf(os.Stderr, "config: pass %s -> %s\n", mount.HostPath, mount.DestPath)
	}
	for _, path := range report.Missing {
		fmt.Fprintf(os.Stderr, "config: missing %s\n", path)
	}
	for _, path := range report.Excluded {
		fmt.Fprintf(os.Stderr, "config: excluded %s\n", path)
	}
	for _, path := range report.Refused {
		fmt.Fprintf(os.Stderr, "config: refused %s\n", path)
	}
}

func logEnvResolution(agent config.Agent, requested, resolved []string) {
	passed := make(map[string]struct{}, len(resolved))
	for _, name := range resolved {
		passed[name] = struct{}{}
	}
	for _, name := range config.EnvVarNames(agent, requested) {
		if _, ok := passed[name]; ok {
			fmt.Fprintf(os.Stderr, "env: pass %s\n", name)
		} else {
			fmt.Fprintf(os.Stderr, "env: skipped %s (unset or disallowed)\n", name)
		}
	}
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
