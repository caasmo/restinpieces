package core

import (
	"strconv"
	"strings"
	"time"

	"github.com/caasmo/restinpieces/cache"
	"github.com/caasmo/restinpieces/config"
)

// FixedWindowLimiter keeps its counters in a cache. The label selects the
// config entry, so one object can serve every fixed-window label. The cache
// key is the entry prefix, the identity and the current window number, so a
// new window starts every identity at zero.
type FixedWindowLimiter struct {
	cache   *cache.Default[string, any]
	entries map[string]config.FixedWindowEntry
}

// NewFixedWindowLimiter returns a fixed-window limiter for entries.
func NewFixedWindowLimiter(entries config.FixedWindow, counterCache *cache.Default[string, any]) *FixedWindowLimiter {
	return &FixedWindowLimiter{
		cache:   counterCache,
		entries: entries,
	}
}

// Limit returns true if identity already used Max calls in the current window,
// false otherwise. Every call is counted, including refused ones. A label the
// config does not define returns true.
func (f *FixedWindowLimiter) Limit(label, identity string) bool {
	entry, defined := f.entries[label]
	if !defined {
		return true
	}

	bucket := time.Now().UnixNano() / entry.Window.Nanoseconds()
	key := fixedWindowKey(entry.Prefix, identity, bucket)

	count, err := f.cache.Increment(key, 1)
	if err != nil {
		return true
	}

	return count > uint64(entry.Max)
}

// fixedWindowKey joins the prefix, identity and window number. The underscore
// keeps the identity from running into the window number: identity "user1"
// with window 23 must not share a key with identity "user12" with window 3.
// For example, fixedWindowKey("pr_", "user@example.com", 493827) is
// "pr_user@example.com_493827".
//
// The digits fit a 20-byte array (19 digits plus a sign for any int64), and
// Grow sizes the builder to the full length, so the whole key is one
// allocation that the cache keeps.
func fixedWindowKey(prefix, identity string, bucket int64) string {
	var digits [20]byte
	number := strconv.AppendInt(digits[:0], bucket, 10)

	var key strings.Builder
	key.Grow(len(prefix) + len(identity) + len(number) + 1)
	key.WriteString(prefix)
	key.WriteString(identity)
	key.WriteByte('_')
	_, _ = key.Write(number)

	return key.String()
}
