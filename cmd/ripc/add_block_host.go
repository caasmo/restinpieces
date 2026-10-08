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

	raw, ok := tomlArrayOf(tree.Get(path))
	if !ok {
		return fmt.Errorf("%w: %s is not an array", ErrNotCollection, path)
	}

	for _, item := range raw {
		if str, ok := item.(string); ok && str == value {
			return nil
		}
	}

	SetWithComment(tree, path, append(raw, value))
	return nil
}
