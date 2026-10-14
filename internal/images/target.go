package images

import (
	"fmt"
	"sort"

	"github.com/JeremyOrr4/caged/internal/plan"
)

// Target is one image caged builds and keeps current: its tag, its build
// context, and the labels that identify what it was built from.
type Target struct {
	Role        Role
	Ref         string
	Files       []plan.File
	Identity    []plan.Label // labels that must match for the image to be current
	Version     string       // the caged version that would build it
	Source      string       // where an extension came from; empty for the base
	ContextName string       // names the temporary build directory
}

// Base returns the runtime image built by the caged version.
func Base(version string) Target {
	files := baseFiles()
	return Target{
		Role:  RoleRuntime,
		Ref:   BaseRef,
		Files: files,
		Identity: []plan.Label{
			{Key: LabelRole, Value: string(RoleRuntime)},
			{Key: LabelRuntimeHash, Value: ContextHash(files)},
		},
		Version:     version,
		ContextName: "caged-runtime",
	}
}

// Spec returns the build of the target. Its labels are the identity labels and
// the version, sorted by key. The returned spec shares no memory with t.
func (t Target) Spec(opts plan.BuildOptions) plan.BuildSpec {
	files := make([]plan.File, len(t.Files))
	for i, file := range t.Files {
		file.Data = append([]byte(nil), file.Data...)
		files[i] = file
	}
	labels := append(append([]plan.Label(nil), t.Identity...), plan.Label{Key: LabelVersion, Value: t.Version})
	sort.Slice(labels, func(i, j int) bool { return labels[i].Key < labels[j].Key })
	return plan.BuildSpec{
		Image:       t.Ref,
		Dockerfile:  "Dockerfile",
		ContextName: t.ContextName,
		Files:       files,
		Labels:      labels,
		NoCache:     opts.NoCache,
	}
}

// State says whether an installed image matches its target.
type State int

// The states of an installed image.
const (
	StateCurrent   State = iota
	StateMissing         // no image with the target's tag
	StateUnlabeled       // built before caged wrote labels, or by something else
	StateChanged         // labeled, but built from different files
)

// Freshness is the result of Assess.
type Freshness struct {
	State   State
	BuiltBy string // the caged version label of the installed image, if any
}

// Assess compares the labels of the installed image with the target. found is
// false when no image has the target's tag. An image without the role label or
// the target's hash label is unlabeled, and counts as stale.
func (t Target) Assess(labels map[string]string, found bool) Freshness {
	if !found {
		return Freshness{State: StateMissing}
	}
	freshness := Freshness{BuiltBy: labels[LabelVersion]}
	if _, ok := labels[LabelRole]; !ok {
		freshness.State = StateUnlabeled
		return freshness
	}
	if _, ok := labels[t.hashLabel()]; !ok {
		freshness.State = StateUnlabeled
		return freshness
	}
	for _, want := range t.Identity {
		if got, ok := labels[want.Key]; !ok || got != want.Value {
			freshness.State = StateChanged
			return freshness
		}
	}
	freshness.State = StateCurrent
	return freshness
}

// hashLabel is the label that holds the hash of the target's build context.
func (t Target) hashLabel() string {
	if t.Role == RoleExtension {
		return LabelExtensionHash
	}
	return LabelRuntimeHash
}

// Notice returns the message printed before the target is built, or "" when
// it is current. The "built by caged X" form is used only when the installed
// image has a version label that differs from the target's.
func (t Target) Notice(f Freshness) string {
	if t.Role == RoleExtension {
		if f.State == StateCurrent {
			return ""
		}
		return fmt.Sprintf("Building image extension from %s...", t.Source)
	}
	switch f.State {
	case StateMissing:
		return fmt.Sprintf("Runtime image %s not found; building it first...", t.Ref)
	case StateUnlabeled:
		return fmt.Sprintf("Runtime image %s was built by an older caged; rebuilding it once (this can take a few minutes)...", t.Ref)
	case StateChanged:
		if f.BuiltBy != "" && f.BuiltBy != t.Version {
			return fmt.Sprintf("Runtime image %s was built by caged %s; rebuilding it for caged %s (this can take a few minutes)...", t.Ref, f.BuiltBy, t.Version)
		}
		return fmt.Sprintf("Runtime image %s is out of date (its runtime files changed); rebuilding...", t.Ref)
	default:
		return ""
	}
}

// Describe returns the --verbose text for a target that is current.
func (t Target) Describe() string {
	if t.Role == RoleExtension {
		return fmt.Sprintf("extension from %s -> %s", t.Source, t.Ref)
	}
	return fmt.Sprintf("%s is current (runtime %.12s)", t.Ref, t.labelValue(t.hashLabel()))
}

func (t Target) labelValue(key string) string {
	for _, label := range t.Identity {
		if label.Key == key {
			return label.Value
		}
	}
	return ""
}

// WrapBuildError adds target-specific advice to a failed build. A failed
// extension names its file, because a typo in a package name blocks every run
// until the file is fixed. The runtime image has no such advice.
func (t Target) WrapBuildError(err error) error {
	if t.Role == RoleExtension {
		return fmt.Errorf("building the image extension from %s failed; fix or remove that file: %w", t.extensionPath(), err)
	}
	return err
}
