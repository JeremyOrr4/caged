package agent

// pi is the Pi coding agent.
func pi() Agent {
	return Agent{
		Name:        "pi",
		Command:     []string{"pi"},
		Description: "Pi coding agent",
		ConfigPaths: []ConfigPath{
			{HostPath: ".config/pi", ContainerPath: ".config/pi", Writable: true, Staged: true},
			{HostPath: ".pi", ContainerPath: ".pi", Writable: true, Staged: true},
		},
		Prompt: PromptPositional,
		Auth:   AuthStrategy{Environment: []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"}},
	}
}
