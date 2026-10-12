package cli

import (
	"fmt"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/app"
	"github.com/JeremyOrr4/caged/internal/runtime"
	"github.com/spf13/cobra"
)

type runTarget struct {
	kind  string
	agent agent.Agent
}

// NewRootCmd creates the root caged command.
func NewRootCmd(version string) *cobra.Command {
	var noMount bool
	var workDir string
	var noConfig bool
	var verbose bool
	var allowBroadMount bool
	var envVars []string

	cmd := &cobra.Command{
		Use:           "caged [agent [prompt]]",
		Short:         "Run AI coding agents in a disposable Docker container",
		Long:          "Run a bash shell or a supported AI coding agent in a disposable Docker container",
		Args:          cobra.MaximumNArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return agent.Names(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolveTarget(args)
			if err != nil {
				return err
			}

			for _, envName := range envVars {
				if err := agent.ValidateEnvName(envName); err != nil {
					return err
				}
			}
			request := app.Request{NoMount: noMount, WorkDir: workDir, NoConfig: noConfig, Verbose: verbose, AllowBroadMount: allowBroadMount, EnvVars: envVars}
			if len(args) == 2 {
				prompt := args[1]
				request.Prompt = &prompt
			}
			if target.kind == "agent" {
				request.Agent = &target.agent
			}
			return app.DefaultService().Run(request)
		},
	}

	cmd.AddCommand(newBuildCmd())
	cmd.AddCommand(newListCmd())
	cmd.Flags().BoolVar(&noMount, "no-mount", false, "do not mount the current working directory")
	cmd.Flags().StringVarP(&workDir, "workdir", "w", "", "mount this host directory at /workspace/<folder-name>")
	cmd.Flags().BoolVar(&allowBroadMount, "allow-broad-mount", false, "allow mounting the filesystem root or home directory")
	cmd.Flags().BoolVar(&noConfig, "no-config", false, "do not mount any host AI config into the container")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "print which config files and environment names are passed")
	cmd.Flags().StringArrayVar(&envVars, "env", nil, "pass a host environment variable into the container by NAME only")
	cmd.Version = version
	return cmd
}

func resolveTarget(args []string) (runTarget, error) {
	if len(args) == 0 {
		return runTarget{kind: "shell"}, nil
	}

	agent, err := agent.Lookup(args[0])
	if err != nil {
		return runTarget{}, err
	}
	return runTarget{kind: "agent", agent: agent}, nil
}

func agentCommand(agent agent.Agent) []string {
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
			if err := app.DefaultService().Build(noCache); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "built runtime image", runtime.Image)
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
			for _, name := range agent.Names() {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), name); err != nil {
					return err
				}
			}
			return nil
		},
	}
	return cmd
}
