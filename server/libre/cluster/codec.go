// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package cluster

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
)

// Every payload exchanged between nodes (on top of memberlist user messages)
// starts with a three byte header:
//
//	[magic] [kind] [flags]
//
// followed by a kind specific body.
const (
	envelopeMagic      byte = 0xB7
	envelopeHeaderSize      = 3

	kindMessage  byte = 1 // a model.ClusterMessage
	kindBatch    byte = 2 // several envelopes sent at once
	kindFragment byte = 3 // a part of an envelope too large to be sent at once
	kindRequest  byte = 4 // an RPC request
	kindResponse byte = 5 // an RPC response

	flagBestEffort byte = 1 << 0
)

var errMalformed = errors.New("malformed cluster payload")

func newEnvelope(kind, flags byte, bodySize int) []byte {
	buf := make([]byte, envelopeHeaderSize, envelopeHeaderSize+bodySize)
	buf[0] = envelopeMagic
	buf[1] = kind
	buf[2] = flags
	return buf
}

func parseEnvelope(buf []byte) (kind, flags byte, body []byte, err error) {
	if len(buf) < envelopeHeaderSize || buf[0] != envelopeMagic {
		return 0, 0, nil, errMalformed
	}
	return buf[1], buf[2], buf[envelopeHeaderSize:], nil
}

func appendBytes(buf, b []byte) []byte {
	buf = binary.AppendUvarint(buf, uint64(len(b)))
	return append(buf, b...)
}

func appendString(buf []byte, s string) []byte {
	buf = binary.AppendUvarint(buf, uint64(len(s)))
	return append(buf, s...)
}

type reader struct {
	buf []byte
	err error
}

func (r *reader) uvarint() uint64 {
	if r.err != nil {
		return 0
	}
	v, n := binary.Uvarint(r.buf)
	if n <= 0 {
		r.err = errMalformed
		return 0
	}
	r.buf = r.buf[n:]
	return v
}

func (r *reader) bytes() []byte {
	l := r.uvarint()
	if r.err != nil {
		return nil
	}
	if l > uint64(len(r.buf)) {
		r.err = errMalformed
		return nil
	}
	b := r.buf[:l:l]
	r.buf = r.buf[l:]
	return b
}

func (r *reader) string() string {
	return string(r.bytes())
}

// encodeClusterMessage serializes msg into an envelope. SendType and
// WaitForAllToSend are local sending instructions and are not transmitted.
func encodeClusterMessage(msg *model.ClusterMessage, flags byte) []byte {
	size := len(msg.Event) + len(msg.Data) + 16
	for k, v := range msg.Props {
		size += len(k) + len(v) + 4
	}
	buf := newEnvelope(kindMessage, flags, size)
	buf = appendString(buf, string(msg.Event))
	buf = binary.AppendUvarint(buf, uint64(len(msg.Props)))
	for k, v := range msg.Props {
		buf = appendString(buf, k)
		buf = appendString(buf, v)
	}
	buf = appendBytes(buf, msg.Data)
	return buf
}

func decodeClusterMessage(body []byte) (*model.ClusterMessage, error) {
	r := &reader{buf: body}
	msg := &model.ClusterMessage{Event: model.ClusterEvent(r.string())}
	nProps := r.uvarint()
	if r.err == nil && nProps > uint64(len(r.buf)) {
		return nil, errMalformed
	}
	if nProps > 0 {
		msg.Props = make(map[string]string, nProps)
		for i := uint64(0); i < nProps && r.err == nil; i++ {
			k := r.string()
			msg.Props[k] = r.string()
		}
	}
	data := r.bytes()
	if r.err != nil {
		return nil, r.err
	}
	if len(data) > 0 {
		msg.Data = data
	}
	return msg, nil
}

func encodeBatch(items [][]byte) []byte {
	size := 8
	for _, it := range items {
		size += len(it) + binary.MaxVarintLen64
	}
	buf := newEnvelope(kindBatch, 0, size)
	buf = binary.AppendUvarint(buf, uint64(len(items)))
	for _, it := range items {
		buf = appendBytes(buf, it)
	}
	return buf
}

func decodeBatch(body []byte) ([][]byte, error) {
	r := &reader{buf: body}
	n := r.uvarint()
	if r.err != nil {
		return nil, r.err
	}
	if n > uint64(len(r.buf)) {
		return nil, errMalformed
	}
	items := make([][]byte, 0, n)
	for i := uint64(0); i < n; i++ {
		it := r.bytes()
		if r.err != nil {
			return nil, r.err
		}
		items = append(items, it)
	}
	return items, nil
}

type fragment struct {
	id    string
	index int
	total int
	data  []byte
}

// splitIntoFragments splits an envelope into fragment envelopes of at most
// chunkSize bytes of payload each.
func splitIntoFragments(payload []byte, chunkSize int) [][]byte {
	id := model.NewId()
	total := (len(payload) + chunkSize - 1) / chunkSize
	out := make([][]byte, 0, total)
	for i := range total {
		end := min((i+1)*chunkSize, len(payload))
		chunk := payload[i*chunkSize : end]
		buf := newEnvelope(kindFragment, 0, len(chunk)+len(id)+24)
		buf = appendString(buf, id)
		buf = binary.AppendUvarint(buf, uint64(i))
		buf = binary.AppendUvarint(buf, uint64(total))
		buf = appendBytes(buf, chunk)
		out = append(out, buf)
	}
	return out
}

func decodeFragment(body []byte) (*fragment, error) {
	r := &reader{buf: body}
	f := &fragment{id: r.string()}
	index := r.uvarint()
	total := r.uvarint()
	f.data = r.bytes()
	if r.err != nil {
		return nil, r.err
	}
	if total == 0 || total > maxFragments || index >= total || f.id == "" {
		return nil, fmt.Errorf("%w: invalid fragment %d/%d", errMalformed, index, total)
	}
	f.index = int(index)
	f.total = int(total)
	return f, nil
}
