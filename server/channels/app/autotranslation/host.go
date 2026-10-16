// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package autotranslation

import (
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/jobs"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

// host is the narrow view of the server the service depends on. Keeping it an
// interface lets the unit tests run without a database or a real server.
type host interface {
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
	AgentsServiceCompletion(serviceID string, req app.BridgeCompletionRequest) (string, error)
	// JobServer returns the job server, used to build the recovery job.
	JobServer() *jobs.JobServer
}

// serverHost adapts *app.Server to the host interface.
type serverHost struct {
	srv *app.Server
}

func newServerHost(srv *app.Server) *serverHost {
	return &serverHost{srv: srv}
}

func (h *serverHost) Config() *model.Config { return h.srv.Config() }
func (h *serverHost) Store() store.Store    { return h.srv.Store() }
func (h *serverHost) Log() mlog.LoggerIFace { return h.srv.Log() }

func (h *serverHost) Metrics() einterfaces.MetricsInterface { return h.srv.GetMetrics() }

func (h *serverHost) Publish(event *model.WebSocketEvent) {
	h.srv.Platform().Publish(event)
}

func (h *serverHost) AddConfigListener(listener func(oldCfg, newCfg *model.Config)) string {
	return h.srv.Platform().AddConfigListener(listener)
}

func (h *serverHost) RemoveConfigListener(id string) {
	h.srv.Platform().RemoveConfigListener(id)
}

func (h *serverHost) HTTPClient() *http.Client {
	// The provider URL is set by a system administrator, so it is trusted in
	// the same way as other admin-configured integration endpoints (it is
	// commonly a LibreTranslate instance on the internal network).
	return h.srv.HTTPService().MakeClient(true)
}

func (h *serverHost) app() *app.App {
	ch := h.srv.Channels()
	if ch == nil {
		return nil
	}
	return app.New(app.ServerConnector(ch))
}

func (h *serverHost) AgentsBridgeStatus() (bool, string) {
	a := h.app()
	if a == nil {
		return false, "app.agents.bridge.not_available.plugin_env_not_initialized"
	}
	return a.GetAIPluginBridgeStatus(request.EmptyContext(h.srv.Log()))
}

func (h *serverHost) AgentsServiceCompletion(serviceID string, req app.BridgeCompletionRequest) (string, error) {
	a := h.app()
	if a == nil {
		return "", errAgentsUnavailable
	}
	return a.ServiceCompletion("", serviceID, req)
}

func (h *serverHost) JobServer() *jobs.JobServer { return h.srv.Jobs }
