// Package testkit holds fakes shared by the tests of several packages. Only
// _test.go files import it; cmd/caged/architecture_test.go enforces that.
package testkit

// Env is a fixed process environment. Its methods have value receivers, so
// Env and *Env both satisfy the consumer interfaces app.Env, userconfig.Env
// and hostconfig.Env, and a test may embed it to count or change one call.
type Env struct {
	Vars       map[string]string // what LookupEnv finds; a key with "" is set but empty
	Home       string
	HomeErr    error
	WorkDir    string
	WorkDirErr error
}

// LookupEnv reports Vars[key] and whether key is set.
func (e Env) LookupEnv(key string) (string, bool) {
	value, ok := e.Vars[key]
	return value, ok
}

// UserHomeDir returns Home, or HomeErr.
func (e Env) UserHomeDir() (string, error) { return e.Home, e.HomeErr }

// Getwd returns WorkDir, or WorkDirErr.
func (e Env) Getwd() (string, error) { return e.WorkDir, e.WorkDirErr }
