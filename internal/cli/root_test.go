package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCmdRejectsExtraArguments(t *testing.T) {
	t.Parallel()

	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"unexpected"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an error for an extra argument")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("Execute() error = %q, want an unexpected-argument error", err)
	}
}
