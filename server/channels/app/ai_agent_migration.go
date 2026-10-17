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

// The one-time migration of the Agents plugin's bots to agents (Antimatter).
//
// The bots the plugin owns (now the AI chat app's) become bots of kind "agent", shown AGENT. Each gets a server
// agent's settings from the plugin's agent of the same account, from its configuration's bots (by username) or its
// table Agents_UserAgents (by account): its service (the connection migrated from it keeps the service's ID) and
// model, its instructions as the system prompt, its reasoning and tool turns, and its access. Access the server
// expresses differently (the plugin's lists of channels where a bot answers, its block lists, attribute-based access)
// is narrowed to nobody, or to the members of those channels, and the agent then needs an admin's attention, as does
// one whose service didn't become a connection. While the AI chat app runs, it keeps answering for its own bots.
//
// The bots the app makes afterwards are agents from the start (CreateBot), so that a fresh server shows them as an
// upgraded one does.

const agentsFromBotsMigrationKey = "AntimatterAgentsFromBotsMigrationComplete"

const (
	attentionAgentAccess     = "The AI chat app limited where or for whom this agent answered in a way agents don't express: review who may talk to it."
	attentionAgentConnection = "This agent's service didn't become a model connection: pick its connection and model."
)

// Access levels of the Agents plugin.
const (
	agentsPluginAccessAll   = 0
	agentsPluginAccessAllow = 1
)

// agentsPluginAgentToAIAgent maps one of the plugin's agents to an agent's settings for its account.
func agentsPluginAgentToAIAgent(userID string, src *model.AIAgentsPluginAgent, connectionExists func(id string) bool) *model.AIAgent {
	agent := &model.AIAgent{
		UserId:       userID,
		Scope:        model.AIAgentScopeServer,
		SystemPrompt: strings.TrimSpace(src.CustomInstructions),
		Model:        strings.TrimSpace(src.Model),
		Tools:        model.AIAgentToolPolicy{Enabled: !src.DisableTools, Allowed: []string{}, AutoRun: []string{}},
		Reasoning: model.AIAgentReasoning{
			Enabled:      src.ReasoningEnabled,
			Effort:       strings.ToLower(strings.TrimSpace(src.ReasoningEffort)),
			BudgetTokens: min(max(src.ThinkingBudget, 0), model.AIAgentReasoningMaxBudget),
		},
		MaxToolTurns: min(max(src.MaxToolTurns, 0), model.AIAgentMaxToolTurns),
	}
	switch agent.Reasoning.Effort {
	case "", model.AIAgentReasoningEffortMinimal, model.AIAgentReasoningEffortLow, model.AIAgentReasoningEffortMedium, model.AIAgentReasoningEffortHigh:
	default:
		agent.Reasoning.Effort = model.AIAgentReasoningEffortMedium
	}
	if runes := []rune(agent.SystemPrompt); len(runes) > model.AIAgentSystemPromptMaxRunes {
		agent.SystemPrompt = string(runes[:model.AIAgentSystemPromptMaxRunes])
	}
	if len(agent.Model) > model.AIModelNameMaxLength || strings.ContainsAny(agent.Model, "\r\n") {
		agent.Model = ""
	}
	if model.IsValidId(src.CreatorID) {
		agent.CreatorId = src.CreatorID
	}

	var attention []string
	if model.IsValidId(src.ServiceID) && connectionExists(src.ServiceID) {
		agent.ConnectionId = src.ServiceID
	} else {
		attention = append(attention, attentionAgentConnection)
	}

	ids := func(list []string) []string {
		out := []string{}
		for _, id := range list {
			if model.IsValidId(id) && len(out) < model.AIAgentMaxListItems {
				out = append(out, id)
			}
		}
		return out
	}
	access := model.AIAgentAccess{TeamIDs: []string{}, ChannelIDs: []string{}, Roles: []string{}, UserIDs: []string{}}
	narrowed := false
	switch src.UserAccessLevel {
	case agentsPluginAccessAll:
		access.Everyone = true
	case agentsPluginAccessAllow:
		access.UserIDs = ids(src.UserIDs)
		access.TeamIDs = ids(src.TeamIDs)
	default:
		// Block lists, nobody, attribute-based: nobody, until an admin decides.
		narrowed = true
	}
	switch src.ChannelAccessLevel {
	case agentsPluginAccessAll:
	case agentsPluginAccessAllow:
		// The plugin answered only in those channels; the closest is their members, and only them.
		if access.Everyone || len(access.UserIDs) > 0 || len(access.TeamIDs) > 0 {
			access.Everyone = false
			access.UserIDs, access.TeamIDs = []string{}, []string{}
			access.ChannelIDs = ids(src.ChannelIDs)
		}
		narrowed = true
	default:
		access.Everyone = false
		access.UserIDs, access.TeamIDs, access.ChannelIDs = []string{}, []string{}, []string{}
		narrowed = true
	}
	if narrowed {
		attention = append(attention, attentionAgentAccess)
	}
	agent.Access = access
	agent.NeedsAttention = strings.Join(attention, " ")
	return agent
}

// doAgentsFromBotsMigration runs the migration once. A failure is logged, and it's tried again at the next start.
func (s *Server) doAgentsFromBotsMigration() error {
	var nfErr *store.ErrNotFound
	if _, err := s.Store().System().GetByName(agentsFromBotsMigrationKey); err == nil {
		return nil
	} else if !errors.As(err, &nfErr) {
		return fmt.Errorf("could not query migration: %w", err)
	}
	rctx := request.EmptyContext(s.Log())

	changed, err := s.Store().Bot().SetKindForOwner(model.AIAgentsPluginId, model.BotKindAgent)
	if err != nil {
		s.Log().Error("Couldn't make the Agents plugin's bots agents: they'll be migrated at the next start", mlog.Err(err))
		return nil
	}
	bots, err := s.Store().Bot().GetAll(&model.BotGetOptions{OwnerId: model.AIAgentsPluginId, IncludeDeleted: true, PerPage: 1000})
	if err != nil {
		s.Log().Error("Couldn't list the Agents plugin's bots: they'll be migrated at the next start", mlog.Err(err))
		return nil
	}

	// The plugin's agents: its configuration's bots by username, its table's by account.
	byUsername := map[string]*model.AIAgentsPluginAgent{}
	if raw, cfgErr := s.Store().AIConnection().GetAgentsPluginConfig(rctx); cfgErr != nil {
		s.Log().Warn("Couldn't read the Agents plugin's configuration: its bots become agents without its settings", mlog.Err(cfgErr))
	} else if strings.TrimSpace(raw) != "" {
		var cfg struct {
			Bots []*model.AIAgentsPluginAgent `json:"bots"`
		}
		if jsonErr := json.Unmarshal([]byte(raw), &cfg); jsonErr != nil {
			s.Log().Warn("The Agents plugin's configuration can't be read: its bots become agents without its settings", mlog.Err(jsonErr))
		}
		for _, bot := range cfg.Bots {
			if bot != nil && bot.Name != "" {
				byUsername[strings.ToLower(bot.Name)] = bot
			}
		}
	}
	byUserID := map[string]*model.AIAgentsPluginAgent{}
	if userAgents, uaErr := s.Store().AIAgent().GetAgentsPluginUserAgents(rctx); uaErr != nil {
		s.Log().Warn("Couldn't read the agents people made in the Agents plugin: they become agents without their settings", mlog.Err(uaErr))
	} else {
		for _, ua := range userAgents {
			byUserID[ua.BotUserID] = ua
		}
	}
	connectionExists := func(id string) bool {
		c, getErr := s.Store().AIConnection().Get(rctx, id)
		return getErr == nil && c.DeleteAt == 0
	}

	made := 0
	for _, bot := range bots {
		if _, getErr := s.Store().AIAgent().Get(rctx, bot.UserId); getErr == nil {
			continue
		}
		src := byUserID[bot.UserId]
		if src == nil {
			src = byUsername[strings.ToLower(bot.Username)]
		}
		if src == nil {
			// A bot the plugin keeps for itself (or no longer configures): an agent with nobody to talk to it until an
			// admin sets it up.
			src = &model.AIAgentsPluginAgent{UserAccessLevel: 3}
		}
		agent := agentsPluginAgentToAIAgent(bot.UserId, src, connectionExists)
		if _, saveErr := s.Store().AIAgent().Save(rctx, agent); saveErr != nil {
			s.Log().Error("Couldn't save the settings of an agent migrated from the Agents plugin", mlog.String("user_id", bot.UserId), mlog.String("username", bot.Username), mlog.Err(saveErr))
			continue
		}
		made++
		if agent.NeedsAttention != "" {
			s.Log().Warn("An agent migrated from the Agents plugin needs attention", mlog.String("user_id", bot.UserId), mlog.String("username", bot.Username))
		}
	}

	a := New(ServerConnector(s.Channels()))
	for _, id := range changed {
		a.InvalidateCacheForUser(id)
	}
	a.invalidateAIAgents()
	s.Log().Info("Made the Agents plugin's bots agents", mlog.Int("bots", len(changed)), mlog.Int("settings", made))
	if err := s.Store().System().SaveOrUpdate(&model.System{Name: agentsFromBotsMigrationKey, Value: "true"}); err != nil {
		return fmt.Errorf("failed to mark the agents migration as completed: %w", err)
	}
	return nil
}
