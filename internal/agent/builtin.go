package agent

// Builtin returns the agents that ship with caged. To add an agent, create a
// file in this package with a function that returns its Agent value, then add
// one line calling that function to the list below.
func Builtin() *Registry {
	registry, err := NewRegistry(
		claude(),
		codex(),
		copilot(),
		gemini(),
		gh(),
		pi(),
	)
	if err != nil {
		// The built-in definitions are fixed at compile time, and
		// TestBuiltinRegistryContract fails before a broken one can ship.
		panic(err)
	}
	return registry
}
