// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package cluster

import (
	"encoding/binary"
	"sort"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// Every reliable send opens a new memberlist stream, and the receiving node
// handles streams concurrently. To preserve the order of reliable messages,
// each peerSender numbers what it sends: (sender id, epoch, sequence). The
// receiver delivers payloads in sequence order and drops duplicates (a
// retried send which had in fact been received). When a send definitively
// fails the sender starts a new epoch, so the receiver never waits for a
// sequence number which will not come.

const (
	kindSequenced byte = 6

	// A missing sequence number is given up on after this delay.
	sequenceGapTimeout = 10 * time.Second
	maxBufferedPerPeer = 4096
)

func encodeSequenced(from string, epoch, seq uint64, inner []byte) []byte {
	buf := newEnvelope(kindSequenced, 0, len(inner)+len(from)+24)
	buf = appendString(buf, from)
	buf = binary.AppendUvarint(buf, epoch)
	buf = binary.AppendUvarint(buf, seq)
	return appendBytes(buf, inner)
}

func decodeSequenced(body []byte) (from string, epoch, seq uint64, inner []byte, err error) {
	r := &reader{buf: body}
	from = r.string()
	epoch = r.uvarint()
	seq = r.uvarint()
	inner = r.bytes()
	return from, epoch, seq, inner, r.err
}

type inboundStream struct {
	mu       sync.Mutex
	epoch    uint64
	next     uint64
	buffered map[uint64][]byte
	since    time.Time // when the oldest buffered payload arrived
}

type sequencer struct {
	c       *Cluster
	mu      sync.Mutex
	streams map[string]*inboundStream
}

func newSequencer(c *Cluster) *sequencer {
	return &sequencer{c: c, streams: make(map[string]*inboundStream)}
}

func (s *sequencer) stream(from string) *inboundStream {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.streams[from]
	if !ok {
		st = &inboundStream{buffered: make(map[uint64][]byte)}
		s.streams[from] = st
	}
	return st
}

func (s *sequencer) forget(from string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.streams, from)
}

func (s *sequencer) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.streams = make(map[string]*inboundStream)
}

func (s *sequencer) deliver(inner []byte) {
	s.c.handlePayload(inner, 1)
}

// flush delivers every buffered payload in order. Must hold st.mu.
func (s *sequencer) flush(st *inboundStream) {
	if len(st.buffered) == 0 {
		return
	}
	seqs := make([]uint64, 0, len(st.buffered))
	for seq := range st.buffered {
		seqs = append(seqs, seq)
	}
	sort.Slice(seqs, func(i, j int) bool { return seqs[i] < seqs[j] })
	for _, seq := range seqs {
		s.deliver(st.buffered[seq])
		delete(st.buffered, seq)
		st.next = seq + 1
	}
}

func (s *sequencer) handle(body []byte) {
	from, epoch, seq, inner, err := decodeSequenced(body)
	if err != nil {
		s.c.logger().Warn("Cluster: discarding malformed sequenced message", mlog.Err(err))
		return
	}

	st := s.stream(from)
	st.mu.Lock()
	defer st.mu.Unlock()

	switch {
	case epoch > st.epoch:
		// The sender started over: whatever is still buffered won't be
		// completed.
		s.flush(st)
		st.epoch = epoch
		st.next = 0
	case epoch < st.epoch:
		// Late payload from an abandoned epoch.
		s.deliver(inner)
		return
	}

	if seq < st.next {
		return // duplicate
	}
	if seq > st.next {
		if _, ok := st.buffered[seq]; !ok {
			if len(st.buffered) == 0 {
				st.since = time.Now()
			}
			st.buffered[seq] = inner
		}
		if len(st.buffered) > maxBufferedPerPeer {
			s.c.logger().Warn("Cluster: too many out of order messages, skipping missing ones", mlog.String("node_id", from))
			s.flush(st)
		}
		return
	}

	s.deliver(inner)
	st.next++
	for {
		next, ok := st.buffered[st.next]
		if !ok {
			break
		}
		delete(st.buffered, st.next)
		s.deliver(next)
		st.next++
	}
	if len(st.buffered) > 0 {
		st.since = time.Now()
	}
}

// skipGaps delivers the payloads stuck behind a sequence number which never
// arrived.
func (s *sequencer) skipGaps(now time.Time) {
	s.mu.Lock()
	streams := make(map[string]*inboundStream, len(s.streams))
	for k, v := range s.streams {
		streams[k] = v
	}
	s.mu.Unlock()

	for from, st := range streams {
		st.mu.Lock()
		if len(st.buffered) > 0 && now.Sub(st.since) > sequenceGapTimeout {
			s.c.logger().Warn("Cluster: gave up waiting for a missing message", mlog.String("node_id", from), mlog.Uint("expected_seq", st.next))
			s.flush(st)
		}
		st.mu.Unlock()
	}
}
