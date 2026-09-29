// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package saml

import (
	"sync"
	"time"
)

const (
	defaultReplayCacheSize = 100000
	// replayFallbackTTL is how long an assertion ID is remembered when the
	// assertion carries no usable NotOnOrAfter.
	replayFallbackTTL = time.Hour
)

// replayCache remembers the IDs of the assertions consumed by this node until
// they expire, so that a captured SAMLResponse cannot be posted a second time.
// It is local to the node: in a cluster a response could still be replayed
// once against each other node within its validity window.
type replayCache struct {
	mu      sync.Mutex
	entries map[string]time.Time
	maxSize int
}

func newReplayCache(maxSize int) *replayCache {
	return &replayCache{entries: make(map[string]time.Time), maxSize: maxSize}
}

// markUsed records id as consumed until expiry and reports whether it had
// already been consumed.
func (c *replayCache) markUsed(id string, expiry, now time.Time) (alreadyUsed bool) {
	if id == "" {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if exp, ok := c.entries[id]; ok && now.Before(exp) {
		return true
	}

	if len(c.entries) >= c.maxSize {
		c.pruneLocked(now)
		if len(c.entries) >= c.maxSize {
			// Still full of live entries: evict arbitrary ones to bound memory.
			for k := range c.entries {
				delete(c.entries, k)
				if len(c.entries) < c.maxSize/2 {
					break
				}
			}
		}
	}

	c.entries[id] = expiry
	return false
}

func (c *replayCache) pruneLocked(now time.Time) {
	for k, exp := range c.entries {
		if !now.Before(exp) {
			delete(c.entries, k)
		}
	}
}
