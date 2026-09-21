package api

import "errors"

var (
	errSetupAlreadyDone         = errors.New("setup already completed")
	errInvalidCredentials       = errors.New("invalid username or password")
	errNotAuthenticated         = errors.New("not authenticated")
	errUnknownAutomationRule    = errors.New("unknown automation rule")
	errScriptNameOrContentEmpty = errors.New("name and content are required")
	errWorkflowNameEmpty        = errors.New("name is required")
)

const minPasswordLength = 8

func validateCredentials(username, password string) error {
	if len(username) < 3 {
		return errors.New("username must be at least 3 characters")
	}
	if len(password) < minPasswordLength {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}
