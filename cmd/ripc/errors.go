package main

import "errors"

// Shared error variables for the ripc command.
var (
	ErrSecureStoreGet  = errors.New("failed to get from secure store")
	ErrSecureStoreSave = errors.New("failed to save to secure store")
	ErrConfigUnmarshal = errors.New("failed to unmarshal config")
	ErrConfigMarshal   = errors.New("failed to marshal config")
	ErrWriteOutput     = errors.New("failed to write output")
)
