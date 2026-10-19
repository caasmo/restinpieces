package main

import (
	"errors"
	"fmt"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml/v2"
)

// ErrConfigInvalid reports a document that unmarshals but fails config.Validate.
var ErrConfigInvalid = errors.New("invalid config")

// validateConfig decodes the whole document onto the defaults and runs the
// same config.Validate the app runs at startup, so a writer cannot save a
// config the app would refuse to load.
func validateConfig(data []byte, configPath string) error {
	cfg := config.NewDefaultConfig()
	if err := toml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("%w: refusing to save, value at '%s' breaks the config, run migrate to repair: %w", ErrConfigUnmarshal, configPath, err)
	}
	if err := config.Validate(cfg); err != nil {
		return fmt.Errorf("%w: refusing to save, '%s' leaves the config invalid: %w", ErrConfigInvalid, configPath, err)
	}
	return nil
}

// validateTomlAsConfig tries to unmarshal the updated TOML into config.Config.
// A bad type swap (like a TOML table turned into a string) fails here, before
// anything reaches the database.
//
// Deprecated: only checks TOML types, never the config rules. New callers use
// validateConfig; set, walk, rm, update and add still call this one.
func validateTomlAsConfig(data []byte, configPath string) error {
	var cfg config.Config
	unmarshalErr := toml.Unmarshal(data, &cfg)
	if unmarshalErr != nil {
		return fmt.Errorf("%w: refusing to save, value at '%s' breaks the config, run migrate to repair: %w", ErrConfigUnmarshal, configPath, unmarshalErr)
	}
	return nil
}
