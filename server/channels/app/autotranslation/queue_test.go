// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package autotranslation

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

func (p *workerPool) workers() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.stops)
}

// Overlapping resizes (two quick saves of the settings) end with the last count asked for.
func TestWorkerPoolConcurrentResize(t *testing.T) {
	p := newWorkerPool(func(*task) {}, nil)
	p.start(4)
	t.Cleanup(p.stop)

	for round := range 20 {
		var wg sync.WaitGroup
		sizes := []int{8, 2, 6, 3 + round%4}
		for _, n := range sizes {
			p.setSize(n) // in the order the changes came
			wg.Go(p.applySize)
		}
		wg.Wait()
		require.Equal(t, sizes[len(sizes)-1], p.workers(), "round %d", round)
	}

	p.resize(5)
	assert.Equal(t, 5, p.workers())
}

// The background resize recovers from a panic instead of stopping the server.
func TestWorkerPoolResizeRecoversPanic(t *testing.T) {
	env := newTestEnv(t, providerLibreTranslate, "http://lt.example.com")
	logger, err := mlog.NewLogger()
	require.NoError(t, err)
	buffer := &mlog.Buffer{}
	require.NoError(t, mlog.AddWriterTarget(logger, buffer, true, mlog.StdAll...))
	t.Cleanup(func() { _ = logger.Shutdown() })
	env.h.logger = logger

	ran := make(chan struct{})
	env.svc.goRecovered("resizing the workers", func() {
		defer close(ran)
		panic("boom")
	})
	select {
	case <-ran:
	case <-time.After(10 * time.Second):
		require.Fail(t, "the background function never ran")
	}
	require.Eventually(t, func() bool {
		require.NoError(t, logger.Flush())
		return strings.Contains(buffer.String(), "Auto-translation panicked while resizing the workers")
	}, 10*time.Second, 50*time.Millisecond)
}
