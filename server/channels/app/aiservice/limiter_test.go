// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package aiservice

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLimiter(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	l := NewLimiter()
	l.SetClock(func() time.Time { return now })

	for range 3 {
		ok, _ := l.Allow("conn", 3)
		assert.True(t, ok)
	}
	ok, wait := l.Allow("conn", 3)
	assert.False(t, ok, "a minute's worth, then nothing")
	assert.InDelta(t, 20*time.Second, wait, float64(time.Second))

	ok, _ = l.Allow("other", 3)
	assert.True(t, ok, "each key has its own bucket")

	now = now.Add(20 * time.Second)
	ok, _ = l.Allow("conn", 3)
	assert.True(t, ok, "refilled at the rate")

	ok, _ = l.Allow("unlimited", 0)
	assert.True(t, ok)
}

func TestLimiterAllowAll(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	l := NewLimiter()
	l.SetClock(func() time.Time { return now })

	ok, _ := l.AllowAll(map[string]int{"conn": 10, "conn:user": 1})
	assert.True(t, ok)
	ok, _ = l.AllowAll(map[string]int{"conn": 10, "conn:user": 1})
	assert.False(t, ok, "the person's bucket is empty")

	// The refused call took nothing from the connection's bucket: nine left.
	for range 9 {
		ok, _ = l.Allow("conn", 10)
		assert.True(t, ok)
	}
	ok, _ = l.Allow("conn", 10)
	assert.False(t, ok)
}
