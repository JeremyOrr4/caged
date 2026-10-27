package images

import (
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
)

func TestContextHash(t *testing.T) {
	t.Parallel()

	base := []plan.File{
		{Name: "Dockerfile", Data: []byte("FROM scratch\n"), Mode: 0o644},
		{Name: "entrypoint", Data: []byte("#!/bin/sh\n"), Mode: 0o755},
	}
	want := ContextHash(base)

	t.Run("is stable and a lowercase SHA-256", func(t *testing.T) {
		t.Parallel()
		if got := ContextHash(base); got != want || len(got) != 64 || strings.Trim(got, "0123456789abcdef") != "" {
			t.Fatalf("ContextHash() = %q, want %q (64 hex characters)", got, want)
		}
	})
	t.Run("does not depend on the order of files", func(t *testing.T) {
		t.Parallel()
		reversed := []plan.File{base[1], base[0]}
		if got := ContextHash(reversed); got != want {
			t.Fatalf("reordered hash = %q, want %q", got, want)
		}
		if reversed[0].Name != "entrypoint" {
			t.Fatal("ContextHash reordered its argument")
		}
	})
	t.Run("ignores the Label field", func(t *testing.T) {
		t.Parallel()
		labeled := []plan.File{{Name: "Dockerfile", Label: "x", Data: base[0].Data, Mode: 0o644}, base[1]}
		if got := ContextHash(labeled); got != want {
			t.Fatalf("hash changed with the error-message label: %q", got)
		}
	})
	tests := []struct {
		name  string
		files []plan.File
	}{
		{name: "a changed byte", files: []plan.File{{Name: "Dockerfile", Data: []byte("FROM scratcH\n"), Mode: 0o644}, base[1]}},
		{name: "a changed name", files: []plan.File{{Name: "Dockerfile2", Data: base[0].Data, Mode: 0o644}, base[1]}},
		{name: "a changed mode", files: []plan.File{base[0], {Name: "entrypoint", Data: base[1].Data, Mode: 0o644}}},
		{name: "a removed file", files: base[:1]},
		{name: "a file moving its bytes into its neighbor", files: []plan.File{{Name: "Dockerfile", Data: []byte("FROM scratch\n#!/bin/sh\n"), Mode: 0o644}, {Name: "entrypoint", Data: nil, Mode: 0o755}}},
	}
	for _, tt := range tests {
		t.Run("changes with "+tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ContextHash(tt.files); got == want {
				t.Fatalf("hash did not change for %s", tt.name)
			}
		})
	}
}

// TestContextHashKnownAnswer pins the hash itself, not only its properties: a
// change to the encoding would rebuild every user's image once. The expected
// value was computed outside Go, with Python's hashlib over the documented
// encoding (prefix, then per file sorted by name: name, octal mode, length, data).
func TestContextHashKnownAnswer(t *testing.T) {
	t.Parallel()

	files := []plan.File{{Name: "b", Mode: 0o644, Data: []byte("two\n")}, {Name: "a", Mode: 0o755, Data: []byte("one")}}
	if got, want := ContextHash(files), "611767621eabf2e8130fce8fbe24075bd9583d5073e05022cb1b4b8e7dfc44e5"; got != want {
		t.Errorf("ContextHash = %s, want %s", got, want)
	}
}
