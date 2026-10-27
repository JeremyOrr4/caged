package hostconfig

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
	"github.com/JeremyOrr4/caged/internal/testkit"
)

// TestFoldCaseJudgesNamesInAnyCase pins W7-T8 (bugs.md Q-WB-7) at the
// resolver: with Deps.FoldCase, as on macOS, a root named ~/.SSH is the
// denied ~/.ssh and a state folder named in capitals overlaps caged's
// config folder. Without it, as on Linux, both are other folders.
func TestFoldCaseJudgesNamesInAnyCase(t *testing.T) {
	t.Parallel()
	for _, foldCase := range []bool{true, false} {
		home, xdg := realHome(t), realHome(t)
		writeFiles(t, home, ".SSH/id_ed25519")
		dir, state := filepath.Join(xdg, "caged"), filepath.Join(xdg, "Caged") // one folder where case is ignored
		writeFiles(t, dir, "config.yaml")
		links := policies{own: []string{dir, state}}
		report, err := NewResolver(Deps{
			FS: system.FS{}, Env: testkit.Env{Vars: map[string]string{"CLAUDE_CONFIG_DIR": filepath.Join(home, ".SSH")}},
			Agents: agent.Builtin().All(), Policies: links, ConfigDirs: stateDirs{dir, state}, FoldCase: foldCase,
		}).Resolve(home)
		if err != nil {
			t.Fatal(err)
		}
		ssh, sshRefused := refusalAt(report, filepath.Join(home, ".SSH"))
		cfg, cfgRefused := refusalAt(report, dir)
		_, sshMounted := mountAt(report, "/home/caged/.claude")
		_, cfgMounted := mountAt(report, "/home/caged/.config/caged")
		if foldCase {
			wantSSH := plan.Refusal{Path: filepath.Join(home, ".SSH"), Reason: "config path is denied", Hint: "~/.ssh is on caged's built-in deny list and cannot be allowed"}
			if !sshRefused || ssh != wantSSH || sshMounted {
				t.Errorf("FoldCase: ~/.SSH refusal = %+v, %v (mounted %v); want %+v", ssh, sshRefused, sshMounted, wantSSH)
			}
			if !cfgRefused || cfg.Reason != "config path overlaps caged's approval state folder "+state || cfgMounted {
				t.Errorf("FoldCase: caged folder refusal = %+v, %v (mounted %v); want the overlap with %s", cfg, cfgRefused, cfgMounted, state)
			}
			continue
		}
		if sshRefused || !sshMounted || cfgRefused || !cfgMounted {
			t.Errorf("exact names: ~/.SSH refused %v mounted %v, caged folder refused %v mounted %v; want both mounted; refused = %v", sshRefused, sshMounted, cfgRefused, cfgMounted, refusedStrings(report.Refused))
		}
	}
}

// TestFoldCaseRefusesALinkPolicyNamingHomeInOtherLetters: Resolve fails, as
// for any invalid link policy, when an allow root is the home directory in
// other letters (W7-R1).
func TestFoldCaseRefusesALinkPolicyNamingHomeInOtherLetters(t *testing.T) {
	t.Parallel()
	home := realHome(t)
	links := policies{allow: []string{strings.ToUpper(home)}}
	deps := Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: agent.Builtin().All(), Policies: links}
	if _, err := NewResolver(deps).Resolve(home); err != nil {
		t.Fatalf("exact Resolve error = %v; want the root accepted where names keep case", err)
	}
	deps.FoldCase = true
	if _, err := NewResolver(deps).Resolve(home); err == nil || !strings.Contains(err.Error(), "is too broad: it contains the home directory") {
		t.Fatalf("folded Resolve error = %v; want the allow root refused as too broad", err)
	}
}
