// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package autotranslation

import (
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/jobs"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

// Host is the narrow view of the server the service depends on; the app layer
// implements it (channels/app/auto_translation_host.go). Keeping it an
// interface lets the unit tests run without a database or a real server.
type Host interface {
	Config() *model.Config
	Store() store.Store
	Log() mlog.LoggerIFace
	Metrics() einterfaces.MetricsInterface
	Publish(event *model.WebSocketEvent)
	AddConfigListener(listener func(oldCfg, newCfg *model.Config)) string
	RemoveConfigListener(id string)
	// HTTPClient returns a client for calls to the administrator-configured
	// translation provider.
	HTTPClient() *http.Client
	// AgentsBridgeStatus reports whether the Agents plugin bridge can be used.
	AgentsBridgeStatus() (bool, string)
	// AgentsServiceCompletion runs a non-streaming completion against the
	// given LLM service through the Agents plugin bridge.
	AgentsServiceCompletion(serviceID string, req CompletionRequest) (string, error)
	// JobServer returns the job server, used to build the recovery job.
	JobServer() *jobs.JobServer
}

// CompletionMessage is a message of a completion request.
type CompletionMessage struct {
	Role    string
	Message string
}

// CompletionRequest is an auto-translation completion run through the Agents
// plugin bridge (the host sends it as the bridge's auto_translate operation).
type CompletionRequest struct {
	Messages         []CompletionMessage
	JSONOutputFormat map[string]any
}
