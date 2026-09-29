// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package cluster

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// registerDiscovery records this node in the ClusterDiscovery table, which
// other nodes read to find the members of the cluster.
func (c *Cluster) registerDiscovery() {
	cds := c.host.ClusterDiscoveryStore()
	if err := cds.Cleanup(); err != nil {
		c.logger().Warn("Cluster: failed to clean up outdated cluster discovery entries", mlog.Err(err))
	}

	exists, err := cds.Exists(c.discovery)
	if err != nil {
		c.logger().Warn("Cluster: failed to check for an existing cluster discovery entry", mlog.Err(err))
	} else if exists {
		if _, err := cds.Delete(c.discovery); err != nil {
			c.logger().Warn("Cluster: failed to remove the previous cluster discovery entry", mlog.Err(err))
		}
	}

	if err := cds.Save(c.discovery); err != nil {
		c.logger().Error("Cluster: failed to save the cluster discovery entry, other nodes may not find this node", mlog.Err(err))
	}
}

func (c *Cluster) unregisterDiscovery() {
	if c.discovery == nil {
		return
	}
	if _, err := c.host.ClusterDiscoveryStore().Delete(c.discovery); err != nil {
		c.logger().Warn("Cluster: failed to remove the cluster discovery entry", mlog.Err(err))
	}
}

// pingDiscovery keeps this node's discovery entry alive, re-creating it if
// it disappeared (for instance cleaned up after a long database outage).
func (c *Cluster) pingDiscovery() {
	cds := c.host.ClusterDiscoveryStore()
	exists, err := cds.Exists(c.discovery)
	if err != nil {
		c.logger().Warn("Cluster: failed to check the cluster discovery entry", mlog.Err(err))
		return
	}
	if !exists {
		c.logger().Info("Cluster: cluster discovery entry missing, saving it again")
		if err := cds.Save(c.discovery); err != nil {
			c.logger().Error("Cluster: failed to save the cluster discovery entry", mlog.Err(err))
		}
		return
	}
	if err := cds.SetLastPingAt(c.discovery); err != nil {
		c.logger().Error("Cluster: failed to update the cluster discovery entry", mlog.Err(err))
	}
}

func (c *Cluster) discoveryLoop() {
	defer c.wg.Done()
	ticker := time.NewTicker(c.discoveryPing)
	defer ticker.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.pingDiscovery()
		}
	}
}

func (c *Cluster) joinLoop() {
	defer c.wg.Done()
	for {
		interval := c.joinInterval
		if len(c.peers()) == 0 {
			interval = c.joinIntervalLone
		}
		select {
		case <-c.stopCh:
			return
		case <-time.After(interval):
			c.joinFromDiscovery(false)
		}
	}
}

// joinFromDiscovery contacts the nodes registered in the discovery table
// which are not known members yet. Gossip then spreads the full membership.
func (c *Cluster) joinFromDiscovery(initial bool) {
	list := c.memberlist()
	if list == nil {
		return
	}
	entries, err := c.host.ClusterDiscoveryStore().GetAll(model.CDSTypeApp, c.clusterName)
	if err != nil {
		c.logger().Warn("Cluster: failed to read the cluster discovery entries", mlog.Err(err))
		return
	}

	known := map[string]bool{c.advertise: true}
	for _, p := range c.peers() {
		known[p.node.Address()] = true
	}

	var addrs []string
	for _, e := range entries {
		if e.Id == c.discovery.Id || (e.Hostname == c.discovery.Hostname && e.GossipPort == c.discovery.GossipPort) {
			continue
		}
		addr := net.JoinHostPort(e.Hostname, strconv.Itoa(int(e.GossipPort)))
		if known[addr] {
			continue
		}
		addrs = append(addrs, addr)
	}
	if len(addrs) == 0 {
		return
	}

	// Contact every address concurrently: unreachable, stale entries must
	// not delay joining the live ones.
	var wg sync.WaitGroup
	var mu sync.Mutex
	joined := 0
	for _, addr := range addrs {
		wg.Go(func() {
			if _, err := list.Join([]string{addr}); err != nil {
				c.logger().Debug("Cluster: failed to contact node", mlog.String("address", addr), mlog.Err(err))
				return
			}
			mu.Lock()
			joined++
			mu.Unlock()
		})
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	timeout := time.After(initialJoinTimeout)
	select {
	case <-done:
	case <-timeout:
	case <-c.stopCh:
	}

	mu.Lock()
	defer mu.Unlock()
	if initial {
		c.logger().Info("Cluster: joined the cluster", mlog.Int("contacted", joined), mlog.Int("candidates", len(addrs)))
	} else if joined > 0 {
		c.logger().Debug("Cluster: contacted cluster nodes", mlog.Int("contacted", joined))
	}
}

// loadEncryptionKey returns the gossip encryption key shared by the nodes of
// the cluster through the database, creating it on first use.
func (c *Cluster) loadEncryptionKey() ([]byte, error) {
	raw := make([]byte, 32) // AES-256
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	system, err := c.host.SystemStore().InsertIfExists(&model.System{
		Name:  model.SystemClusterEncryptionKey,
		Value: base64.StdEncoding.EncodeToString(raw),
	})
	if err != nil {
		return nil, err
	}
	if system == nil || system.Value == "" {
		return nil, fmt.Errorf("empty %s", model.SystemClusterEncryptionKey)
	}
	return decodeEncryptionKey(system.Value), nil
}

// decodeEncryptionKey accepts a base64 encoded AES-128/192/256 key. Any
// other value is turned into an AES-256 key by hashing it, so that all the
// nodes still derive the same key.
func decodeEncryptionKey(value string) []byte {
	if key, err := base64.StdEncoding.DecodeString(value); err == nil && memberlist.ValidateKey(key) == nil {
		return key
	}
	sum := sha256.Sum256([]byte(value))
	return sum[:]
}
