// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package cluster

import (
	"fmt"
	"sync"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

const eventQueueSize = 4096

// dispatcher hands received cluster messages to the registered handlers.
// Each event type has its own queue and goroutine: messages of a given type
// are handled in the order they were received, and a slow handler (e.g.
// plugin installation) doesn't delay the others.
type dispatcher struct {
	c       *Cluster
	mu      sync.Mutex
	queues  map[model.ClusterEvent]chan *model.ClusterMessage
	quit    chan struct{}
	stopped bool
	wg      sync.WaitGroup
}

func newDispatcher(c *Cluster) *dispatcher {
	return &dispatcher{
		c:      c,
		queues: make(map[model.ClusterEvent]chan *model.ClusterMessage),
		quit:   make(chan struct{}),
	}
}

func (d *dispatcher) queue(event model.ClusterEvent) chan *model.ClusterMessage {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped {
		return nil
	}
	q, ok := d.queues[event]
	if !ok {
		q = make(chan *model.ClusterMessage, eventQueueSize)
		d.queues[event] = q
		d.wg.Add(1)
		go d.run(q)
	}
	return q
}

func (d *dispatcher) dispatch(msg *model.ClusterMessage, bestEffort bool) {
	if d.c.handler(msg.Event) == nil {
		d.c.logger().Debug("Cluster: no handler registered for event", mlog.String("event", string(msg.Event)))
		return
	}
	q := d.queue(msg.Event)
	if q == nil {
		return
	}
	if bestEffort {
		// Never block memberlist's packet handler.
		select {
		case q <- msg:
		default:
			d.c.logger().Warn("Cluster: dropping best-effort message, handler queue is full", msg.LogFields()...)
		}
		return
	}
	// Reliable messages apply back-pressure on the sending node's stream.
	select {
	case q <- msg:
	case <-d.quit:
	}
}

func (d *dispatcher) run(q chan *model.ClusterMessage) {
	defer d.wg.Done()
	for {
		select {
		case <-d.quit:
			return
		case msg := <-q:
			d.handle(msg)
		}
	}
}

func (d *dispatcher) handle(msg *model.ClusterMessage) {
	h := d.c.handler(msg.Event)
	if h == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			d.c.logger().Error("Cluster: message handler panicked", append(msg.LogFields(), mlog.String("panic", fmt.Sprint(r)))...)
		}
	}()
	h(msg)
}

func (d *dispatcher) stop() {
	d.mu.Lock()
	if d.stopped {
		d.mu.Unlock()
		return
	}
	d.stopped = true
	close(d.quit)
	d.mu.Unlock()
	d.wg.Wait()
}
