// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package aiservice

import (
	"sync"
	"time"
)

// Limiter is a set of token buckets, one per key (a connection, or a connection and a person), each refilled at its
// rate of requests per minute and holding at most a minute's worth. It is per server node: a cluster of n nodes lets
// through up to n times the rate, while the monthly budgets, counted in the database, hold for the whole cluster.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	now     func() time.Time
	calls   int
}

type bucket struct {
	tokens float64
	rate   int
	last   time.Time
}

// NewLimiter makes an empty limiter.
func NewLimiter() *Limiter {
	return &Limiter{buckets: map[string]*bucket{}, now: time.Now}
}

// SetClock replaces the clock, for tests.
func (l *Limiter) SetClock(now func() time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.now = now
}

// Allow takes a request from the bucket of key, allowed perMinute requests a minute (0 or less: no limit). When the
// bucket is empty it tells how long until the next request would be allowed.
func (l *Limiter) Allow(key string, perMinute int) (bool, time.Duration) {
	return l.AllowAll(map[string]int{key: perMinute})
}

// AllowAll takes a request from each bucket, or from none when one of them is empty (then it tells how long until
// they all have one).
func (l *Limiter) AllowAll(limits map[string]int) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.calls++
	if l.calls%1024 == 0 {
		l.prune(now)
	}
	var wait time.Duration
	ok := true
	active := make([]*bucket, 0, len(limits))
	for key, perMinute := range limits {
		if perMinute <= 0 {
			continue
		}
		b := l.buckets[key]
		if b == nil || b.rate != perMinute {
			b = &bucket{tokens: float64(perMinute), rate: perMinute, last: now}
			l.buckets[key] = b
		}
		perSecond := float64(perMinute) / 60
		b.tokens = min(float64(perMinute), b.tokens+now.Sub(b.last).Seconds()*perSecond)
		b.last = now
		if b.tokens < 1 {
			ok = false
			wait = max(wait, time.Duration((1-b.tokens)/perSecond*float64(time.Second)))
		}
		active = append(active, b)
	}
	if !ok {
		return false, wait
	}
	for _, b := range active {
		b.tokens--
	}
	return true, 0
}

// prune forgets the buckets that are full again: they behave as new ones.
func (l *Limiter) prune(now time.Time) {
	for key, b := range l.buckets {
		if now.Sub(b.last) > time.Minute {
			delete(l.buckets, key)
		}
	}
}
