// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// The one-time migration of the Agents plugin's LLM services to model connections (Antimatter).
//
// It reads the plugin's active configuration (its table Agents_ConfigHistory, the row marked active only: the
// history keeps older keys in clear and is never read), and makes a server-wide connection of each service, shared
// with every community and open to everyone as the plugin's were, its key sealed on the way. A service keeps its ID
// when it's a valid one, so that what named it (the plugin's agents, AutoTranslationSettings.Agents.LLMServiceID)
// names the connection. Services of a type Antimatter doesn't speak (Bedrock, Vertex, Cohere North...) become
// connections that need attention, with the reason: they're reached through an OpenAI-compatible endpoint or a
// gateway. The embedding provider becomes the embeddings model of the connection with the same address and key, or a
// connection of its own; the transcription bot's service gets whisper-1. The rewrite and recaps use the default
// bot's service and model. The plugin's data is left as it is.

const aiConnectionsFromAgentsMigrationKey = "AntimatterAIConnectionsFromAgentsMigrationComplete"

type agentsPluginService struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Type               string `json:"type"`
	APIKey             string `json:"apiKey"`
	OrgID              string `json:"orgId"`
	DefaultModel       string `json:"defaultModel"`
	APIURL             string `json:"apiURL"`
	AWSAccessKeyID     string `json:"awsAccessKeyID"`
	AWSSecretAccessKey string `json:"awsSecretAccessKey"`
}

type agentsPluginBot struct {
	Name      string `json:"name"`
	ServiceID string `json:"serviceID"`
	Model     string `json:"model"`
}

type agentsPluginConfig struct {
	Services              []agentsPluginService `json:"services"`
	Bots                  []agentsPluginBot     `json:"bots"`
	DefaultBotName        string                `json:"defaultBotName"`
	TranscriptGenerator   string                `json:"transcriptBackend"`
	EmbeddingSearchConfig struct {
		EmbeddingProvider struct {
			Type       string          `json:"type"`
			Parameters json.RawMessage `json:"parameters"`
		} `json:"embeddingProvider"`
	} `json:"embeddingSearchConfig"`
}

// migratedAIConnection is a connection made from the plugin's configuration, with its secrets still in clear.
type migratedAIConnection struct {
	connection *model.AIConnection
	apiKey     string
	headers    map[string]string
}

const (
	attentionGateway = "This service signs in with its cloud's own credentials, which Antimatter doesn't use. Point this connection to the service's OpenAI-compatible endpoint, or to a gateway such as LiteLLM or OpenRouter, with its address and key."
	attentionAzure   = "Azure OpenAI: check the address. It was set to the resource's OpenAI v1 endpoint (…/openai/v1) with the key sent as api-key; with Entra ID sign-in, go through a gateway."
	attentionNoURL   = "This OpenAI-compatible service had no address: set it."
	attentionUnknown = "This service's type isn't one Antimatter speaks: point the connection to an OpenAI-compatible endpoint or a gateway."
)

// withV1 gives an OpenAI-style base URL with its /v1: the plugin's addresses come with or without it.
func withV1(raw string) string {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" || strings.HasSuffix(raw, "/v1") {
		return raw
	}
	return raw + "/v1"
}

func trimName(name string) string {
	name = strings.TrimSpace(name)
	if runes := []rune(name); len(runes) > model.AIConnectionNameMaxRunes {
		name = string(runes[:model.AIConnectionNameMaxRunes])
	}
	return name
}

// agentsServiceToConnection maps one of the plugin's services; nil for one to leave out (the load-test mock).
func agentsServiceToConnection(svc agentsPluginService) *migratedAIConnection {
	c := &model.AIConnection{
		Name:     trimName(svc.Name),
		Scope:    model.AIConnectionScopeServer,
		Provider: model.AIProviderOpenAICompatible,
		Models:   model.AIConnectionModels{Chat: strings.TrimSpace(svc.DefaultModel)},
		Sharing:  model.AIConnectionSharing{AllTeams: true},
		Audience: model.AIConnectionAudience{Everyone: true},
	}
	if c.Name == "" {
		c.Name = trimName(svc.Type)
	}
	if model.IsValidId(svc.ID) {
		c.Id = svc.ID
	}
	m := &migratedAIConnection{connection: c, apiKey: strings.TrimSpace(svc.APIKey), headers: map[string]string{}}
	url := strings.TrimSpace(svc.APIURL)

	switch svc.Type {
	case "openai":
		c.BaseURL = withV1(url)
		if c.BaseURL == "" {
			c.BaseURL = "https://api.openai.com/v1"
		}
		if org := strings.TrimSpace(svc.OrgID); org != "" {
			m.headers["OpenAI-Organization"] = org
		}
	case "openaicompatible":
		c.BaseURL = withV1(url)
		if c.BaseURL == "" {
			c.NeedsAttention = attentionNoURL
		}
	case "anthropic":
		c.Provider = model.AIProviderAnthropic
		c.BaseURL = withV1(url)
	case "mistral":
		c.BaseURL = withV1(url)
		if c.BaseURL == "" {
			c.BaseURL = "https://api.mistral.ai/v1"
		}
	case "cohere":
		c.BaseURL = url
		if c.BaseURL == "" {
			c.BaseURL = "https://api.cohere.ai/compatibility/v1"
		}
	case "gemini":
		c.BaseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
		if url != "" {
			c.BaseURL = url
			c.NeedsAttention = attentionUnknown
		}
	case "azure":
		if url != "" {
			c.BaseURL = strings.TrimRight(url, "/") + "/openai/v1"
		}
		if m.apiKey != "" {
			m.headers["api-key"] = m.apiKey
			m.apiKey = ""
		}
		c.NeedsAttention = attentionAzure
	case "bedrock", "vertex", "north":
		// The AWS secret and the Vertex service account aren't carried over: Antimatter never uses them.
		c.BaseURL = ""
		c.NeedsAttention = attentionGateway
	case "loadtest_mock":
		return nil
	default:
		c.NeedsAttention = attentionUnknown
	}
	if !model.ValidAIBaseURL(c.BaseURL) {
		c.BaseURL = ""
		if c.NeedsAttention == "" {
			c.NeedsAttention = attentionUnknown
		}
	}
	return m
}

// agentsConfigToConnections maps the plugin's configuration to connections, and gives the connection and model the
// built-in features should use (from the default bot), and the transcription service.
func agentsConfigToConnections(cfg agentsPluginConfig) (connections []*migratedAIConnection, featureConnection, featureModel string) {
	byServiceID := map[string]*migratedAIConnection{}
	for _, svc := range cfg.Services {
		m := agentsServiceToConnection(svc)
		if m == nil {
			continue
		}
		if m.connection.Id == "" {
			m.connection.Id = model.NewId()
		}
		connections = append(connections, m)
		byServiceID[svc.ID] = m
	}

	botService := func(name string) (*migratedAIConnection, string) {
		for _, bot := range cfg.Bots {
			if bot.Name == name && name != "" {
				return byServiceID[bot.ServiceID], strings.TrimSpace(bot.Model)
			}
		}
		return nil, ""
	}

	if m, botModel := botService(cfg.DefaultBotName); m != nil && m.connection.NeedsAttention == "" {
		featureConnection, featureModel = m.connection.Id, botModel
	}
	if m, _ := botService(cfg.TranscriptGenerator); m != nil && m.connection.Provider == model.AIProviderOpenAICompatible && m.connection.Models.Transcription == "" {
		// The plugin always transcribed with whisper-1.
		m.connection.Models.Transcription = "whisper-1"
	}

	// The embedding provider: the embeddings model of the connection with the same address and key, or its own.
	provider := cfg.EmbeddingSearchConfig.EmbeddingProvider
	if provider.Type != "" && len(provider.Parameters) > 0 {
		var params struct {
			Provider       string `json:"provider"`
			APIKey         string `json:"apiKey"`
			APIURL         string `json:"apiURL"`
			EmbeddingModel string `json:"embeddingModel"`
			Model          string `json:"model"`
		}
		if json.Unmarshal(provider.Parameters, &params) == nil {
			embedModel := strings.TrimSpace(params.EmbeddingModel)
			if embedModel == "" {
				embedModel = strings.TrimSpace(params.Model)
			}
			supported := provider.Type == "openai" || provider.Type == "openai-compatible" || (provider.Type == "bifrost" && params.Provider == "openai")
			if supported && embedModel != "" {
				baseURL := withV1(params.APIURL)
				if baseURL == "" {
					baseURL = "https://api.openai.com/v1"
				}
				var target *migratedAIConnection
				for _, m := range connections {
					if m.connection.Provider == model.AIProviderOpenAICompatible && m.connection.BaseURL == baseURL && m.apiKey == strings.TrimSpace(params.APIKey) {
						target = m
						break
					}
				}
				if target == nil {
					target = &migratedAIConnection{
						connection: &model.AIConnection{
							Id:       model.NewId(),
							Name:     "Embeddings",
							Scope:    model.AIConnectionScopeServer,
							Provider: model.AIProviderOpenAICompatible,
							BaseURL:  baseURL,
							Sharing:  model.AIConnectionSharing{AllTeams: true},
							Audience: model.AIConnectionAudience{Everyone: true},
						},
						apiKey:  strings.TrimSpace(params.APIKey),
						headers: map[string]string{},
					}
					connections = append(connections, target)
				}
				target.connection.Models.Embeddings = embedModel
			}
		}
	}
	return connections, featureConnection, featureModel
}

// doAIConnectionsFromAgentsMigration runs the migration once. It never stops the server: a failure is logged, and
// the migration is tried again at the next start unless the plugin's configuration can't be read at all.
func (s *Server) doAIConnectionsFromAgentsMigration() error {
	var nfErr *store.ErrNotFound
	if _, err := s.Store().System().GetByName(aiConnectionsFromAgentsMigrationKey); err == nil {
		return nil
	} else if !errors.As(err, &nfErr) {
		return fmt.Errorf("could not query migration: %w", err)
	}
	markDone := func() error {
		if err := s.Store().System().SaveOrUpdate(&model.System{Name: aiConnectionsFromAgentsMigrationKey, Value: "true"}); err != nil {
			return fmt.Errorf("failed to mark the AI connections migration as completed: %w", err)
		}
		return nil
	}

	rctx := request.EmptyContext(s.Log())
	raw, err := s.Store().AIConnection().GetAgentsPluginConfig(rctx)
	if err != nil {
		s.Log().Error("Couldn't read the Agents plugin's services: they'll be migrated to model connections at the next start", mlog.Err(err))
		return nil
	}
	if strings.TrimSpace(raw) == "" {
		return markDone()
	}
	var cfg agentsPluginConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		s.Log().Warn("The Agents plugin's configuration can't be read: its services aren't migrated to model connections", mlog.Err(err))
		return markDone()
	}
	connections, featureConnection, featureModel := agentsConfigToConnections(cfg)
	if len(connections) == 0 {
		return markDone()
	}
	if _, appErr := s.aiKeyring(); appErr != nil {
		s.Log().Error("The Agents plugin's services wait for an AI encryption key to become model connections", mlog.Err(appErr))
		return nil
	}

	a := New(ServerConnector(s.Channels()))
	migrated := 0
	for _, m := range connections {
		c := m.connection
		if existing, err := s.Store().AIConnection().Get(rctx, c.Id); err == nil && existing != nil {
			continue
		}
		apiKey, headers := m.apiKey, m.headers
		if appErr := a.sealAIConnectionSecrets(c, &apiKey, &headers); appErr != nil {
			s.Log().Error("Couldn't seal the key of a migrated model connection", mlog.String("connection_id", c.Id), mlog.Err(appErr))
			return nil
		}
		if _, err := s.Store().AIConnection().Save(rctx, c); err != nil {
			s.Log().Error("Couldn't save a model connection migrated from the Agents plugin", mlog.String("connection_id", c.Id), mlog.String("name", c.Name), mlog.Err(err))
			continue
		}
		migrated++
		if c.NeedsAttention != "" {
			s.Log().Warn("A model connection migrated from the Agents plugin needs attention", mlog.String("connection_id", c.Id), mlog.String("name", c.Name))
		}
	}

	if featureConnection != "" {
		cfgNow := s.Config()
		rewrite := !cfgNow.AISettings.Rewrite.IsConfigured()
		recaps := !cfgNow.AISettings.Recaps.IsConfigured()
		if rewrite || recaps {
			s.platform.UpdateConfig(func(cfg *model.Config) {
				if rewrite {
					cfg.AISettings.Rewrite.ConnectionId = model.NewPointer(featureConnection)
					cfg.AISettings.Rewrite.Model = model.NewPointer(featureModel)
				}
				if recaps {
					cfg.AISettings.Recaps.ConnectionId = model.NewPointer(featureConnection)
					cfg.AISettings.Recaps.Model = model.NewPointer(featureModel)
				}
			})
		}
	}

	s.Log().Info("Migrated the Agents plugin's services to model connections", mlog.Int("connections", migrated))
	return markDone()
}
