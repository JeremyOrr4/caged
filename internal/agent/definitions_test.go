package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuiltinAgentDefinitionsAreUnchanged pins every field of the built-in
// agents so that moving the definitions around cannot silently change them.
func TestBuiltinAgentDefinitionsAreUnchanged(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	for _, name := range []string{"claude", "codex", "copilot", "gemini", "gh", "pi"} {
		definition, err := Builtin().Lookup(name)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "%#v\n", definition)
	}
	path := filepath.Join("testdata", "builtin_agents.golden")
	want, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && os.Getenv("CAGED_UPDATE_GOLDEN") == "1" {
		// Create a missing golden file; existing files are never overwritten.
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err != nil {
		t.Fatalf("read golden file (run with CAGED_UPDATE_GOLDEN=1 to create it): %v", err)
	}
	if got := b.String(); got != string(want) {
		t.Errorf("built-in agent definitions changed\n--- want\n%s\n--- got\n%s", want, got)
	}
}
