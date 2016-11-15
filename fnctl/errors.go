package main

type notFoundError struct {
	S string
}

func (e *notFoundError) Error() string {
	return e.S
}

func newNotFoundError(s string) *notFoundError {
	return &notFoundError{S: s}
}
