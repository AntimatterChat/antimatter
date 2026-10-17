// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app/autotranslation"
	"github.com/mattermost/mattermost/server/v8/channels/jobs"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

// autoTranslationHost implements autotranslation.Host on top of the server.
type autoTranslationHost struct {
	srv *Server
}

var _ autotranslation.Host = (*autoTranslationHost)(nil)

func newAutoTranslationHost(srv *Server) *autoTranslationHost {
	return &autoTranslationHost{srv: srv}
}

func (h *autoTranslationHost) Config() *model.Config { return h.srv.Config() }
func (h *autoTranslationHost) Store() store.Store    { return h.srv.Store() }
func (h *autoTranslationHost) Log() mlog.LoggerIFace { return h.srv.Log() }

func (h *autoTranslationHost) Metrics() einterfaces.MetricsInterface { return h.srv.GetMetrics() }

func (h *autoTranslationHost) Publish(event *model.WebSocketEvent) {
	h.srv.Platform().Publish(event)
}

func (h *autoTranslationHost) AddConfigListener(listener func(oldCfg, newCfg *model.Config)) string {
	return h.srv.Platform().AddConfigListener(listener)
}

func (h *autoTranslationHost) RemoveConfigListener(id string) {
	h.srv.Platform().RemoveConfigListener(id)
}

func (h *autoTranslationHost) HTTPClient() *http.Client {
	// The provider URL is set by a system administrator, so it is trusted in
	// the same way as other admin-configured integration endpoints (it is
	// commonly a LibreTranslate instance on the internal network).
	return h.srv.HTTPService().MakeClient(true)
}

func (h *autoTranslationHost) app() *App {
	ch := h.srv.Channels()
	if ch == nil {
		return nil
	}
	return New(ServerConnector(ch))
}

func (h *autoTranslationHost) AgentsBridgeStatus() (bool, string) {
	a := h.app()
	if a == nil {
		return false, "app.agents.bridge.not_available.plugin_env_not_initialized"
	}
	return a.GetAIPluginBridgeStatus(request.EmptyContext(h.srv.Log()))
}

func (h *autoTranslationHost) AgentsServiceCompletion(serviceID string, req autotranslation.CompletionRequest) (string, error) {
	a := h.app()
	if a == nil {
		return "", autotranslation.ErrAgentsUnavailable
	}
	return a.ServiceCompletion("", serviceID, autoTranslationBridgeRequest(req))
}

// autoTranslationBridgeRequest turns an auto-translation completion into the
// Agents bridge's request for the auto_translate operation.
func autoTranslationBridgeRequest(req autotranslation.CompletionRequest) BridgeCompletionRequest {
	messages := make([]BridgeMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, BridgeMessage{Role: m.Role, Message: m.Message})
	}
	return BridgeCompletionRequest{
		Operation:        BridgeOperationAutoTranslate,
		ClientOperation:  string(BridgeOperationAutoTranslate),
		Messages:         messages,
		JSONOutputFormat: req.JSONOutputFormat,
	}
}

func (h *autoTranslationHost) JobServer() *jobs.JobServer { return h.srv.Jobs }
