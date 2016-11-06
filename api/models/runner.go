package models

import "errors"

var (
	ErrRunnerRouteNotFound   = errors.New("Route not found on that application")
	ErrRunnerInvalidPayload  = errors.New("Invalid payload")
	ErrRunnerRunRoute        = errors.New("Couldn't run this route in the job server")
	ErrRunnerAPICantConnect  = errors.New("Couldn`t connect to the job server API")
	ErrRunnerAPICreateJob    = errors.New("Could not create a job in job server")
	ErrRunnerInvalidResponse = errors.New("Invalid response")
	ErrRunnerTimeout         = errors.New("Timed out")
)
