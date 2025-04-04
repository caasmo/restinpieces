package proxy

import (
	"github.com/caasmo/restinpieces/config"
)

// BlockIp implements the FeatureBlocker interface using configuration settings.
type BlockIp struct {
	config *config.Config
}

// DisabledBlock implements the FeatureBlocker interface but always returns false,
// effectively disabling the blocking feature.
type DisabledBlock struct{}

// IsEnabled always returns false, indicating the feature is disabled.
func (d *DisabledBlock) IsEnabled() bool {
	return false
}

// IsBlocked always returns false, indicating no IP is ever blocked.
func (d *DisabledBlock) IsBlocked(ip string) bool {
	return false
}
