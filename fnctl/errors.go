package main

type NotFoundError struct {
	S string
}

func (e *NotFoundError) Error() string {
	return e.S
}

func newNotFoundError(s string) *NotFoundError {
	return &NotFoundError{S: s}
}
