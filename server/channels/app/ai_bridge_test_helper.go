// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/mattermost/mattermost/server/public/ai"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// The end-to-end tests' stand-in model (Antimatter), behind /api/v4/system/e2e/ai_bridge, which needs
// ServiceSettings.EnableTesting and a system admin. Installing it makes a server-wide model connection the built-in
// features use (rewrite, recaps, auto-translation), whose calls go through the AI service as any other (its checks,
// limits and usage) and are answered by the completions the tests queued per operation, and recorded. It may also
// set what /api/v4/agents, /agents/status and /llmservices answer. Removing it puts the features' settings back and
// deletes the connection. A server without EnableTesting never uses it.

const (
	e2eAIConnectionName  = "End-to-end test model"
	e2eAIConnectionURL   = "https://e2e-test-model.invalid/v1"
	e2eAIConnectionModel = "e2e-model"
)

type aiE2EModel struct {
	mu sync.Mutex

	connectionID string
	// previous are the features' settings before the model was installed, put back when it's removed.
	previous map[string]model.AIFeatureSettings

	status           *model.AIBridgeTestHelperStatus
	agents           *[]model.BridgeAgentInfo
	services         *[]model.BridgeServiceInfo
	completions      map[string][]model.AIBridgeTestHelperCompletion
	recordRequests   bool
	recordedRequests []model.AIBridgeTestHelperRecordedRequest
}

// aiE2EModel gives the installed stand-in model, only while testing is on.
func (a *App) aiE2EModel() *aiE2EModel {
	if !*a.Config().ServiceSettings.EnableTesting {
		return nil
	}
	a.Srv().ai.mu.Lock()
	defer a.Srv().ai.mu.Unlock()
	return a.Srv().ai.e2eModel
}

func (m *aiE2EModel) serves(connectionID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return connectionID != "" && connectionID == m.connectionID
}

func (m *aiE2EModel) statusOverride() (bool, string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.status == nil {
		return false, "", false
	}
	return m.status.Available, m.status.Reason, true
}

func (m *aiE2EModel) agentsOverride() ([]model.BridgeAgentInfo, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.agents == nil {
		return nil, false
	}
	return slices.Clone(*m.agents), true
}

func (m *aiE2EModel) servicesOverride() ([]model.BridgeServiceInfo, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.services == nil {
		return nil, false
	}
	return slices.Clone(*m.services), true
}

// setConfig checks and takes a configuration, keeping the connection.
func (m *aiE2EModel) setConfig(config *model.AIBridgeTestHelperConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	completions := make(map[string][]model.AIBridgeTestHelperCompletion, len(config.AgentCompletions))
	for operation, list := range config.AgentCompletions {
		if operation == "" {
			return fmt.Errorf("agent completion operation key cannot be empty")
		}
		completions[operation] = make([]model.AIBridgeTestHelperCompletion, 0, len(list))
		for idx, completion := range list {
			if (completion.Completion != "") == (completion.Error != "") {
				return fmt.Errorf("agent completion %q at index %d must set exactly one of completion or error", operation, idx)
			}
			if completion.Error != "" && completion.StatusCode == 0 {
				completion.StatusCode = http.StatusInternalServerError
			}
			completions[operation] = append(completions[operation], completion)
		}
	}
	for idx, agent := range config.Agents {
		if agent.ID == "" {
			return fmt.Errorf("agent at index %d is missing id", idx)
		}
		if agent.Username == "" {
			return fmt.Errorf("agent %q is missing username", agent.ID)
		}
	}
	for idx, service := range config.Services {
		if service.ID == "" {
			return fmt.Errorf("service at index %d is missing id", idx)
		}
		if service.Name == "" {
			return fmt.Errorf("service %q is missing name", service.ID)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = nil
	if config.Status != nil {
		status := *config.Status
		m.status = &status
	}
	m.agents = nil
	if config.Agents != nil {
		agents := slices.Clone(config.Agents)
		m.agents = &agents
	}
	m.services = nil
	if config.Services != nil {
		services := slices.Clone(config.Services)
		m.services = &services
	}
	m.completions = completions
	m.recordRequests = config.RecordRequests != nil && *config.RecordRequests
	m.recordedRequests = []model.AIBridgeTestHelperRecordedRequest{}
	return nil
}

func (m *aiE2EModel) state() *model.AIBridgeTestHelperState {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := &model.AIBridgeTestHelperState{
		RecordRequests:   m.recordRequests,
		RecordedRequests: slices.Clone(m.recordedRequests),
		AgentCompletions: make(map[string][]model.AIBridgeTestHelperCompletion, len(m.completions)),
	}
	if m.status != nil {
		status := *m.status
		state.Status = &status
	}
	if m.agents != nil {
		state.Agents = slices.Clone(*m.agents)
	}
	if m.services != nil {
		state.Services = slices.Clone(*m.services)
	}
	for operation, list := range m.completions {
		state.AgentCompletions[operation] = slices.Clone(list)
	}
	return state
}

// e2eOperationKey gives the operation a call's completions are queued under: the feature's part of its operation
// ("recap_summary:summarize_channel" is recap_summary; the rewrite's "message_rewrite:..." is rewrite).
func e2eOperationKey(operation string) (key, subType string) {
	key, subType, _ = strings.Cut(operation, ":")
	if key == "message_rewrite" {
		key = "rewrite"
	}
	return key, subType
}

// complete answers a chat with the next completion queued for its operation ("" when none), recording it.
func (m *aiE2EModel) complete(call AICall, req ai.ChatRequest) (string, error) {
	key, subType := e2eOperationKey(req.Operation)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.recordRequests {
		recorded := model.AIBridgeTestHelperRecordedRequest{
			Operation:        key,
			ClientOperation:  req.Operation,
			OperationSubType: subType,
			UserID:           call.UserID,
			ChannelID:        call.ChannelID,
			ServiceID:        m.connectionID,
			Messages:         make([]model.AIBridgeTestHelperMessage, 0, len(req.Messages)),
		}
		for _, message := range req.Messages {
			recorded.Messages = append(recorded.Messages, model.AIBridgeTestHelperMessage{Role: string(message.Role), Message: message.Content})
		}
		if len(req.JSONSchema) > 0 {
			var schema map[string]any
			if json.Unmarshal(req.JSONSchema, &schema) == nil {
				recorded.JSONOutputFormat = schema
			}
		}
		m.recordedRequests = append(m.recordedRequests, recorded)
	}
	queue := m.completions[key]
	if len(queue) == 0 {
		return "", nil
	}
	completion := queue[0]
	m.completions[key] = queue[1:]
	if completion.Error != "" {
		return "", &ai.ProviderError{StatusCode: completion.StatusCode, Message: completion.Error}
	}
	return completion.Completion, nil
}

// aiE2EProvider answers a call's chats from the stand-in model.
type aiE2EProvider struct {
	m    *aiE2EModel
	call AICall
}

func (p *aiE2EProvider) Chat(_ context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	text, err := p.m.complete(p.call, req)
	if err != nil {
		return nil, err
	}
	return &ai.ChatResponse{
		Model:        req.Model,
		Message:      ai.Message{Role: ai.RoleAssistant, Content: text},
		FinishReason: ai.FinishReasonStop,
		Usage:        ai.Usage{InputTokens: int64(ai.EstimateMessagesTokens(req.Messages)), OutputTokens: int64(ai.EstimateTokens(text))},
	}, nil
}

func (p *aiE2EProvider) ChatStream(ctx context.Context, req ai.ChatRequest) (*ai.Stream, error) {
	resp, err := p.Chat(ctx, req)
	if err != nil {
		return nil, err
	}
	return ai.StreamFromResponse(resp), nil
}

func (p *aiE2EProvider) Embed(context.Context, ai.EmbedRequest) (*ai.EmbedResponse, error) {
	return nil, ai.ErrUnsupported
}

func (p *aiE2EProvider) Transcribe(context.Context, ai.TranscribeRequest) (*ai.TranscribeResponse, error) {
	return nil, ai.ErrUnsupported
}

func (p *aiE2EProvider) Decide(context.Context, ai.DecideRequest) (*ai.DecideResponse, error) {
	return nil, ai.ErrUnsupported
}

func (p *aiE2EProvider) ListModels(context.Context) ([]ai.ModelInfo, error) {
	return []ai.ModelInfo{{ID: e2eAIConnectionModel}}, nil
}

var _ ai.Provider = (*aiE2EProvider)(nil)

// e2eFeatureSettings are the built-in features' settings the stand-in model takes over.
func e2eFeatureSettings(cfg *model.Config) map[string]*model.AIFeatureSettings {
	return map[string]*model.AIFeatureSettings{
		"rewrite":          cfg.AISettings.Rewrite,
		"recaps":           cfg.AISettings.Recaps,
		"auto_translation": cfg.AISettings.AutoTranslation,
	}
}

// SetAIBridgeTestHelperConfig installs or reconfigures the stand-in model.
func (a *App) SetAIBridgeTestHelperConfig(config *model.AIBridgeTestHelperConfig) *model.AppError {
	invalid := func(err error) *model.AppError {
		return model.NewAppError("SetAIBridgeTestHelperConfig", "app.ai_bridge_test_helper.invalid_config", nil, err.Error(), http.StatusBadRequest)
	}
	a.Srv().ai.mu.Lock()
	m := a.Srv().ai.e2eModel
	a.Srv().ai.mu.Unlock()
	fresh := m == nil
	if fresh {
		m = &aiE2EModel{}
	}
	if err := m.setConfig(config); err != nil {
		return invalid(err)
	}

	if fresh {
		rctx := RequestContextWithMaster(request.EmptyContext(a.Log()))
		connection, appErr := a.CreateAIConnection(rctx, &model.AIConnectionRequest{
			Name:     e2eAIConnectionName,
			Scope:    model.AIConnectionScopeServer,
			Provider: model.AIProviderOpenAICompatible,
			BaseURL:  e2eAIConnectionURL,
			Models:   model.AIConnectionModels{Chat: e2eAIConnectionModel},
			Sharing:  model.AIConnectionSharing{AllTeams: true},
			Audience: model.AIConnectionAudience{Everyone: true, AllowGuests: true},
		}, "")
		if appErr != nil {
			return appErr
		}
		m.connectionID = connection.Id
		m.previous = map[string]model.AIFeatureSettings{}
		for name, settings := range e2eFeatureSettings(a.Config()) {
			m.previous[name] = model.AIFeatureSettings{ConnectionId: model.NewPointer(model.SafeDereference(settings.ConnectionId)), Model: model.NewPointer(model.SafeDereference(settings.Model))}
		}
		a.UpdateConfig(func(cfg *model.Config) {
			for _, settings := range e2eFeatureSettings(cfg) {
				settings.ConnectionId = model.NewPointer(connection.Id)
				settings.Model = model.NewPointer("")
			}
		})
		a.Srv().ai.mu.Lock()
		a.Srv().ai.e2eModel = m
		a.Srv().ai.mu.Unlock()
	}

	if config.FeatureFlags != nil && config.FeatureFlags.EnableAIRecaps != nil {
		configStore := a.Srv().Platform().GetConfigStore()
		configStore.SetReadOnlyFF(false)
		defer configStore.SetReadOnlyFF(true)
		a.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.EnableAIRecaps = *config.FeatureFlags.EnableAIRecaps
		})
	}
	return nil
}

// GetAIBridgeTestHelperState gives the stand-in model's state: its queued completions and recorded requests.
func (a *App) GetAIBridgeTestHelperState() *model.AIBridgeTestHelperState {
	a.Srv().ai.mu.Lock()
	m := a.Srv().ai.e2eModel
	a.Srv().ai.mu.Unlock()
	state := &model.AIBridgeTestHelperState{RecordedRequests: []model.AIBridgeTestHelperRecordedRequest{}}
	if m != nil {
		state = m.state()
	}
	state.FeatureFlags = &model.AIBridgeTestHelperFeatureFlags{
		EnableAIRecaps: model.NewPointer(a.Config().FeatureFlags.EnableAIRecaps),
	}
	return state
}

// ResetAIBridgeTestHelper removes the stand-in model: the features' settings are put back and its connection deleted.
func (a *App) ResetAIBridgeTestHelper() {
	a.Srv().ai.mu.Lock()
	m := a.Srv().ai.e2eModel
	a.Srv().ai.e2eModel = nil
	a.Srv().ai.mu.Unlock()
	if m == nil {
		return
	}
	a.UpdateConfig(func(cfg *model.Config) {
		for name, settings := range e2eFeatureSettings(cfg) {
			if previous, ok := m.previous[name]; ok {
				*settings = previous
			}
		}
	})
	rctx := request.EmptyContext(a.Log())
	if connection, appErr := a.GetAIConnection(rctx, m.connectionID); appErr == nil {
		if appErr := a.DeleteAIConnection(rctx, connection); appErr != nil {
			rctx.Logger().Warn("Couldn't delete the end-to-end tests' model connection", mlog.Err(appErr))
		}
	}
}
