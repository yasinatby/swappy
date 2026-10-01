// Package apperr defines domain errors with a stable machine-readable code.
// The HTTP layer maps Kind to a status code and returns
// {"error": {"code": ..., "message": ...}}.
package apperr

type Kind int

const (
	Invalid Kind = iota
	NotFound
	Forbidden
	Conflict
	Unauthorized
)

type Error struct {
	Kind    Kind
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}
