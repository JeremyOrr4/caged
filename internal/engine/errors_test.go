package engine

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorText(t *testing.T) {
	t.Parallel()

	cause := errors.New("exec: \"docker\": executable file not found in $PATH")
	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{name: "message only", err: &Error{Kind: KindDaemonDown, Message: "daemon unavailable"}, want: "daemon unavailable"},
		{name: "message and cause", err: &Error{Kind: KindNotInstalled, Message: "Docker is not installed", Err: cause}, want: `Docker is not installed: exec: "docker": executable file not found in $PATH`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.err.Error(); got != tt.want {
				t.Fatalf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUnwrapExposesTheCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("boom")
	err := fmt.Errorf("outer: %w", &Error{Kind: KindImageInUse, Message: "in use", Err: cause})
	if !errors.Is(err, cause) {
		t.Fatal("errors.Is does not see through *Error to its cause")
	}
}

func TestIsKind(t *testing.T) {
	t.Parallel()

	inUse := &Error{Kind: KindImageInUse, Message: "in use"}
	tests := []struct {
		name string
		err  error
		kind Kind
		want bool
	}{
		{name: "same kind", err: inUse, kind: KindImageInUse, want: true},
		{name: "different kind", err: inUse, kind: KindImageNotFound, want: false},
		{name: "wrapped", err: fmt.Errorf("context: %w", inUse), kind: KindImageInUse, want: true},
		{name: "joined", err: errors.Join(errors.New("other"), inUse), kind: KindImageInUse, want: true},
		{name: "plain error", err: errors.New("plain"), kind: KindUnknown, want: false},
		{name: "nil", err: nil, kind: KindUnknown, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsKind(tt.err, tt.kind); got != tt.want {
				t.Fatalf("IsKind(%v, %v) = %v, want %v", tt.err, tt.kind, got, tt.want)
			}
		})
	}
}
