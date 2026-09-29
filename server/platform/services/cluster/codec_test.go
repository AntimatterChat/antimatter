// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package cluster

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestClusterMessageCodec(t *testing.T) {
	msg := &model.ClusterMessage{
		Event:    model.ClusterEventPluginEvent,
		SendType: model.ClusterSendReliable,
		Data:     []byte("some data"),
		Props:    map[string]string{"PluginID": "p", "EventID": "e"},
	}
	buf := encodeClusterMessage(msg, flagBestEffort)
	kind, flags, body, err := parseEnvelope(buf)
	require.NoError(t, err)
	assert.Equal(t, kindMessage, kind)
	assert.Equal(t, flagBestEffort, flags)

	got, err := decodeClusterMessage(body)
	require.NoError(t, err)
	assert.Equal(t, msg.Event, got.Event)
	assert.Equal(t, msg.Data, got.Data)
	assert.Equal(t, msg.Props, got.Props)
	assert.Empty(t, got.SendType)

	empty, err := decodeClusterMessage(encodeClusterMessage(&model.ClusterMessage{Event: "x"}, 0)[envelopeHeaderSize:])
	require.NoError(t, err)
	assert.Nil(t, empty.Data)
	assert.Nil(t, empty.Props)

	for i := range body {
		// Truncated bodies must be rejected, never panic.
		_, _ = decodeClusterMessage(body[:i])
	}
	_, _, _, err = parseEnvelope([]byte{1, 2, 3})
	assert.Error(t, err)
}

func TestBatchCodec(t *testing.T) {
	items := [][]byte{[]byte("a"), {}, bytes.Repeat([]byte("b"), 1000)}
	kind, _, body, err := parseEnvelope(encodeBatch(items))
	require.NoError(t, err)
	assert.Equal(t, kindBatch, kind)
	got, err := decodeBatch(body)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, items[2], got[2])
	assert.Empty(t, got[1])
}

func TestFragments(t *testing.T) {
	payload := make([]byte, 10*1024+7)
	_, err := rand.Read(payload)
	require.NoError(t, err)

	frags := splitIntoFragments(payload, 1024)
	require.Len(t, frags, 11)

	r := newReassembler(nil)
	var full []byte
	// Deliver out of order, with a duplicate.
	order := []int{3, 0, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	for i, idx := range order {
		kind, _, body, err := parseEnvelope(frags[idx])
		require.NoError(t, err)
		require.Equal(t, kindFragment, kind)
		f, err := decodeFragment(body)
		require.NoError(t, err)
		full = r.add(f)
		if i < len(order)-1 {
			require.Nil(t, full)
		}
	}
	assert.Equal(t, payload, full)
	assert.Empty(t, r.pending)
}

func TestDecodeEncryptionKey(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 16)
	assert.Equal(t, key, decodeEncryptionKey(base64.StdEncoding.EncodeToString(key)))
	// Anything else still yields a stable, valid AES-256 key.
	k1 := decodeEncryptionKey("not a key")
	assert.Len(t, k1, 32)
	assert.Equal(t, k1, decodeEncryptionKey("not a key"))
}

func TestClusterLabel(t *testing.T) {
	assert.Equal(t, labelPrefix+"prod", clusterLabel("prod"))
	long := clusterLabel(string(bytes.Repeat([]byte("x"), 400)))
	assert.LessOrEqual(t, len(long), 255)
}

func TestConfigHashIgnoresNodeSettings(t *testing.T) {
	cfg := &model.Config{}
	cfg.SetDefaults()
	other := cfg.Clone()
	*other.ClusterSettings.OverrideHostname = "node2"
	*other.ClusterSettings.AdvertiseAddress = "10.0.0.2"
	assert.Equal(t, configHash(cfg), configHash(other))
	*other.ServiceSettings.SiteURL = "https://example.com"
	assert.NotEqual(t, configHash(cfg), configHash(other))
}
