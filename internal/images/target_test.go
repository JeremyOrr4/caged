package images

import (
	"reflect"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
)

func TestTargetSpecLabelsAreSortedAndTheSpecIsACopy(t *testing.T) {
	t.Parallel()

	target := Base("0.1.0")
	spec := target.Spec(plan.BuildOptions{NoCache: true})
	wantLabels := []plan.Label{
		{Key: LabelRole, Value: "runtime"},
		{Key: LabelRuntimeHash, Value: ContextHash(target.Files)},
		{Key: LabelVersion, Value: "0.1.0"},
	}
	if !reflect.DeepEqual(spec.Labels, wantLabels) {
		t.Fatalf("Labels = %+v, want %+v", spec.Labels, wantLabels)
	}
	if spec.Image != BaseRef || spec.Dockerfile != "Dockerfile" || spec.ContextName != "caged-runtime" || !spec.NoCache {
		t.Fatalf("Spec() = %+v", spec)
	}
	if !reflect.DeepEqual(spec.Files, target.Files) {
		t.Fatalf("Spec() files differ from the target's")
	}
	spec.Files[0].Data[0] = 'X'
	spec.Labels[0].Value = "changed"
	if target.Files[0].Data[0] == 'X' || target.Identity[0].Value == "changed" || target.Spec(plan.BuildOptions{}).NoCache {
		t.Fatal("Spec must return fresh data and honor the options")
	}
}

func TestTargetAssess(t *testing.T) {
	t.Parallel()

	base := Base("0.2.0")
	hash := ContextHash(base.Files)
	current := map[string]string{LabelRole: "runtime", LabelRuntimeHash: hash, LabelVersion: "0.2.0"}
	with := func(key, value string) map[string]string {
		labels := map[string]string{}
		for k, v := range current {
			labels[k] = v
		}
		labels[key] = value
		return labels
	}
	without := func(key string) map[string]string {
		labels := with(key, "")
		delete(labels, key)
		return labels
	}
	extension := Target{Role: RoleExtension, Ref: ExtensionRef, Identity: []plan.Label{
		{Key: LabelRole, Value: "extension"}, {Key: LabelExtensionHash, Value: "ext-hash"}, {Key: LabelBaseID, Value: "sha256:base"},
	}, Version: "0.2.0"}
	extLabels := map[string]string{LabelRole: "extension", LabelExtensionHash: "ext-hash", LabelBaseID: "sha256:base", LabelVersion: "0.2.0"}

	tests := []struct {
		name   string
		target Target
		labels map[string]string
		found  bool
		want   Freshness
	}{
		{name: "not found", target: base, found: false, want: Freshness{State: StateMissing}},
		{name: "not found ignores labels", target: base, labels: current, found: false, want: Freshness{State: StateMissing}},
		{name: "no labels at all", target: base, labels: nil, found: true, want: Freshness{State: StateUnlabeled}},
		{name: "role label missing", target: base, labels: without(LabelRole), found: true, want: Freshness{State: StateUnlabeled, BuiltBy: "0.2.0"}},
		{name: "hash label missing", target: base, labels: without(LabelRuntimeHash), found: true, want: Freshness{State: StateUnlabeled, BuiltBy: "0.2.0"}},
		{name: "hash differs, other version", target: base, labels: map[string]string{LabelRole: "runtime", LabelRuntimeHash: "old", LabelVersion: "0.1.0"}, found: true, want: Freshness{State: StateChanged, BuiltBy: "0.1.0"}},
		{name: "hash differs, same version", target: base, labels: with(LabelRuntimeHash, "old"), found: true, want: Freshness{State: StateChanged, BuiltBy: "0.2.0"}},
		{name: "version label missing, hash matches", target: base, labels: without(LabelVersion), found: true, want: Freshness{State: StateCurrent}},
		{name: "role differs", target: base, labels: with(LabelRole, "extension"), found: true, want: Freshness{State: StateChanged, BuiltBy: "0.2.0"}},
		{name: "current", target: base, labels: current, found: true, want: Freshness{State: StateCurrent, BuiltBy: "0.2.0"}},
		{name: "current built by another version", target: base, labels: with(LabelVersion, "0.1.0"), found: true, want: Freshness{State: StateCurrent, BuiltBy: "0.1.0"}},
		{name: "extension current", target: extension, labels: extLabels, found: true, want: Freshness{State: StateCurrent, BuiltBy: "0.2.0"}},
		{name: "extension hash differs", target: extension, labels: func() map[string]string {
			l := map[string]string{}
			for k, v := range extLabels {
				l[k] = v
			}
			l[LabelExtensionHash] = "x"
			return l
		}(), found: true, want: Freshness{State: StateChanged, BuiltBy: "0.2.0"}},
		{name: "extension base id differs", target: extension, labels: func() map[string]string {
			l := map[string]string{}
			for k, v := range extLabels {
				l[k] = v
			}
			l[LabelBaseID] = "sha256:other"
			return l
		}(), found: true, want: Freshness{State: StateChanged, BuiltBy: "0.2.0"}},
		{name: "extension without a base id is stale", target: extension, labels: func() map[string]string {
			l := map[string]string{}
			for k, v := range extLabels {
				l[k] = v
			}
			delete(l, LabelBaseID)
			return l
		}(), found: true, want: Freshness{State: StateChanged, BuiltBy: "0.2.0"}},
		{name: "extension with only the base's labels is unlabeled", target: extension, labels: current, found: true, want: Freshness{State: StateUnlabeled, BuiltBy: "0.2.0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.target.Assess(tt.labels, tt.found); got != tt.want {
				t.Fatalf("Assess() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestTargetNotice(t *testing.T) {
	t.Parallel()

	target := Base("0.2.0")
	tests := []struct {
		name string
		in   Freshness
		want string
	}{
		{name: "missing", in: Freshness{State: StateMissing}, want: "Runtime image caged-runtime:latest not found; building it first..."},
		{name: "unlabeled", in: Freshness{State: StateUnlabeled}, want: "Runtime image caged-runtime:latest was built by an older caged; rebuilding it once (this can take a few minutes)..."},
		{name: "unlabeled ignores the version", in: Freshness{State: StateUnlabeled, BuiltBy: "0.1.0"}, want: "Runtime image caged-runtime:latest was built by an older caged; rebuilding it once (this can take a few minutes)..."},
		{name: "changed by another version", in: Freshness{State: StateChanged, BuiltBy: "0.1.0"}, want: "Runtime image caged-runtime:latest was built by caged 0.1.0; rebuilding it for caged 0.2.0 (this can take a few minutes)..."},
		{name: "changed, same version", in: Freshness{State: StateChanged, BuiltBy: "0.2.0"}, want: "Runtime image caged-runtime:latest is out of date (its runtime files changed); rebuilding..."},
		{name: "changed, no version label", in: Freshness{State: StateChanged}, want: "Runtime image caged-runtime:latest is out of date (its runtime files changed); rebuilding..."},
		{name: "current says nothing", in: Freshness{State: StateCurrent, BuiltBy: "0.1.0"}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := target.Notice(tt.in); got != tt.want {
				t.Fatalf("Notice() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTargetDescribeAndWrapBuildError(t *testing.T) {
	t.Parallel()

	target := Base("dev")
	want := "caged-runtime:latest is current (runtime " + ContextHash(target.Files)[:12] + ")"
	if got := target.Describe(); got != want {
		t.Fatalf("Describe() = %q, want %q", got, want)
	}
	boom := errTest("boom")
	if got := target.WrapBuildError(boom); got != boom {
		t.Fatalf("WrapBuildError() = %v, want the error unchanged", got)
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }
