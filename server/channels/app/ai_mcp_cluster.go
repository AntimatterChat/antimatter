// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// clusterEventInvalidateAIMCPTools tells the other nodes to drop their cached tool lists of an MCP server (all of
// them, or one person's).
const clusterEventInvalidateAIMCPTools = model.ClusterEvent("inv_ai_mcp_tools")

type aiMCPToolsInvalidation struct {
	ServerID string `json:"server_id"`
	UserID   string `json:"user_id,omitempty"`
}

// forgetAIMCPTools drops an MCP server's cached tool lists (userID "") or one person's, on this node and the others.
func (a *App) forgetAIMCPTools(serverID, userID string) {
	a.aiMCP().forget(serverID, userID)
	cluster := a.Srv().Platform().Cluster()
	if cluster == nil {
		return
	}
	data, err := json.Marshal(aiMCPToolsInvalidation{ServerID: serverID, UserID: userID})
	if err != nil {
		return
	}
	cluster.SendClusterMessage(&model.ClusterMessage{Event: clusterEventInvalidateAIMCPTools, SendType: model.ClusterSendReliable, Data: data})
}

func (s *Server) clusterInvalidateAIMCPToolsHandler(msg *model.ClusterMessage) {
	applyAIMCPToolsInvalidation(s.Log(), &s.ai.mcp, msg)
}

func applyAIMCPToolsInvalidation(logger mlog.LoggerIFace, state *aiMCPState, msg *model.ClusterMessage) {
	var inv aiMCPToolsInvalidation
	if err := json.Unmarshal(msg.Data, &inv); err != nil || inv.ServerID == "" {
		logger.Warn("Couldn't read which MCP server's tools to forget", mlog.Err(err))
		return
	}
	state.forget(inv.ServerID, inv.UserID)
}
