// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package cluster

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

func newRequestContext(logger mlog.LoggerIFace) request.CTX {
	return request.EmptyContext(logger)
}

// sortedResults returns the results ordered by node id, so that aggregated
// output is stable.
func sortedResults(results map[string]rpcResult) []rpcResult {
	out := make([]rpcResult, 0, len(results))
	for _, r := range results {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].node < out[j].node })
	return out
}

// logFailure logs a node which couldn't answer a request. Aggregating
// methods tolerate such failures and return the data of the other nodes.
func (c *Cluster) logFailure(op string, res rpcResult) {
	fields := []mlog.Field{mlog.String("op", op), mlog.String("node_id", res.node)}
	if res.err != nil {
		c.logger().Warn("Cluster: node did not answer", append(fields, mlog.Err(res.err))...)
	} else if res.remoteErr != "" {
		c.logger().Warn("Cluster: node failed to answer", append(fields, mlog.String("error", res.remoteErr))...)
	}
}

func decodeResult[T any](res rpcResult, out *T) error {
	if res.err != nil {
		return res.err
	}
	if res.remoteErr != "" && len(res.payload) == 0 {
		return errors.New(res.remoteErr)
	}
	if len(res.payload) == 0 {
		return nil
	}
	return json.Unmarshal(res.payload, out)
}

func timeoutAppError(where string, err error) *model.AppError {
	return model.NewAppError(where, "ent.cluster.timeout.error", nil, "", http.StatusInternalServerError).Wrap(err)
}

func encodeAppError(where string, err error) *model.AppError {
	return model.NewAppError(where, "ent.cluster.json_encode.error", nil, "", http.StatusInternalServerError).Wrap(err)
}

// GetClusterStats returns the statistics of the other nodes.
func (c *Cluster) GetClusterStats(rctx request.CTX) ([]*model.ClusterStats, *model.AppError) {
	results, err := c.rpc.call(opClusterStats, nil, c.timeouts.normal)
	if err != nil {
		return nil, encodeAppError("GetClusterStats", err)
	}
	stats := make([]*model.ClusterStats, 0, len(results))
	for _, res := range sortedResults(results) {
		var s model.ClusterStats
		if err := decodeResult(res, &s); err != nil {
			c.logFailure(opClusterStats, res)
			continue
		}
		stats = append(stats, &s)
	}
	return stats, nil
}

// GetLogs returns the latest log lines of the other nodes, each block being
// preceded by the node's hostname.
func (c *Cluster) GetLogs(rctx request.CTX, page, perPage int) ([]string, *model.AppError) {
	results, err := c.rpc.call(opGetLogs, logsArgs{Page: page, PerPage: perPage}, c.timeouts.logs)
	if err != nil {
		return nil, encodeAppError("GetLogs", err)
	}
	var lines []string
	for _, res := range sortedResults(results) {
		var nodeLines []string
		if err := decodeResult(res, &nodeLines); err != nil {
			c.logFailure(opGetLogs, res)
			continue
		}
		lines = append(lines, nodeLines...)
	}
	if lines == nil {
		lines = []string{}
	}
	return lines, nil
}

// QueryLogs returns the latest log lines of the other nodes, by hostname.
func (c *Cluster) QueryLogs(rctx request.CTX, page, perPage int) (map[string][]string, *model.AppError) {
	results, err := c.rpc.call(opQueryLogs, logsArgs{Page: page, PerPage: perPage}, c.timeouts.logs)
	if err != nil {
		return nil, encodeAppError("QueryLogs", err)
	}
	logs := make(map[string][]string)
	for _, res := range sortedResults(results) {
		var nodeLogs map[string][]string
		if err := decodeResult(res, &nodeLogs); err != nil {
			c.logFailure(opQueryLogs, res)
			continue
		}
		for host, lines := range nodeLogs {
			if _, exists := logs[host]; exists {
				// Two nodes with the same hostname: keep both.
				host = fmt.Sprintf("%s (%s)", host, res.node)
			}
			logs[host] = lines
		}
	}
	return logs, nil
}

// GenerateSupportPacket collects the platform support packet files of the
// other nodes, keyed by node id. File names are prefixed with the hostname
// of the node which generated them.
func (c *Cluster) GenerateSupportPacket(rctx request.CTX, options *model.SupportPacketOptions) (map[string][]model.FileData, error) {
	timeout := c.timeouts.supportPacket
	if options != nil && options.CPUProfileDuration != nil {
		timeout += *options.CPUProfileDuration
	} else {
		timeout += 5 * time.Second
	}
	results, err := c.rpc.call(opSupportPacket, options, timeout)
	if err != nil {
		return nil, err
	}

	files := make(map[string][]model.FileData, len(results))
	var errs []error
	for _, res := range sortedResults(results) {
		if res.err != nil {
			errs = append(errs, fmt.Errorf("node %s: %w", res.node, res.err))
			continue
		}
		if res.remoteErr != "" {
			errs = append(errs, fmt.Errorf("node %s: %s", res.node, res.remoteErr))
		}
		var nodeFiles []model.FileData
		if len(res.payload) > 0 {
			if err := json.Unmarshal(res.payload, &nodeFiles); err != nil {
				errs = append(errs, fmt.Errorf("node %s: %w", res.node, err))
				continue
			}
		}
		files[res.node] = nodeFiles
	}
	return files, errors.Join(errs...)
}

// GetPluginStatuses returns the plugin statuses of the other nodes.
func (c *Cluster) GetPluginStatuses() (model.PluginStatuses, *model.AppError) {
	results, err := c.rpc.call(opPluginStatuses, nil, c.timeouts.normal)
	if err != nil {
		return nil, encodeAppError("GetPluginStatuses", err)
	}
	var statuses model.PluginStatuses
	for _, res := range sortedResults(results) {
		var nodeStatuses model.PluginStatuses
		if err := decodeResult(res, &nodeStatuses); err != nil {
			c.logFailure(opPluginStatuses, res)
			continue
		}
		statuses = append(statuses, nodeStatuses...)
	}
	return statuses, nil
}

// ConfigChanged is called after the configuration was saved on this node.
// When sendToOtherServer is true, the other nodes are asked to apply the
// new configuration.
func (c *Cluster) ConfigChanged(previousConfig *model.Config, newConfig *model.Config, sendToOtherServer bool) *model.AppError {
	if newConfig == nil {
		return nil
	}
	// The hash covers the effective configuration (with environment
	// overrides), which the config listener normally already refreshed.
	c.refreshConfigHash(c.host.Config())

	if previousConfig != nil && c.running.Load() && !reflect.DeepEqual(previousConfig.ClusterSettings, newConfig.ClusterSettings) {
		c.logger().Warn("Cluster configuration has changed. The cluster may become unstable and a restart is required. To ensure the cluster is configured correctly you should perform a rolling restart immediately.", mlog.String("node_id", c.id))
	}

	if !sendToOtherServer || !c.running.Load() {
		return nil
	}

	results, err := c.rpc.call(opSaveConfig, newConfig, c.timeouts.normal)
	if err != nil {
		return encodeAppError("ConfigChanged", err)
	}
	for _, res := range results {
		if res.err != nil || res.remoteErr != "" {
			c.logFailure(opSaveConfig, res)
		}
	}
	return nil
}

// WebConnCountForUser returns the number of websocket connections of the
// user on the other nodes. Any node failing to answer results in an error,
// so that callers don't wrongly consider the user offline.
func (c *Cluster) WebConnCountForUser(userID string) (int, *model.AppError) {
	results, err := c.rpc.call(opWebConnCount, userArgs{UserID: userID}, c.timeouts.short)
	if err != nil {
		return 0, encodeAppError("WebConnCountForUser", err)
	}
	total := 0
	for _, res := range results {
		var count int
		if err := decodeResult(res, &count); err != nil {
			if errors.Is(err, errNodeGone) {
				// The node left the cluster: it has no connection anymore.
				continue
			}
			return 0, timeoutAppError("WebConnCountForUser", fmt.Errorf("node %s: %w", res.node, err))
		}
		total += count
	}
	return total, nil
}

// GetWSQueues returns, for each other node, the websocket queues it holds
// for the given connection (nil when it has none).
func (c *Cluster) GetWSQueues(userID, connectionID string, seqNum int64) (map[string]*model.WSQueues, error) {
	results, err := c.rpc.call(opWSQueues, wsQueuesArgs{UserID: userID, ConnectionID: connectionID, SeqNum: seqNum}, c.timeouts.short)
	if err != nil {
		return nil, err
	}
	queues := make(map[string]*model.WSQueues, len(results))
	for _, res := range results {
		var q *model.WSQueues
		if err := decodeResult(res, &q); err != nil {
			if errors.Is(err, errNodeGone) {
				continue
			}
			return nil, fmt.Errorf("failed to get websocket queues from node %s: %w", res.node, err)
		}
		queues[res.node] = q
	}
	return queues, nil
}
