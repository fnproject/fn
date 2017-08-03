package main

import "github.com/urfave/cli"

type notFoundError struct {
	S string
}

func (e *notFoundError) Error() string {
	return e.S
}

func newNotFoundError(s string) *notFoundError {
	return &notFoundError{S: s}
}

func clierr(err error) error {
	return cli.NewExitError(err, 1)
}
