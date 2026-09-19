package main

import (
	"fmt"

	"github.com/caasmo/restinpieces/crypto"
	toml "github.com/pelletier/go-toml"
)

// This file produces fresh random secrets for the jwt.* configuration paths.

// jwtUpdater produces a fresh random secret.
type jwtUpdater struct{}

func (jwtUpdater) Update(tree *toml.Tree, arg string) error {
	for _, path := range tomlPaths(arg) {
		tree.Set(path, crypto.RandomString(32, crypto.AlphanumericAlphabet))
	}
	return nil
}

// Print reports the fresh secret by prefix and length only. Full secrets never reach scrollback.
func (jwtUpdater) Print(ui UI, tree *toml.Tree, arg string) error {
	for _, path := range tomlPaths(arg) {
		value, _ := tree.Get(path).(string)
		prefix := value
		if len(prefix) > 7 {
			prefix = prefix[:7]
		}
		_, err := fmt.Fprintf(ui.Err, "%s = %s... (%d chars)\n", path, prefix, len(value))
		if err != nil {
			return fmt.Errorf("%w: failed to write output: %w", ErrWriteOutput, err)
		}
	}

	return nil
}
