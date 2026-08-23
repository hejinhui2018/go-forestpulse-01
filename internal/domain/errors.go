package domain

import "fmt"

type ErrorKind string

const (
	KindValidation  ErrorKind = "validation"
	KindConflict    ErrorKind = "conflict"
	KindStorage     ErrorKind = "storage"
	KindNotFound    ErrorKind = "not_found"
	KindUnavailable ErrorKind = "unavailable"
)

type Error struct {
	Kind      ErrorKind
	Operation string
	Detail    string
	Cause     error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s %s: %s: %v", e.Kind, e.Operation, e.Detail, e.Cause)
	}
	return fmt.Sprintf("%s %s: %s", e.Kind, e.Operation, e.Detail)
}
func (e *Error) Unwrap() error { return e.Cause }
func E(kind ErrorKind, op, detail string, cause error) *Error {
	return &Error{Kind: kind, Operation: op, Detail: detail, Cause: cause}
}
