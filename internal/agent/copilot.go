package agent

// copilot is the GitHub Copilot CLI.
func copilot() Agent {
	return Agent{
		Name:        "copilot",
		Command:     []string{"copilot"},
		Description: "GitHub Copilot CLI",
		ConfigPaths: []ConfigPath{
			{HostPath: ".copilot", ContainerPath: ".copilot", Writable: true, EnvOverride: "COPILOT_HOME"},
		},
		Prompt:     PromptFlag,
		PromptFlag: "-p",
		Auth:       AuthStrategy{Environment: []string{"COPILOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"}, CredentialPaths: []string{".copilot/"}, SupportsDeviceLogin: true},
	}
}
