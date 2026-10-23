// Package apperr holds the error kinds all features share, so httpx maps kinds to statuses without importing any feature.
package apperr

type Kind string

const (
	KindInvalid      Kind = "invalid"
	KindUnauthorized Kind = "unauthorized"
	KindForbidden    Kind = "forbidden"
	KindNotFound     Kind = "not_found"
	KindConflict     Kind = "conflict"
)

// Error carries a message that is safe to show to the client.
type Error struct {
	Kind    Kind
	Message string
	Fields  map[string]string
}

func (e *Error) Error() string { return e.Message }

func Invalid(message string) *Error      { return &Error{Kind: KindInvalid, Message: message} }
func Unauthorized(message string) *Error { return &Error{Kind: KindUnauthorized, Message: message} }
func Forbidden(message string) *Error    { return &Error{Kind: KindForbidden, Message: message} }
func NotFound(message string) *Error     { return &Error{Kind: KindNotFound, Message: message} }
func Conflict(message string) *Error     { return &Error{Kind: KindConflict, Message: message} }

// Validation reports one message per rejected field.
func Validation(fields map[string]string) *Error {
	return &Error{Kind: KindInvalid, Message: "validation failed", Fields: fields}
}
