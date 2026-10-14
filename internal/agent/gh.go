package agent

// gh is the GitHub CLI. It takes no initial prompt.
func gh() Agent {
	return Agent{
		Name:        "gh",
		Command:     []string{"gh"},
		Description: "GitHub CLI",
		ConfigPaths: []ConfigPath{
			{HostPath: ".config/gh", ContainerPath: ".config/gh", Writable: true},
		},
		Auth: AuthStrategy{Environment: []string{"GH_TOKEN", "GITHUB_TOKEN"}},
	}
}
