package system

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"testing"
)

// signalHelperEnv makes TestExecutorSignalHelperProcess act as the helper:
// "catch" runs its child with CatchSignals, "plain" without, "after" runs a
// first command with CatchSignals and then the child without, and "nohup"
// ignores SIGHUP, as nohup does, before a run with CatchSignals.
const signalHelperEnv = "CAGED_SIGNAL_HELPER"

// TestExecutorSignalHelperProcess is not a test of its own: the signal tests
// re-exec the test binary to run it, so the signal they send reaches that
// process and not the test runner. It runs a child that announces itself,
// sleeps and finishes, then prints how Run returned.
func TestExecutorSignalHelperProcess(t *testing.T) {
	mode := os.Getenv(signalHelperEnv)
	if mode == "" {
		t.Skip("helper process for TestExecutorCatchesSignalsWhileTheChildRuns")
	}
	switch mode {
	case "after":
		if err := (Executor{}).Run(context.Background(), Command{Path: "/bin/sh", Args: []string{"-c", "exit 0"}, CatchSignals: true}); err != nil {
			fmt.Printf("result: first run: %v\n", err)
		}
	case "nohup":
		signal.Ignore(syscall.SIGHUP)
	}
	err := Executor{}.Run(context.Background(), Command{
		Path:         "/bin/sh",
		Args:         []string{"-c", "echo ready; sleep 1; echo child-done"},
		Stdout:       os.Stdout,
		CatchSignals: mode == "catch" || mode == "nohup",
	})
	if signalled, ok := errors.AsType[*SignalError](err); ok {
		fmt.Printf("result: signal %d %q child-err=%v\n", signalled.ExitCode(), err.Error(), signalled.err)
	} else {
		fmt.Printf("result: %v\n", err)
	}
	os.Exit(0)
}

// runSignalHelper starts the helper in mode, sends it sig once its child is
// running, and returns everything it printed and how it ended.
func runSignalHelper(t *testing.T, mode string, sig syscall.Signal) (string, *os.ProcessState) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestExecutorSignalHelperProcess$", "-test.count=1")
	cmd.Env = append(os.Environ(), signalHelperEnv+"="+mode)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	lines := bufio.NewScanner(out)
	var printed strings.Builder
	for lines.Scan() {
		printed.WriteString(lines.Text() + "\n")
		if lines.Text() == "ready" {
			if err := cmd.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
		}
	}
	_ = cmd.Wait() // the state says how it ended; a non-zero end is what some cases expect
	return printed.String(), cmd.ProcessState
}

// TestExecutorCatchesSignalsWhileTheChildRuns pins WB-T3 (bugs.md Q-CS-6,
// D-25): with CatchSignals, caged survives SIGINT, SIGHUP and SIGTERM sent to
// it alone, relays nothing (the child finishes its work), waits for the child,
// and returns a *SignalError whose exit code is the one the shell would have
// seen had caged died, 128 plus the signal number.
func TestExecutorCatchesSignalsWhileTheChildRuns(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		sig  syscall.Signal
		name string
	}{{syscall.SIGTERM, "terminated"}, {syscall.SIGINT, "interrupt"}, {syscall.SIGHUP, "hangup"}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			printed, state := runSignalHelper(t, "catch", tc.sig)
			want := fmt.Sprintf("ready\nchild-done\nresult: signal %d %q child-err=<nil>\n", 128+int(tc.sig), "received signal: "+tc.name)
			if printed != want || !state.Success() {
				t.Fatalf("helper printed %q and ended %v; want %q and a clean exit", printed, state, want)
			}
		})
	}
}

// TestExecutorLeavesSignalsAloneByDefault: without CatchSignals, Run changes
// nothing about how caged reacts to a signal: SIGTERM ends it at once.
func TestExecutorLeavesSignalsAloneByDefault(t *testing.T) {
	t.Parallel()
	printed, state := runSignalHelper(t, "plain", syscall.SIGTERM)
	status, ok := state.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGTERM || strings.Contains(printed, "result:") {
		t.Fatalf("helper printed %q and ended %v; want it killed by SIGTERM before Run returned", printed, state)
	}
}

func TestSignalErrorUnwrapsTheCommandError(t *testing.T) {
	t.Parallel()
	exit := errors.New("exit status 1")
	err := &SignalError{Signal: syscall.SIGTERM, err: exit}
	if !errors.Is(err, exit) || err.ExitCode() != 143 || err.Error() != "received signal: terminated" {
		t.Fatalf("SignalError = %q, exit code %d, unwraps %v", err.Error(), err.ExitCode(), errors.Unwrap(err))
	}
}

// TestExecutorRestoresSignalHandlingAfterTheRun: once a Run with
// CatchSignals returns, SIGTERM ends caged again, as before it.
func TestExecutorRestoresSignalHandlingAfterTheRun(t *testing.T) {
	t.Parallel()
	printed, state := runSignalHelper(t, "after", syscall.SIGTERM)
	status, ok := state.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGTERM || strings.Contains(printed, "result:") {
		t.Fatalf("helper printed %q and ended %v; want it killed by SIGTERM during the second run", printed, state)
	}
}

// TestExecutorKeepsAnIgnoredSignalIgnored: under nohup SIGHUP is ignored, and
// CatchSignals leaves it so: the run is not reported as signalled.
func TestExecutorKeepsAnIgnoredSignalIgnored(t *testing.T) {
	t.Parallel()
	printed, state := runSignalHelper(t, "nohup", syscall.SIGHUP)
	if printed != "ready\nchild-done\nresult: <nil>\n" || !state.Success() {
		t.Fatalf("helper printed %q and ended %v; want the run to finish untouched", printed, state)
	}
}

// TestExecutorReportsACommandEndedByASignal pins WB-B3 (bugs.md Q-WB-3): with
// CatchSignals, a command that a signal ended (docker's CLI killed on its own,
// by kill -9 or the OOM killer) is reported as a *KilledError, so the caller
// can tell it from an exit status, and its exit code is the shell's 128 plus
// the signal number, not the -1 of exec.ExitError. The message is exec's.
func TestExecutorReportsACommandEndedByASignal(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		sig  syscall.Signal
		name string
		text string
	}{{syscall.SIGKILL, "KILL", "signal: killed"}, {syscall.SIGHUP, "HUP", "signal: hangup"}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := Executor{}.Run(context.Background(), Command{Path: "/bin/sh", Args: []string{"-c", "kill -" + tc.name + " $$"}, CatchSignals: true})
			var killed *KilledError
			var exitErr *exec.ExitError
			if !errors.As(err, &killed) || killed.Signal != tc.sig || killed.ExitCode() != 128+int(tc.sig) || err.Error() != tc.text || !errors.As(err, &exitErr) {
				t.Fatalf("Run() = %#v (%v); want a *KilledError for %v around the *exec.ExitError, exit code %d", err, err, tc.sig, 128+int(tc.sig))
			}
		})
	}
}

// TestExecutorKeepsExitStatusesAndPlainRunsAsTheyWere: an ordinary non-zero
// exit stays an *exec.ExitError with its own code, and without CatchSignals
// a command ended by a signal is reported as before.
func TestExecutorKeepsExitStatusesAndPlainRunsAsTheyWere(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		script string
		catch  bool
		code   int
	}{{"exit 3", true, 3}, {"kill -KILL $$", false, -1}} {
		err := Executor{}.Run(context.Background(), Command{Path: "/bin/sh", Args: []string{"-c", tc.script}, CatchSignals: tc.catch})
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || errors.As(err, new(*KilledError)) || exitErr.ExitCode() != tc.code {
			t.Errorf("%q with CatchSignals=%v: Run() = %#v; want a plain *exec.ExitError with code %d", tc.script, tc.catch, err, tc.code)
		}
	}
}
