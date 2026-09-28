package main

import (
	"fmt"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml/v2"
)

// validateTomlAsConfig tries to unmarshal the updated TOML into config.Config.
// A bad type swap (like a TOML table turned into a string) fails here, before
// anything reaches the database.
func validateTomlAsConfig(data []byte, configPath string) error {
	var cfg config.Config
	unmarshalErr := toml.Unmarshal(data, &cfg)
	if unmarshalErr != nil {
		return fmt.Errorf("%w: refusing to save, value at '%s' breaks the config, run migrate to repair: %w", ErrConfigUnmarshal, configPath, unmarshalErr)
	}
	return nil
}
