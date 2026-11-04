package app

import (
	"context"
	"fmt"

	"github.com/JeremyOrr4/caged/internal/engine"
)

// minDockerEngine is the oldest Docker Engine version that caged's behavior
// (image labels, local FROM resolution, the port-in-use messages) was checked
// against. An older engine gets a warning from `caged doctor`; runs are never
// blocked.
const minDockerEngine = "25.0"

// Status is the outcome of one doctor check.
type Status int

// The statuses of a doctor check.
const (
	StatusPass Status = iota
	StatusWarn
	StatusFail
	StatusSkip
)

// String returns the label doctor prints for the status.
func (s Status) String() string {
	switch s {
	case StatusPass:
		return "PASS"
	case StatusWarn:
		return "WARN"
	case StatusFail:
		return "FAIL"
	default:
		return "SKIP"
	}
}

// Result is the outcome of one doctor check.
type Result struct {
	Name   string
	Status Status
	Detail string
}

// DoctorReport is the outcome of all checks: a header naming the caged version
// and platform, and one result per check, in order.
type DoctorReport struct {
	Header  string
	Results []Result
}

// Failed counts the checks that failed.
func (r DoctorReport) Failed() int {
	failed := 0
	for _, result := range r.Results {
		if result.Status == StatusFail {
			failed++
		}
	}
	return failed
}

// ChecksFailedError is returned when `caged doctor` found a failing check.
type ChecksFailedError struct{ Failed int }

// Error returns the text printed on stderr.
func (e *ChecksFailedError) Error() string {
	return fmt.Sprintf("doctor: %d checks failed", e.Failed)
}

// DoctorOptions are the settings of one doctor run.
type DoctorOptions struct {
	// WorkDir is the project directory to probe; empty means the current one.
	WorkDir string
}

// DoctorDeps are everything the doctor reads. It never builds, pulls, or
// removes an image, and never prints an environment value.
type DoctorDeps struct {
	Daemon     Daemon
	Store      ImageInspector
	Prober     Prober
	Extensions Extensions
	Config     ConfigFile
	Configs    ConfigResolver
	Project    ProjectResolver
	Env        Env
	Host       Host
	Version    string
}

// Doctor is the doctor use case.
type Doctor struct {
	deps DoctorDeps
}

// NewDoctor returns a Doctor that reads through deps.
func NewDoctor(deps DoctorDeps) *Doctor {
	return &Doctor{deps: deps}
}

// Diagnose runs every check, in order. A check that depends on a check that
// failed or was skipped is skipped, so one root cause does not produce a wall
// of failures.
func (d *Doctor) Diagnose(ctx context.Context, opts DoctorOptions) DoctorReport {
	return DoctorReport{
		Header:  fmt.Sprintf("caged %s (%s/%s)", d.deps.Version, d.deps.Host.OS, d.deps.Host.Arch),
		Results: runChecks(ctx, &facts{doctor: d, opts: opts, home: homeLookup{env: d.deps.Env}}, d.checks()),
	}
}

// runChecks runs checks in order and returns one result per check, named after
// it. A check is skipped when one of its needs failed, was skipped, or has not
// run yet.
func runChecks(ctx context.Context, known *facts, checks []check) []Result {
	var results []Result
	statuses := map[string]Status{}
	for _, check := range checks {
		var result Result
		if blocker, blocked := firstBlocker(check.needs, statuses); blocked {
			result = Result{Status: StatusSkip, Detail: "needs " + blocker}
		} else {
			result = check.run(ctx, known)
		}
		result.Name = check.name
		statuses[check.name] = result.Status
		results = append(results, result)
	}
	return results
}

// firstBlocker returns the first of needs that failed, was skipped, or has
// not run, so a need that names no earlier check fails safe.
func firstBlocker(needs []string, statuses map[string]Status) (string, bool) {
	for _, need := range needs {
		if status, ran := statuses[need]; !ran || status == StatusFail || status == StatusSkip {
			return need, true
		}
	}
	return "", false
}

// engineKindIsHostProblem reports whether err means docker cannot be used at
// all on this host, as opposed to a daemon that is not running.
func engineKindIsHostProblem(err error) bool {
	return engine.IsKind(err, engine.KindNotInstalled) || engine.IsKind(err, engine.KindUnsupportedHost)
}

// olderThan reports whether version is older than minimum, comparing only the
// numeric major.minor of each ("28.0.0-rc.1" counts as 28.0). ok is false when
// either side has no readable major.minor.
func olderThan(version, minimum string) (older bool, ok bool) {
	versionMajor, versionMinor, ok1 := majorMinor(version)
	minimumMajor, minimumMinor, ok2 := majorMinor(minimum)
	if !ok1 || !ok2 {
		return false, false
	}
	if versionMajor != minimumMajor {
		return versionMajor < minimumMajor, true
	}
	return versionMinor < minimumMinor, true
}

// majorMinor reads "N.N" from the start of version.
func majorMinor(version string) (major, minor int, ok bool) {
	major, rest, ok := leadingNumber(version)
	if !ok || len(rest) == 0 || rest[0] != '.' {
		return 0, 0, false
	}
	minor, _, ok = leadingNumber(rest[1:])
	return major, minor, ok
}

// leadingNumber parses the digits at the start of s.
func leadingNumber(s string) (number int, rest string, ok bool) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' && i < 9 {
		number = number*10 + int(s[i]-'0')
		i++
	}
	return number, s[i:], i > 0
}
