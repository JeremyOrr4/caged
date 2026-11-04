package app

import "github.com/JeremyOrr4/caged/internal/agent"

// Request is a parsed command line: what to run and how to set it up.
type Request struct {
	Agent                                       *agent.Agent
	Prompt                                      *string
	NoMount, NoConfig, Verbose, AllowBroadMount bool
	TrustConfig                                 bool // approve a changed config.yaml without asking
	WorkDir                                     string
	EnvVars                                     []string
	Ports                                       []string // -p values, parsed by the planner
	AgentArgs                                   []string // arguments after --, passed to the agent unchanged
}
