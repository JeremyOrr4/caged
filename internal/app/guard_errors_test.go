package app

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestPrepareErrors(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	t.Run("creation failure stops a run", func(t *testing.T) {
		t.Parallel()
		r := newRig(configV1, configV1, true)
		r.init.err = boom
		if err := r.guard().prepare(guardOptions{Create: true}); !errors.Is(err, boom) {
			t.Errorf("prepare() error = %v; want the creation error", err)
		}
	})
	t.Run("a config folder that cannot be created warns and the run goes on", func(t *testing.T) {
		t.Parallel()
		r := newRig("", "", false)
		r.content.found = false
		r.init.err = missingDirError{boom}
		if err := r.guard().prepare(guardOptions{Create: true}); err != nil {
			t.Fatalf("prepare() error = %v; want a warning: with no folder nothing is mounted", err)
		}
		if got := r.out.String(); got != "caged: warning: boom\n" {
			t.Errorf("output %q; want exactly the warning", got)
		}
		if !reflect.DeepEqual(r.approvals.approvedTo, []string{""}) {
			t.Errorf("approvals %q; want an empty approved copy, so a file that appears later is asked about", r.approvals.approvedTo)
		}
	})
	t.Run("a missing folder keeps an earlier approval", func(t *testing.T) {
		t.Parallel()
		r := newRig(configV1, configV1, true)
		r.init.err = missingDirError{boom}
		if err := r.guard().prepare(guardOptions{Create: true}); err != nil {
			t.Fatal(err)
		}
		if len(r.approvals.approvedTo) != 0 {
			t.Errorf("approvals %q; want the earlier approval kept", r.approvals.approvedTo)
		}
	})
	t.Run("a missing folder whose empty approval cannot be recorded stops the run", func(t *testing.T) {
		t.Parallel()
		r := newRig("", "", false)
		r.content.found = false
		r.init.err = missingDirError{boom}
		r.approvals.writeErr = errors.New("read-only state")
		if err := r.guard().prepare(guardOptions{Create: true}); err == nil || err.Error() != "read-only state" {
			t.Fatalf("prepare() error = %v; want the record failure", err)
		}
	})
	t.Run("a missing folder still stops a run with a changed file", func(t *testing.T) {
		t.Parallel()
		r := newRig(configV2, configV1, true)
		r.tty = false
		r.init.err = missingDirError{boom}
		if err := r.guard().prepare(guardOptions{Create: true}); err == nil || !strings.Contains(err.Error(), "no terminal to ask") {
			t.Fatalf("prepare() error = %v; want the review to stop the run", err)
		}
	})
	t.Run("creation failure only warns in notify mode", func(t *testing.T) {
		t.Parallel()
		r := newRig(configV1, configV1, true)
		r.init.err = boom
		r.guard().notify()
		if !strings.Contains(r.out.String(), "caged: warning: boom") {
			t.Errorf("output %q; want a warning", r.out.String())
		}
	})
	t.Run("an invalid file is the command's error", func(t *testing.T) {
		t.Parallel()
		r := newRig(configV1, configV1, true)
		r.content.err = boom
		if err := r.guard().prepare(guardOptions{}); !errors.Is(err, boom) {
			t.Errorf("prepare() error = %v; want the parse error", err)
		}
		r.out.Reset()
		r.guard().notify()
		if r.out.Len() != 0 {
			t.Errorf("notify output %q; want nothing: the command reports the invalid file", r.out.String())
		}
	})
	t.Run("an unreadable approval fails closed", func(t *testing.T) {
		t.Parallel()
		r := newRig(configV1, configV1, true)
		r.approvals.readErr = boom
		if err := r.guard().prepare(guardOptions{}); !errors.Is(err, boom) {
			t.Errorf("prepare() error = %v; want the read error", err)
		}
	})
	t.Run("a failed record fails closed", func(t *testing.T) {
		t.Parallel()
		r := newRig(configV1, "", false)
		r.approvals.writeErr = boom
		if err := r.guard().prepare(guardOptions{}); !errors.Is(err, boom) {
			t.Errorf("prepare() error = %v; want the write error", err)
		}
	})
	t.Run("a failed record of an approved change fails closed", func(t *testing.T) {
		t.Parallel()
		for _, opts := range []guardOptions{{Trust: true}, {}} {
			r := newRig(configV2, configV1, true)
			r.answer = "y\n"
			r.approvals.writeErr = boom
			if err := r.guard().prepare(opts); !errors.Is(err, boom) {
				t.Errorf("prepare(%+v) error = %v; want the write error", opts, err)
			}
		}
	})
	t.Run("a change whose path cannot be found fails closed", func(t *testing.T) {
		t.Parallel()
		r := newRig(configV2, configV1, true)
		r.content.sourceErr = boom
		r.answer = "y\n"
		if err := r.guard().prepare(guardOptions{}); !errors.Is(err, boom) {
			t.Errorf("prepare() error = %v; want the source error", err)
		}
		if r.out.Len() != 0 || len(r.approvals.approvedTo) != 0 {
			t.Errorf("output %q, approvals %v; want neither before the change can be shown", r.out.String(), r.approvals.approvedTo)
		}
		r.guard().notify()
		if r.out.Len() != 0 {
			t.Errorf("notify output %q; want nothing", r.out.String())
		}
	})
	t.Run("a created file whose path cannot be found is still recorded, without the notice", func(t *testing.T) {
		t.Parallel()
		r := newRig("# starter\n", "", false)
		r.init.created = true
		r.content.sourceErr = boom
		if err := r.guard().prepare(guardOptions{Create: true}); err != nil {
			t.Fatalf("prepare() error = %v; want nil", err)
		}
		if r.out.Len() != 0 {
			t.Errorf("output %q; want no notice without a path to name", r.out.String())
		}
		if len(r.approvals.approvedTo) != 1 || r.approvals.approvedTo[0] != "# starter\n" {
			t.Errorf("approved %v; want the starter recorded", r.approvals.approvedTo)
		}
	})
}

// TestNotifyWarnsWhenItCannotRecordAFirstCopy pins bugs.md Q-APP-2: the
// failure is shown as a warning, not dropped.
func TestNotifyWarnsWhenItCannotRecordAFirstCopy(t *testing.T) {
	t.Parallel()

	r := newRig(configV1, "", false)
	r.approvals.writeErr = errors.New("boom")
	r.guard().notify()
	if r.approvals.attempts != 1 {
		t.Fatalf("Approve calls %d; want the first copy tried once", r.approvals.attempts)
	}
	if got, want := r.out.String(), "caged: warning: boom\n"; got != want {
		t.Errorf("output %q, want %q", got, want)
	}
}

// missingDirError stands in for userconfig's error when the config folder
// could not be created.
type missingDirError struct{ err error }

func (e missingDirError) Error() string { return e.err.Error() }

func (e missingDirError) Unwrap() error { return e.err }

func (e missingDirError) MissingDir() bool { return true }
