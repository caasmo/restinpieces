package main

import (
	"fmt"

	toml "github.com/pelletier/go-toml"
)

// This file adds one user agent to the block_user_agent.agents slice.

// userAgentAdder adds a user agent to the block_user_agent.agents slice.
type userAgentAdder struct{}

func (userAgentAdder) Add(tree *toml.Tree, path, value string) error {
	if value == "" {
		return fmt.Errorf("user agent must not be empty")
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
