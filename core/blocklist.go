package core

import (
	"sync"
)

// BlockList maintains a thread-safe list of blocked IPs
type BlockList struct {
	ips sync.Map // map[string]struct{} - empty struct uses 0 memory
}

// NewBlockList creates a new BlockList instance
func NewBlockList() *BlockList {
	return &BlockList{}
}

// Add adds an IP to the blocklist
func (bl *BlockList) Add(ip string) {
	bl.ips.Store(ip, struct{}{})
}

// Remove removes an IP from the blocklist
func (bl *BlockList) Remove(ip string) {
	bl.ips.Delete(ip)
}

// Contains checks if an IP is in the blocklist
func (bl *BlockList) Contains(ip string) bool {
	_, exists := bl.ips.Load(ip)
	return exists
}
