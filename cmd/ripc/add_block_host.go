package main

import (
	"fmt"

	toml "github.com/pelletier/go-toml"
)

// This file adds one host to the block_host.allowed_hosts slice.

// blockHostAdder adds a host to the block_host.allowed_hosts slice.
type blockHostAdder struct{}

func (blockHostAdder) Add(tree *toml.Tree, path, value string) error {
	if value == "" {
		return fmt.Errorf("host must not be empty")
	}

	raw, ok := tree.Get(path).([]interface{})
	if !ok {
		return fmt.Errorf("%w: %s is not a list", ErrNotCollection, path)
	}

	for _, item := range raw {
		if str, ok := item.(string); ok && str == value {
			return nil
		}
	}

	tree.Set(path, append(raw, value))
	return nil
}
