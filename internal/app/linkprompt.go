package app

import (
	"fmt"

	"github.com/JeremyOrr4/caged/internal/pathpolicy"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// LinkAllower adds a links.allow entry to config.yaml and returns the file's
// new content. *userconfig.Store implements it.
type LinkAllower interface {
	AllowLink(path string, readOnly bool) ([]byte, error)
}

// LinkPrompt lets a run offer to add the links.allow entry that a config root
// reached through a symlink (a Stow-linked ~/.pi) needs. Without it the
// agent's whole config is left out, and everything the agent changes there
// is lost when it exits.
type LinkPrompt struct {
	Prompter  *Prompter
	Allower   LinkAllower
	Approvals ConfigApprovals // records the edited file, which the user just approved
}

// WithLinkPrompt makes the planner offer the links.allow entries for refused
// config roots, and returns it.
func (p *Planner) WithLinkPrompt(prompt LinkPrompt) *Planner {
	p.links = &prompt
	return p
}

// concerns reports whether refusal, of a whole config root, matters to this
// run: it is the selected agent's, or shared by every agent. Other agents'
// roots are reported by --verbose and caged doctor.
func concerns(request Request, refusal plan.Refusal) bool {
	if refusal.Agent == plan.SharedOwner {
		return true
	}
	return request.Agent != nil && refusal.Agent == request.Agent.Name
}

// offerLinks asks, for each refused config root of this run that a
// links.allow entry would mount, whether to add that entry, and adds it on a
// yes. It reports whether config.yaml changed, so the config must be
// resolved again. It asks nothing without a terminal or with --trust-config:
// trusting a changed file is not agreeing to widen the link policy.
func (p *Planner) offerLinks(request Request, report plan.ConfigReport, home string, rep *Reporter) bool {
	if p.links == nil || request.TrustConfig || !p.links.Prompter.Interactive() {
		return false
	}
	changed := false
	asked := make(map[string]bool)
	for _, refusal := range report.Refused {
		if refusal.Allow == "" || asked[refusal.Allow] || !concerns(request, refusal) {
			continue
		}
		asked[refusal.Allow] = true
		allow := pathpolicy.Tilde(home, refusal.Allow)
		access := "read/write"
		if refusal.AllowReadOnly {
			access = "read-only"
		}
		rep.Notice("\ncaged: %s.", describeRoot(refusal, home))
		if !p.links.Prompter.Confirm(fmt.Sprintf("Add %s to links.allow (%s) so it is mounted?", allow, access)) {
			continue
		}
		data, err := p.links.Allower.AllowLink(refusal.Allow, refusal.AllowReadOnly)
		if err != nil {
			rep.Notice("caged: links.allow not changed: %v", err)
			continue
		}
		changed = true
		if err := p.links.Approvals.Approve(data); err != nil {
			rep.Notice("caged: warning: %v; the next run will ask you to approve the change", err)
		}
		rep.Notice("caged: added %s to links.allow.", allow)
	}
	return changed
}

// describeRoot says, in plain words, which config is not mounted, why, and
// what that means for the agent.
func describeRoot(refusal plan.Refusal, home string) string {
	path := pathpolicy.Tilde(home, refusal.Path)
	if refusal.Agent == plan.SharedOwner {
		return fmt.Sprintf("shared config %s is NOT mounted (%s), so no agent sees it", path, refusal.Reason)
	}
	return fmt.Sprintf("%s config %s is NOT mounted (%s), so %s runs without it and anything %s changes there is lost when it exits",
		refusal.Agent, path, refusal.Reason, refusal.Agent, refusal.Agent)
}

// rootWarning is the line printed on every run, and by caged doctor, for a
// refused config root.
func rootWarning(refusal plan.Refusal, home string) string {
	text := describeRoot(refusal, home)
	if refusal.Hint != "" {
		text += ". Fix: " + refusal.Hint
	}
	if refusal.Allow != "" {
		text += ", or answer y when caged asks"
	}
	return text
}
