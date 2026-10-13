package agent

// gemini is Google's Gemini CLI.
func gemini() Agent {
	return Agent{
		Name:        "gemini",
		Command:     []string{"gemini"},
		Description: "Gemini CLI",
		ConfigPaths: []ConfigPath{
			{HostPath: ".gemini", ContainerPath: ".gemini", Writable: true},
		},
		Prompt:     PromptFlag,
		PromptFlag: "-i",
		Auth:       AuthStrategy{Environment: []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"}, CredentialPaths: []string{".gemini/oauth_creds.json"}},
	}
}
