// Package engine holds the facts and errors that flow up from the container
// engine to the use cases. The plan package holds the specs that flow down.
package engine

import "errors"

// Kind classifies an engine failure, so callers can branch without reading
// message text.
type Kind int

// The kinds of engine failure caged distinguishes.
const (
	KindUnknown Kind = iota
	KindNotInstalled
	KindDaemonDown
	KindUnsupportedHost
	KindImageNotFound
	KindImageInUse
	KindPortInUse
)

// Error is an engine failure with a Kind. Its text is Message, followed by
// ": " and the cause when there is one.
type Error struct {
	Kind    Kind
	Message string
	Err     error
}

// Error returns the message, with the cause appended when there is one.
func (e *Error) Error() string {
	if e.Err == nil {
		return e.Message
	}
	return e.Message + ": " + e.Err.Error()
}

// Unwrap returns the cause, so errors.Is and errors.As see through an Error.
func (e *Error) Unwrap() error { return e.Err }

// IsKind reports whether err, or an error it wraps, is an *Error of kind k.
func IsKind(err error, k Kind) bool {
	var engineErr *Error
	return errors.As(err, &engineErr) && engineErr.Kind == k
}
