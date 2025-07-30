package main

import "errors"

// Shared error variables for the ripc command.
var (
	ErrScopeRequired   = errors.New("scope is required")
	ErrSecureStoreGet  = errors.New("failed to get from secure store")
	ErrConfigUnmarshal = errors.New("failed to unmarshal config")
	ErrConfigMarshal   = errors.New("failed to marshal config")
	ErrSecureStoreSave = errors.New("failed to save to secure store")
	ErrDbConnection    = errors.New("database connection error")
	ErrQueryPrepare    = errors.New("failed to prepare query")
	ErrWriteOutput     = errors.New("failed to write output")

	// command parsing errors
	ErrMissingArgument  = errors.New("missing required argument")
	ErrTooManyArguments = errors.New("too many arguments")
	ErrInvalidFlag      = errors.New("invalid flag provided")
	ErrNotANumber       = errors.New("argument must be a number")
)