package main

import "github.com/JeremyOrr4/caged/internal/images"

// goldenPrompt contains shell syntax to prove prompts are passed verbatim.
const goldenPrompt = `hello $(world) "quoted" 'single'`

type goldenCase struct {
	name string
	args []string
	// env is applied after the defaults; "$ROOT" is replaced with the scenario root.
	env map[string]string
	// noDocker hides the fake docker binary from PATH.
	noDocker bool
	// files are extra fixture files under $ROOT, by relative path. "$ROOT" in
	// the contents is replaced with the scenario root.
	files map[string]string
	// links are extra symlinks under $ROOT (link -> target); an existing entry at the link is removed first. "$ROOT" in a target is replaced.
	links map[string]string
	// images are the local images; nil means defaultImages.
	images map[string]fakeImage
	// fail makes docker verbs fail, by verb name (see the fake docker header).
	fail map[string]fakeFailure
	// ls are the lines `docker image ls` prints, one JSON object each.
	ls []string
}

// goldenCases are the command lines of TestGoldenDockerInvocations, grouped by
// the protected behaviour (protected-logic.md) they pin. Each name is the
// golden file's name; the order does not matter.
func goldenCases() []goldenCase {
	var cases []goldenCase
	for _, group := range [][]goldenCase{
		runCases(), mountCases(), configCases(), linkCases(), envCases(),
		portCases(), imageCases(), commandCases(), agentCases(),
	} {
		cases = append(cases, group...)
	}
	return cases
}

// runCases: the shell, agent names, passthrough arguments, and how docker's
// exit codes and failures reach the user.
func runCases() []goldenCase {
	return []goldenCase{
		{name: "shell", args: []string{}},
		{name: "agent-uppercase", args: []string{"Claude"}},
		{name: "unsupported-agent", args: []string{"aider"}},
		{name: "too-many-args", args: []string{"claude", "one", "two"}},
		{name: "passthrough-prompt-claude", args: []string{"claude", goldenPrompt, "--", "--model", "opus", "--add-dir", "../lib", "$(id)", `"q"`}},
		{name: "passthrough-gh", args: []string{"gh", "--", "pr", "list"}},
		{name: "passthrough-no-agent", args: []string{"--", "--resume"}},
		{name: "passthrough-dash-build", args: []string{"--", "build"}},
		{name: "run-exit-code", args: []string{"claude"}, fail: map[string]fakeFailure{"run": {Code: 7}}},
		{
			name: "run-mounts-denied",
			args: []string{"codex"},
			fail: map[string]fakeFailure{"run": {Code: 125, Stderr: "docker: Error response from daemon: Mounts denied: the path is not shared from the host."}},
		},
		{name: "daemon-down", args: []string{"claude"}, fail: map[string]fakeFailure{"info": daemonDown}},
		{name: "docker-missing", args: []string{"claude"}, noDocker: true},
	}
}

// mountCases: the project mount, -w, --no-mount, the broad-mount refusal and
// workdirs.allow.
func mountCases() []goldenCase {
	return []goldenCase{
		{name: "shell-no-mount-no-config", args: []string{"--no-mount", "--no-config"}},
		{name: "no-mount-with-workdir", args: []string{"--no-mount", "-w", "$ROOT/work/other"}},
		{name: "workdir-flag", args: []string{"-w", "$ROOT/work/other", "pi"}},
		{name: "workdir-relative", args: []string{"--workdir", "../other", "--no-config"}},
		{name: "workdir-missing", args: []string{"-w", "$ROOT/work/missing"}},
		{name: "workdir-is-home", args: []string{"-w", "$ROOT/home"}},
		{name: "workdir-home-allowed", args: []string{"--allow-broad-mount", "-w", "$ROOT/home", "--no-config"}},
		{name: "workdir-root-allowed", args: []string{"--allow-broad-mount", "-w", "/", "--no-config"}},
		{name: "workdir-root", args: []string{"-w", "/"}},
		{name: "workdir-not-allowed", args: []string{"claude"}, files: map[string]string{goldenExtensionFile: "workdirs:\n  allow:\n    - path: ~/elsewhere\n"}},
		{name: "workdir-allowed", args: []string{"claude"}, files: map[string]string{goldenExtensionFile: "workdirs:\n  allow:\n    - path: $ROOT/work\n"}},
	}
}

// configCases: agent config overrides, an unresolvable home, and config.yaml
// errors and gotchas.
func configCases() []goldenCase {
	return []goldenCase{
		{
			name: "config-overrides",
			args: []string{"--verbose", "--no-mount"},
			env: map[string]string{
				"CODEX_HOME":        "alt-codex",
				"CLAUDE_CONFIG_DIR": "$ROOT/home",
				"COPILOT_HOME":      "$ROOT/outside/copilot",
			},
		},
		{name: "home-unresolvable", args: []string{"--verbose", "--no-mount"}, env: map[string]string{"HOME": "$ROOT/missing-home"}},
		{name: "config-invalid-yaml", args: []string{"--no-mount"}, files: map[string]string{goldenExtensionFile: "image: [\n"}},
		{name: "config-unknown-field", args: []string{"--no-mount"}, files: map[string]string{goldenExtensionFile: "imgae:\n  apt: [jq]\n"}},
		// "~" is quoted: an unquoted ~ is YAML null, which reads as an empty path.
		{name: "config-allow-home", args: []string{"--verbose", "--no-mount", "claude"}, files: map[string]string{goldenExtensionFile: "links:\n  allow:\n    - path: \"~\"\n"}},
		{name: "config-unquoted-tilde", args: []string{"--no-mount"}, files: map[string]string{goldenExtensionFile: "links:\n  allow:\n    - path: ~\n"}},
	}
}

// linkCases: Stow-style symlinks in agent config, refused with a hint,
// allowed, read-only, denied, as the config root, and under Codex masks.
func linkCases() []goldenCase {
	return []goldenCase{
		{
			name:  "links-refused-hint",
			args:  []string{"--verbose", "claude"},
			files: map[string]string{"home/dotfiles/claude/CLAUDE.md": "# rules\n"},
			links: map[string]string{"home/.claude/CLAUDE.md": "../dotfiles/claude/CLAUDE.md"},
		},
		{
			name: "links-allowed",
			args: []string{"--verbose", "claude"},
			files: map[string]string{
				"home/dotfiles/claude/CLAUDE.md": "# rules\n",
				goldenExtensionFile:              "links:\n  allow:\n    - path: ~/dotfiles\n",
			},
			links: map[string]string{"home/.claude/CLAUDE.md": "../dotfiles/claude/CLAUDE.md"},
		},
		{
			name: "links-read-only",
			args: []string{"--verbose", "claude"},
			files: map[string]string{
				"home/dotfiles/claude/CLAUDE.md": "# rules\n",
				goldenExtensionFile:              "links:\n  allow:\n    - path: ~/dotfiles\n      read_only: true\n",
			},
			links: map[string]string{"home/.claude/CLAUDE.md": "../dotfiles/claude/CLAUDE.md"},
		},
		{
			name: "links-denied-ssh",
			args: []string{"--verbose", "claude"},
			files: map[string]string{
				"home/.ssh/id_test": "key\n",
				goldenExtensionFile: "links:\n  allow:\n    - path: ~/dotfiles\n",
			},
			links: map[string]string{"home/.claude/key": "../.ssh/id_test"},
		},
		{
			name: "links-root-allowed",
			args: []string{"--verbose", "pi"},
			files: map[string]string{
				"home/dotfiles/pi/settings.json": "{}\n",
				goldenExtensionFile:              "links:\n  allow:\n    - path: ~/dotfiles\n",
			},
			links: map[string]string{"home/.config/pi": "../dotfiles/pi"},
		},
		{
			name: "links-codex-masked",
			args: []string{"--verbose", "codex"},
			files: map[string]string{
				"home/dotfiles/codex/AGENTS.md": "# a\n",
				goldenExtensionFile:             "links:\n  allow:\n    - path: ~/dotfiles\n",
			},
			links: map[string]string{"home/.codex/AGENTS.md": "../dotfiles/codex/AGENTS.md"},
		},
		{
			name:  "doctor-links-refused",
			args:  []string{"doctor"},
			files: map[string]string{"home/dotfiles/claude/CLAUDE.md": "# rules\n"},
			links: map[string]string{"home/.claude/CLAUDE.md": "../dotfiles/claude/CLAUDE.md"},
		},
	}
}

// envCases: --env forwards names only, values are never printed, and the
// credentials an agent reads.
func envCases() []goldenCase {
	return []goldenCase{
		{name: "env-invalid", args: []string{"--env", "bad-name", "claude"}},
		{name: "env-padded-name", args: []string{"--env", " CAGED_GOLDEN_SET ", "--verbose", "gemini"}, env: map[string]string{"CAGED_GOLDEN_SET": "x"}},
		{
			name: "shell-env",
			args: []string{"--env", "CAGED_GOLDEN_SET", "--env", "CAGED_GOLDEN_UNSET", "--verbose", "--no-config"},
			env:  map[string]string{"CAGED_GOLDEN_SET": "x", "ANTHROPIC_API_KEY": "not-forwarded-to-a-plain-shell"},
		},
		{
			name: "env-extra-names",
			args: []string{"--env", "CAGED_GOLDEN_SET", "--env", "CAGED_GOLDEN_UNSET", "--env", "ANTHROPIC_API_KEY", "--verbose", "codex"},
			env:  map[string]string{"CAGED_GOLDEN_SET": "value-is-never-printed", "OPENAI_API_KEY": "secret"},
		},
		{name: "verbose-claude", args: []string{"--verbose", "claude"}, env: map[string]string{"ANTHROPIC_API_KEY": "secret"}},
		{name: "list-env-set", args: []string{"list"}, env: map[string]string{"ANTHROPIC_API_KEY": "secret-value-never-printed", "GH_TOKEN": "secret-value-never-printed"}},
	}
}

// portCases: -p parsing before Docker, the loopback binding, a port in use.
func portCases() []goldenCase {
	return []goldenCase{
		{name: "ports-publish", args: []string{"-p", "3000", "-p", "8080:3000", "--verbose", "claude"}, env: map[string]string{"ANTHROPIC_API_KEY": "secret"}},
		{name: "ports-invalid", args: []string{"-p", "70000", "claude"}},
		{
			name: "ports-in-use",
			args: []string{"-p", "8080:3000", "claude"},
			fail: map[string]fakeFailure{"run": {Code: 125, Stderr: "docker: Error response from daemon: failed to set up container networking: driver failed programming external connectivity on endpoint x: failed to bind host port 127.0.0.1:8080/tcp: address already in use"}},
		},
	}
}

// imageCases: the runtime image and the image.apt extension image, built when
// missing or stale, and caged build.
func imageCases() []goldenCase {
	return []goldenCase{
		{name: "image-stale-rebuilds", args: []string{"claude"}, images: map[string]fakeImage{images.BaseRef: staleBase()}},
		{name: "image-unlabeled-rebuilds", args: []string{"gemini"}, images: map[string]fakeImage{images.BaseRef: {ID: "sha256:fake-base"}}},
		{name: "image-current-verbose", args: []string{"--verbose", "--no-mount", "--no-config"}},
		{
			name:   "image-rebuild-fails",
			args:   []string{"claude"},
			images: map[string]fakeImage{images.BaseRef: staleBase()},
			fail:   map[string]fakeFailure{"build": {Code: 1, Stderr: "ERROR: failed to solve: process \"/bin/sh -c false\" did not complete successfully"}},
		},
		{name: "image-missing-builds-first", args: []string{"gemini"}, images: map[string]fakeImage{}},
		{
			name:  "extension-first-build",
			args:  []string{"claude"},
			files: map[string]string{goldenExtensionFile: "image:\n  apt: [tmux, neovim, tmux]\n"},
		},
		{
			name:   "extension-current",
			args:   []string{"--verbose", "--no-config", "claude"},
			files:  map[string]string{goldenExtensionFile: "image:\n  apt: [neovim, tmux]\n"},
			images: map[string]fakeImage{images.BaseRef: currentBase(), images.ExtensionRef: currentExtension([]string{"neovim", "tmux"}, "sha256:fake-base")},
		},
		{
			name:  "extension-xdg-config-home",
			args:  []string{"--no-config", "claude"},
			env:   map[string]string{"XDG_CONFIG_HOME": "$ROOT/xdg"},
			files: map[string]string{"xdg/caged/config.yaml": "image:\n  apt: [tmux]\n"},
		},
		{
			name:   "extension-stale-after-edit",
			args:   []string{"--no-config", "claude"},
			files:  map[string]string{goldenExtensionFile: "image:\n  apt: [neovim, tmux, shellcheck]\n"},
			images: map[string]fakeImage{images.BaseRef: currentBase(), images.ExtensionRef: staleExtension("sha256:fake-base")},
		},
		{
			name:  "extension-invalid",
			args:  []string{"claude"},
			files: map[string]string{goldenExtensionFile: "image:\n  apt: [\"vim; curl x\"]\n"},
		},
		{name: "build", args: []string{"build"}},
		{name: "build-no-cache", args: []string{"build", "--no-cache"}},
		{
			name:  "build-with-extension",
			args:  []string{"build"},
			files: map[string]string{goldenExtensionFile: "image:\n  apt: [neovim, tmux]\n"},
		},
		{name: "build-daemon-down", args: []string{"build"}, fail: map[string]fakeFailure{"info": daemonDown}},
	}
}

// commandCases: help, version, completion, argument checks, doctor, list and
// clean.
func commandCases() []goldenCase {
	return []goldenCase{
		{name: "list", args: []string{"list"}},
		{name: "version", args: []string{"--version"}},
		{name: "help", args: []string{"--help"}},
		{name: "help-list", args: []string{"help", "list"}},
		{name: "help-unknown-topic", args: []string{"help", "nope"}},
		{name: "build-help", args: []string{"build", "--help"}},
		{name: "unknown-flag", args: []string{"--bogus"}},
		{name: "build-rejects-args", args: []string{"build", "extra"}},
		{name: "list-rejects-args", args: []string{"list", "extra"}},
		{name: "complete-agents", args: []string{"__complete", ""}},
		{name: "complete-prompt", args: []string{"__complete", "claude", ""}},
		{name: "doctor-ok", args: []string{"doctor"}},
		{name: "doctor-daemon-down", args: []string{"doctor"}, fail: map[string]fakeFailure{"info": daemonDown}},
		{name: "doctor-arch-mismatch", args: []string{"doctor"}, images: map[string]fakeImage{images.BaseRef: withArch(currentBase(), "s390x")}},
		{name: "doctor-legacy-image-json", args: []string{"doctor"}, files: map[string]string{"home/.config/caged/image.json": `{"apt": ["jq"]}`}},
		{name: "clean", args: []string{"clean"}, ls: cleanFixture()},
		{name: "clean-dry-run", args: []string{"clean", "--dry-run"}, ls: cleanFixture()},
		{
			name: "clean-in-use",
			args: []string{"clean"},
			ls:   cleanFixture(),
			fail: map[string]fakeFailure{"image-rm": {Code: 1, Stderr: "Error response from daemon: conflict: unable to delete sha256:aaa (cannot be forced) - image is being used by running container 0123456789ab"}},
		},
	}
}

// agentCases: every agent started plain and with a prompt in its prompt style.
func agentCases() []goldenCase {
	var cases []goldenCase
	for _, name := range []string{"claude", "codex", "copilot", "gemini", "gh", "pi"} {
		cases = append(cases,
			goldenCase{name: "agent-" + name, args: []string{name}},
			goldenCase{name: "prompt-" + name, args: []string{name, goldenPrompt}},
		)
	}
	return cases
}
