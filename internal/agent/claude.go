package agent

// claude is Anthropic's Claude Code CLI.
func claude() Agent {
	return Agent{
		Name:        "claude",
		Command:     []string{"claude"},
		Description: "Claude Code",
		ConfigPaths: []ConfigPath{
			{HostPath: ".claude", ContainerPath: ".claude", Writable: true, EnvOverride: "CLAUDE_CONFIG_DIR"},
		},
		Prompt: PromptPositional,
		Auth:   AuthStrategy{Environment: []string{"ANTHROPIC_API_KEY"}, CredentialPaths: []string{".claude/.credentials.json"}},
	}
}
