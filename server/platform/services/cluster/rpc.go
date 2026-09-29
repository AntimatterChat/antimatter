// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package cluster

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

const (
	opGetLogs         = model.ClusterGossipEventRequestGetLogs
	opQueryLogs       = "gossip_request_query_logs"
	opSupportPacket   = model.ClusterGossipEventRequestGenerateSupportPacket
	opClusterStats    = model.ClusterGossipEventRequestGetClusterStats
	opPluginStatuses  = model.ClusterGossipEventRequestGetPluginStatuses
	opSaveConfig      = model.ClusterGossipEventRequestSaveConfig
	opWebConnCount    = model.ClusterGossipEventRequestWebConnCount
	opWSQueues        = model.ClusterGossipEventRequestWSQueues
	maxConcurrentRPCs = 32
)

var errTimeout = errors.New("timed out waiting for the cluster node to respond")

type rpcTimeouts struct {
	short         time.Duration // websocket queues and connection counts
	normal        time.Duration // stats, plugin statuses, configuration
	logs          time.Duration
	supportPacket time.Duration // added to the CPU profile duration
}

func defaultRPCTimeouts() rpcTimeouts {
	return rpcTimeouts{
		short:         5 * time.Second,
		normal:        15 * time.Second,
		logs:          30 * time.Second,
		supportPacket: 90 * time.Second,
	}
}

type rpcRequest struct {
	ID      string          `json:"id"`
	From    string          `json:"from"`
	Op      string          `json:"op"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type rpcResponse struct {
	ID      string          `json:"id"`
	From    string          `json:"from"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// rpcResult is the outcome of a request for one node.
type rpcResult struct {
	node    string
	payload json.RawMessage
	// remoteErr is an error reported by the remote node while serving the
	// request (the payload may still hold partial data).
	remoteErr string
	// err is set when no response was received.
	err error
}

type pendingCall struct {
	results chan rpcResult
	targets map[string]bool
}

type rpcManager struct {
	c       *Cluster
	mu      sync.Mutex
	pending map[string]*pendingCall
	sem     chan struct{}
}

func newRPCManager(c *Cluster) *rpcManager {
	return &rpcManager{
		c:       c,
		pending: make(map[string]*pendingCall),
		sem:     make(chan struct{}, maxConcurrentRPCs),
	}
}

// call sends a request to every other node and collects their responses. It
// returns one result per node; nodes which didn't answer in time have err
// set.
func (r *rpcManager) call(op string, args any, timeout time.Duration) (map[string]rpcResult, error) {
	if !r.c.running.Load() {
		return map[string]rpcResult{}, nil
	}
	peers := r.c.peers()
	results := make(map[string]rpcResult, len(peers))
	if len(peers) == 0 {
		return results, nil
	}

	start := time.Now()
	if metrics := r.c.host.Metrics(); metrics != nil {
		metrics.IncrementClusterRequest()
		defer func() {
			metrics.ObserveClusterRequestDuration(time.Since(start).Seconds())
		}()
	}

	req := rpcRequest{ID: model.NewId(), From: r.c.id, Op: op}
	if args != nil {
		b, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		req.Payload = b
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	payloads := splitPayload(append(newEnvelope(kindRequest, 0, len(body)), body...))

	pc := &pendingCall{
		results: make(chan rpcResult, 2*len(peers)),
		targets: make(map[string]bool, len(peers)),
	}
	for _, p := range peers {
		pc.targets[p.node.Name] = true
	}
	r.mu.Lock()
	r.pending[req.ID] = pc
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.pending, req.ID)
		r.mu.Unlock()
	}()

	for _, p := range peers {
		name := p.node.Name
		if _, err := r.c.enqueueReliable(name, payloads, false, true); err != nil {
			results[name] = rpcResult{node: name, err: err}
		}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for len(results) < len(peers) {
		select {
		case res := <-pc.results:
			if _, done := results[res.node]; !done && pc.targets[res.node] {
				results[res.node] = res
			}
		case <-timer.C:
			for name := range pc.targets {
				if _, done := results[name]; !done {
					results[name] = rpcResult{node: name, err: errTimeout}
				}
			}
		}
	}
	return results, nil
}

func (r *rpcManager) deliver(id string, res rpcResult) {
	r.mu.Lock()
	pc, ok := r.pending[id]
	r.mu.Unlock()
	if !ok {
		return
	}
	select {
	case pc.results <- res:
	default:
	}
}

func (r *rpcManager) nodeLeft(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, pc := range r.pending {
		if pc.targets[name] {
			select {
			case pc.results <- rpcResult{node: name, err: errNodeGone}:
			default:
			}
		}
	}
}

func (r *rpcManager) failAll(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, pc := range r.pending {
		for name := range pc.targets {
			select {
			case pc.results <- rpcResult{node: name, err: err}:
			default:
			}
		}
	}
}

func (r *rpcManager) handleResponse(body []byte) {
	var resp rpcResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		r.c.logger().Warn("Cluster: discarding malformed response", mlog.Err(err))
		return
	}
	r.deliver(resp.ID, rpcResult{node: resp.From, payload: resp.Payload, remoteErr: resp.Error})
}

func (r *rpcManager) handleRequest(body []byte) {
	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		r.c.logger().Warn("Cluster: discarding malformed request", mlog.Err(err))
		return
	}

	go func() {
		select {
		case r.sem <- struct{}{}:
		case <-r.c.stopCh:
			return
		}
		defer func() { <-r.sem }()

		resp := rpcResponse{ID: req.ID, From: r.c.id}
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					resp.Payload = nil
					resp.Error = fmt.Sprintf("panic while serving %s: %v", req.Op, rec)
				}
			}()
			payload, err := r.c.serve(req.Op, req.Payload)
			if payload != nil {
				b, mErr := json.Marshal(payload)
				if mErr != nil {
					err = errors.Join(err, mErr)
				} else {
					resp.Payload = b
				}
			}
			if err != nil {
				resp.Error = err.Error()
			}
		}()
		if resp.Error != "" {
			r.c.logger().Warn("Cluster: error while serving a request from another node", mlog.String("op", req.Op), mlog.String("node_id", req.From), mlog.String("error", resp.Error))
		}

		b, err := json.Marshal(resp)
		if err != nil {
			r.c.logger().Error("Cluster: failed to encode response", mlog.String("op", req.Op), mlog.Err(err))
			return
		}
		payload := append(newEnvelope(kindResponse, 0, len(b)), b...)
		if _, err := r.c.enqueueReliable(req.From, splitPayload(payload), false, true); err != nil {
			r.c.logger().Warn("Cluster: failed to send response", mlog.String("op", req.Op), mlog.String("node_id", req.From), mlog.Err(err))
		}
	}()
}

// --- request arguments -----------------------------------------------------

type logsArgs struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
}

type userArgs struct {
	UserID string `json:"user_id"`
}

type wsQueuesArgs struct {
	UserID       string `json:"user_id"`
	ConnectionID string `json:"connection_id"`
	SeqNum       int64  `json:"seq_num"`
}

const logSeparator = "-----------------------------------------------------------------------------------------------------------"

// serve computes this node's answer to a request from another node.
func (c *Cluster) serve(op string, raw json.RawMessage) (any, error) {
	rctx := newRequestContext(c.host.Log())
	switch op {
	case opGetLogs:
		var args logsArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		lines, appErr := c.host.GetLogsSkipSend(rctx, args.Page, args.PerPage, &model.LogFilter{})
		if appErr != nil {
			return nil, appErr
		}
		hostname := c.GetMyClusterInfo().Hostname
		out := make([]string, 0, len(lines)+5)
		out = append(out, logSeparator, logSeparator, hostname, logSeparator, logSeparator)
		return append(out, lines...), nil

	case opQueryLogs:
		var args logsArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		lines, appErr := c.host.GetLogsSkipSend(rctx, args.Page, args.PerPage, &model.LogFilter{})
		if appErr != nil {
			return nil, appErr
		}
		return map[string][]string{c.GetMyClusterInfo().Hostname: lines}, nil

	case opSupportPacket:
		var options model.SupportPacketOptions
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &options); err != nil {
				return nil, err
			}
		}
		files, err := c.host.GenerateSupportPacket(rctx, &options)
		hostname := c.GetMyClusterInfo().Hostname
		for i := range files {
			files[i].Filename = hostname + "/" + files[i].Filename
		}
		return files, err

	case opClusterStats:
		return &model.ClusterStats{
			Id:                        c.id,
			TotalWebsocketConnections: c.host.TotalWebsocketConnections(),
			TotalReadDbConnections:    c.host.TotalReadDbConnections(),
			TotalMasterDbConnections:  c.host.TotalMasterDbConnections(),
		}, nil

	case opPluginStatuses:
		statuses, appErr := c.host.GetPluginStatuses()
		if appErr != nil {
			if appErr.Id == "app.plugin.disabled.app_error" {
				return model.PluginStatuses{}, nil
			}
			return nil, appErr
		}
		return statuses, nil

	case opSaveConfig:
		var cfg model.Config
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return nil, err
		}
		return nil, c.host.ApplyRemoteConfig(&cfg)

	case opWebConnCount:
		var args userArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		return c.host.WebConnCountForUser(args.UserID), nil

	case opWSQueues:
		var args wsQueuesArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		return c.host.GetWSQueues(args.UserID, args.ConnectionID, args.SeqNum)
	}
	return nil, fmt.Errorf("unknown cluster request %q", op)
}
