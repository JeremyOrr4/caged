package app

import (
	"context"
	"testing"
)

// These tests pin the Guarded* decorators: which commands create the file,
// which ask, and which only report.

type fakeNext struct{ calls []string }

func (f *fakeNext) Run(context.Context, Request) error { f.calls = append(f.calls, "run"); return nil }
func (f *fakeNext) Build(context.Context, BuildOptions) ([]BuiltImage, error) {
	f.calls = append(f.calls, "build")
	return nil, nil
}
func (f *fakeNext) Diagnose(context.Context, DoctorOptions) DoctorReport {
	f.calls = append(f.calls, "diagnose")
	return DoctorReport{}
}
func (f *fakeNext) List() ([]AgentStatus, error) { f.calls = append(f.calls, "list"); return nil, nil }

func TestGuardedRunnerStopsBeforeRunningUnapprovedConfig(t *testing.T) {
	t.Parallel()

	r := newRig(configV2, configV1, true)
	r.answer = "n\n"
	next := &fakeNext{}
	if err := (GuardedRunner{Guard: r.guard(), Next: next}).Run(context.Background(), Request{}); err == nil {
		t.Fatal("Run() succeeded with an unapproved config")
	}
	if len(next.calls) != 0 {
		t.Errorf("wrapped use case ran: %v", next.calls)
	}
}

func TestGuardedRunnerSkipsCreationWithNoConfigAndPassesTrust(t *testing.T) {
	t.Parallel()

	r := newRig(configV1, configV1, true)
	next := &fakeNext{}
	if err := (GuardedRunner{Guard: r.guard(), Next: next}).Run(context.Background(), Request{NoConfig: true}); err != nil {
		t.Fatal(err)
	}
	if r.init.calls != 0 || len(next.calls) != 1 {
		t.Errorf("Ensure calls %d, wrapped calls %v; want no creation and one run", r.init.calls, next.calls)
	}

	r = newRig(configV2, configV1, true)
	r.tty = false
	if err := (GuardedRunner{Guard: r.guard(), Next: &fakeNext{}}).Run(context.Background(), Request{TrustConfig: true}); err != nil {
		t.Errorf("Run(--trust-config) error = %v", err)
	}
}

func TestGuardedBuilderCreatesAndHonorsTrust(t *testing.T) {
	t.Parallel()

	r := newRig(configV2, configV1, true)
	r.tty = false
	next := &fakeNext{}
	if _, err := (GuardedBuilder{Guard: r.guard(), Next: next}).Build(context.Background(), BuildOptions{}); err == nil {
		t.Error("Build() succeeded without approval")
	}
	if _, err := (GuardedBuilder{Guard: r.guard(), Next: next}).Build(context.Background(), BuildOptions{TrustConfig: true}); err != nil {
		t.Errorf("Build(trust) error = %v", err)
	}
	if r.init.calls != 2 || len(next.calls) != 1 {
		t.Errorf("Ensure calls %d, wrapped calls %v; want 2 and one build", r.init.calls, next.calls)
	}
}

func TestGuardedDoctorAndListerAlwaysGoOn(t *testing.T) {
	t.Parallel()

	r := newRig(configV2, configV1, true)
	r.tty = false
	next := &fakeNext{}
	(GuardedDoctor{Guard: r.guard(), Next: next}).Diagnose(context.Background(), DoctorOptions{})
	if _, err := (GuardedLister{Guard: r.guard(), Next: next}).List(); err != nil {
		t.Fatal(err)
	}
	if len(next.calls) != 2 || r.init.calls != 2 {
		t.Errorf("wrapped calls %v, Ensure calls %d; want both to go on after creating", next.calls, r.init.calls)
	}
	if len(r.approvals.approvedTo) != 0 {
		t.Errorf("approvals %v; reporting commands must not approve", r.approvals.approvedTo)
	}
}
