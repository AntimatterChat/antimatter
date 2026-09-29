// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

const (
	opIndex  = "index"
	opDelete = "delete"

	// maxBulkBodySize bounds the size of a single bulk request.
	maxBulkBodySize = 8 * 1024 * 1024

	bulkRetries      = 3
	bulkRetryBackoff = 500 * time.Millisecond

	liveFlushInterval = 5 * time.Second
)

// BulkAction is a single write of a bulk request.
type BulkAction struct {
	Op    string
	Index string
	ID    string
	Doc   any
}

// BulkFailure describes a write rejected by the search backend.
type BulkFailure struct {
	Index  string
	ID     string
	Status int
	Reason string
}

type bulkResponse struct {
	Errors bool                          `json:"errors"`
	Items  []map[string]bulkResponseItem `json:"items"`
}

type bulkResponseItem struct {
	Index  string `json:"_index"`
	ID     string `json:"_id"`
	Status int    `json:"status"`
	Error  *struct {
		Type   string `json:"type"`
		Reason string `json:"reason"`
	} `json:"error"`
}

func encodeBulkAction(buf *bytes.Buffer, a BulkAction) error {
	meta := map[string]map[string]string{a.Op: {"_index": a.Index, "_id": a.ID}}
	line, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	buf.Write(line)
	buf.WriteByte('\n')
	if a.Op == opIndex {
		doc, err := json.Marshal(a.Doc)
		if err != nil {
			return fmt.Errorf("failed to encode document %s: %w", a.ID, err)
		}
		buf.Write(doc)
		buf.WriteByte('\n')
	}
	return nil
}

// sendBulk sends the actions in as many requests as needed. Rejections caused
// by back pressure (HTTP 429) are retried; other per document failures are
// returned. A non nil error means the request(s) could not be performed.
func (e *Engine) sendBulk(ctx context.Context, c *client, actions []BulkAction) ([]BulkFailure, error) {
	if len(actions) == 0 {
		return nil, nil
	}

	// Make sure every target index exists with the right mappings, even when
	// automatic index creation is disabled on the cluster.
	seen := map[string]bool{}
	for _, a := range actions {
		if a.Op == opIndex && !seen[a.Index] {
			seen[a.Index] = true
			if err := e.ensureIndex(ctx, c, a.Index); err != nil {
				return nil, fmt.Errorf("failed to create index %s: %w", a.Index, err)
			}
		}
	}

	var failures []BulkFailure
	pending := actions
	for attempt := 0; len(pending) > 0; attempt++ {
		var retry []BulkAction
		var buf bytes.Buffer
		var chunk []BulkAction

		sendChunk := func() error {
			if len(chunk) == 0 {
				return nil
			}
			f, r, err := c.bulkRequest(ctx, buf.Bytes(), chunk)
			if err != nil {
				return err
			}
			failures = append(failures, f...)
			retry = append(retry, r...)
			buf.Reset()
			chunk = chunk[:0]
			return nil
		}

		for _, a := range pending {
			if err := encodeBulkAction(&buf, a); err != nil {
				failures = append(failures, BulkFailure{Index: a.Index, ID: a.ID, Reason: err.Error()})
				continue
			}
			chunk = append(chunk, a)
			if buf.Len() >= maxBulkBodySize {
				if err := sendChunk(); err != nil {
					return failures, err
				}
			}
		}
		if err := sendChunk(); err != nil {
			return failures, err
		}

		if len(retry) == 0 {
			break
		}
		if attempt >= bulkRetries {
			for _, a := range retry {
				failures = append(failures, BulkFailure{Index: a.Index, ID: a.ID, Status: http.StatusTooManyRequests, Reason: "rejected by the search backend (too many requests)"})
			}
			break
		}
		select {
		case <-ctx.Done():
			return failures, ctx.Err()
		case <-time.After(bulkRetryBackoff * time.Duration(attempt+1)):
		}
		pending = retry
	}
	return failures, nil
}

// bulkRequest sends one bulk body and splits the outcome in failures and
// actions to retry.
func (c *client) bulkRequest(ctx context.Context, body []byte, actions []BulkAction) (failures []BulkFailure, retry []BulkAction, err error) {
	data, err := c.do(ctx, http.MethodPost, "/_bulk", nil, body)
	if err != nil {
		return nil, nil, err
	}
	var res bulkResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, nil, fmt.Errorf("failed to decode bulk response: %w", err)
	}
	if !res.Errors {
		return nil, nil, nil
	}
	for i, item := range res.Items {
		for op, r := range item {
			if r.Status < 300 || (op == opDelete && r.Status == http.StatusNotFound) {
				continue
			}
			if r.Status == http.StatusTooManyRequests && i < len(actions) {
				retry = append(retry, actions[i])
				continue
			}
			reason := ""
			if r.Error != nil {
				reason = r.Error.Type + ": " + r.Error.Reason
			}
			failures = append(failures, BulkFailure{Index: r.Index, ID: r.ID, Status: r.Status, Reason: reason})
		}
	}
	return failures, retry, nil
}

// bulkProcessor buffers live index writes and sends them in batches of
// LiveIndexingBatchSize, or at least every liveFlushInterval.
type bulkProcessor struct {
	engine *Engine
	client *client

	mu        sync.Mutex
	pending   []BulkAction
	batchSize int

	flushMu sync.Mutex
	flushCh chan struct{}
	stopCh  chan struct{}
	doneCh  chan struct{}
}

func newBulkProcessor(e *Engine, c *client, batchSize int) *bulkProcessor {
	p := &bulkProcessor{
		engine:    e,
		client:    c,
		batchSize: batchSize,
		flushCh:   make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
	go p.loop()
	return p
}

func (p *bulkProcessor) setBatchSize(size int) {
	p.mu.Lock()
	p.batchSize = size
	p.mu.Unlock()
}

func (p *bulkProcessor) add(a BulkAction) {
	p.mu.Lock()
	p.pending = append(p.pending, a)
	n, size := len(p.pending), p.batchSize
	p.mu.Unlock()

	switch {
	case n >= max(size*20, 2000):
		// The backend doesn't keep up: apply back pressure on the caller.
		p.flush()
	case n >= size:
		select {
		case p.flushCh <- struct{}{}:
		default:
		}
	}
}

func (p *bulkProcessor) loop() {
	defer close(p.doneCh)
	ticker := time.NewTicker(liveFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-p.stopCh:
			p.flush()
			return
		case <-ticker.C:
			p.flush()
		case <-p.flushCh:
			p.flush()
		}
	}
}

// flush synchronously sends everything buffered so far.
func (p *bulkProcessor) flush() {
	p.flushMu.Lock()
	defer p.flushMu.Unlock()

	p.mu.Lock()
	actions := p.pending
	p.pending = nil
	p.mu.Unlock()
	if len(actions) == 0 {
		return
	}

	// Bounded, so that stopping an engine whose backend is unreachable
	// doesn't hang.
	ctx, cancel := context.WithTimeout(context.Background(), max(2*p.client.timeout, 10*time.Second))
	defer cancel()
	failures, err := p.engine.sendBulk(ctx, p.client, actions)
	logger := p.engine.logger()
	if err != nil {
		logger.Warn("Failed to send a batch of live index updates to the search backend", mlog.Int("count", len(actions)), mlog.Err(err))
		return
	}
	for _, f := range failures {
		logger.Warn("Search backend rejected a live index update", mlog.String("index", f.Index), mlog.String("id", f.ID), mlog.Int("status", f.Status), mlog.String("reason", f.Reason))
	}
}

func (p *bulkProcessor) stop() {
	close(p.stopCh)
	<-p.doneCh
}
