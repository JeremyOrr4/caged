package cli

import (
	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/app"
	"github.com/spf13/cobra"
)

// NewRootCmd creates the root caged command. It only turns arguments and
// flags into app requests; agents and services do the work.
func NewRootCmd(version string, agents *agent.Registry, services Services) *cobra.Command {
	var flags app.Request

	cmd := &cobra.Command{
		Use:           "caged [agent [prompt]] [-- agent-args...]",
		Short:         "Run AI coding agents in a disposable Docker container",
		Long:          "Run a bash shell or a supported AI coding agent in a disposable Docker container",
		Args:          positionalArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
			return agentSuggestions(agents, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			positional, agentArgs := splitArgs(cmd, args)
			request, err := buildRequest(agents, flags, positional, agentArgs)
			if err != nil {
				return err
			}
			return services.Run.Run(cmd.Context(), request)
		},
	}

	cmd.AddCommand(newBuildCmd(services.Build))
	cmd.AddCommand(newDoctorCmd(services.Doctor))
	cmd.AddCommand(newListCmd(services.List))
	cmd.AddCommand(newCleanCmd(services.Clean))
	cmd.Flags().BoolVar(&flags.NoMount, "no-mount", false, "do not mount the current working directory")
	cmd.Flags().StringVarP(&flags.WorkDir, "workdir", "w", "", "mount this host directory at /workspace/<folder-name>")
	cmd.Flags().BoolVar(&flags.AllowBroadMount, "allow-broad-mount", false, "allow mounting the filesystem root or home directory")
	cmd.Flags().BoolVar(&flags.NoConfig, "no-config", false, "do not mount any host AI config into the container")
	cmd.Flags().BoolVar(&flags.TrustConfig, "trust-config", false, "approve a changed config.yaml without asking (for scripts)")
	cmd.Flags().BoolVar(&flags.Verbose, "verbose", false, "print which config files and environment names are passed")
	cmd.Flags().StringArrayVarP(&flags.Ports, "publish", "p", nil, "publish a container port on 127.0.0.1: PORT or HOST_PORT:CONTAINER_PORT (repeatable)")
	cmd.Flags().StringArrayVar(&flags.EnvVars, "env", nil, "pass a host environment variable into the container by NAME only")
	cmd.SetFlagErrorFunc(flagErrorWithHint)
	cmd.Version = version
	return cmd
}

// agentSuggestions completes the first argument with the agent names and
// their descriptions. While completing, cobra reports the end of args as the
// position of "--", so ArgsLenAtDash cannot tell "after --" apart from "after
// the agent". Both mean the agent is named, so nothing more is suggested.
func agentSuggestions(agents *agent.Registry, args []string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names := agents.Names()
	suggestions := make([]string, 0, len(names))
	for _, name := range names {
		suggestion := name
		if definition, err := agents.Lookup(name); err == nil && definition.Description != "" {
			suggestion += "\t" + definition.Description
		}
		suggestions = append(suggestions, suggestion)
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// buildRequest adds the positional arguments and the arguments after "--" to
// the parsed flags. No positional argument means a shell, the first names an
// agent, and the second is its prompt. Environment names are validated here,
// before anything runs.
func buildRequest(agents *agent.Registry, flags app.Request, args, agentArgs []string) (app.Request, error) {
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
	request.AgentArgs = agentArgs
	return request, nil
}
