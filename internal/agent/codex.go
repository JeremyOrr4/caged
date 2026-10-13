package agent

// codex is OpenAI's Codex CLI. Its home is mounted as one directory so atomic
// config writes work; the masked paths hide the host's daemon sockets and
// runtime binaries from the container.
func codex() Agent {
	return Agent{
		Name:        "codex",
		Command:     []string{"codex"},
		Description: "Codex CLI",
		ConfigPaths: []ConfigPath{
			{
				HostPath:      ".codex",
				ContainerPath: ".codex",
				Writable:      true,
				EnvOverride:   "CODEX_HOME",
				MaskPaths:     []string{"app-server-control", "app-server-daemon", "ipc", "packages/app-server-daemon", "tmp/arg0"},
			},
		},
		Prompt:     PromptFlag,
		PromptFlag: "exec",
		Auth:       AuthStrategy{Environment: []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"}, CredentialPaths: []string{".codex/auth.json"}, SupportsDeviceLogin: true},
	}
}
