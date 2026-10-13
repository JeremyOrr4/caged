package cli

import (
	"context"
	"fmt"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/app"
	"github.com/JeremyOrr4/caged/internal/runtime"
	"github.com/spf13/cobra"
)

// Service runs and builds caged containers. app.Service implements it.
type Service interface {
	Run(ctx context.Context, request app.Request) error
	Build(ctx context.Context, noCache bool) error
}

// Agents finds supported agents. agent.Registry implements it.
type Agents interface {
	Lookup(name string) (agent.Agent, error)
	Names() []string
}

// NewRootCmd creates the root caged command. It only turns arguments and
// flags into an app.Request; agents and service do the work.
func NewRootCmd(version string, agents Agents, service Service) *cobra.Command {
	var flags app.Request

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
			return agents.Names(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			request, err := buildRequest(agents, flags, args)
			if err != nil {
				return err
			}
			return service.Run(cmd.Context(), request)
		},
	}

	cmd.AddCommand(newBuildCmd(service))
	cmd.AddCommand(newListCmd(agents))
	cmd.Flags().BoolVar(&flags.NoMount, "no-mount", false, "do not mount the current working directory")
	cmd.Flags().StringVarP(&flags.WorkDir, "workdir", "w", "", "mount this host directory at /workspace/<folder-name>")
	cmd.Flags().BoolVar(&flags.AllowBroadMount, "allow-broad-mount", false, "allow mounting the filesystem root or home directory")
	cmd.Flags().BoolVar(&flags.NoConfig, "no-config", false, "do not mount any host AI config into the container")
	cmd.Flags().BoolVar(&flags.Verbose, "verbose", false, "print which config files and environment names are passed")
	cmd.Flags().StringArrayVar(&flags.EnvVars, "env", nil, "pass a host environment variable into the container by NAME only")
	cmd.Version = version
	return cmd
}

// buildRequest adds the positional arguments to the parsed flags. No
// argument means a shell, the first names an agent, and the second is its
// prompt. Environment names are validated here, before anything runs.
func buildRequest(agents Agents, flags app.Request, args []string) (app.Request, error) {
	request := flags
	if len(args) > 0 {
		selected, err := agents.Lookup(args[0])
		if err != nil {
			return app.Request{}, err
		}
		request.Agent = &selected
	}
	for _, name := range request.EnvVars {
		if err := agent.ValidateEnvName(name); err != nil {
			return app.Request{}, err
		}
	}
	if len(args) == 2 {
		prompt := args[1]
		request.Prompt = &prompt
	}
	return request, nil
}

func newBuildCmd(service Service) *cobra.Command {
	var noCache bool
	cmd := &cobra.Command{
		Use:          "build",
		Short:        "Build the runtime image",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := service.Build(cmd.Context(), noCache); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "built runtime image", runtime.Image)
			return err
		},
	}
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "build without using the Docker cache")
	return cmd
}

func newListCmd(agents Agents) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List supported AI agents",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, name := range agents.Names() {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), name); err != nil {
					return err
				}
			}
			return nil
		},
	}
	return cmd
}
