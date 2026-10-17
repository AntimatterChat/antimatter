// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"
	"encoding/json"

	sq "github.com/mattermost/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// SqlAIAgentStore keeps the agents' settings (Antimatter), keyed by their bot account's user ID.
type SqlAIAgentStore struct {
	*SqlStore
}

func newSqlAIAgentStore(sqlStore *SqlStore) store.AIAgentStore {
	return &SqlAIAgentStore{SqlStore: sqlStore}
}

var aiAgentColumns = []string{
	"UserId", "Scope", "External", "ConnectionId", "Model", "SystemPrompt", "Tools", "Reasoning", "MaxToolTurns",
	"Access", "NeedsAttention", "CreatorId", "UpdaterId", "CreateAt", "UpdateAt", "DeleteAt",
}

// aiAgentRow is an agent as stored: its structured fields as JSON text.
type aiAgentRow struct {
	UserId         string
	Scope          string
	External       bool
	ConnectionId   string
	Model          string
	SystemPrompt   string
	Tools          string
	Reasoning      string
	MaxToolTurns   int
	Access         string
	NeedsAttention string
	CreatorId      string
	UpdaterId      string
	CreateAt       int64
	UpdateAt       int64
	DeleteAt       int64
}

func (r *aiAgentRow) toModel() (*model.AIAgent, error) {
	a := &model.AIAgent{
		UserId:         r.UserId,
		Scope:          r.Scope,
		External:       r.External,
		ConnectionId:   r.ConnectionId,
		Model:          r.Model,
		SystemPrompt:   r.SystemPrompt,
		MaxToolTurns:   r.MaxToolTurns,
		NeedsAttention: r.NeedsAttention,
		CreatorId:      r.CreatorId,
		UpdaterId:      r.UpdaterId,
		CreateAt:       r.CreateAt,
		UpdateAt:       r.UpdateAt,
		DeleteAt:       r.DeleteAt,
	}
	for _, field := range []struct {
		raw string
		out any
	}{
		{r.Tools, &a.Tools},
		{r.Reasoning, &a.Reasoning},
		{r.Access, &a.Access},
	} {
		if field.raw == "" {
			continue
		}
		if err := json.Unmarshal([]byte(field.raw), field.out); err != nil {
			return nil, errors.Wrapf(err, "failed to read the agent user_id=%s", r.UserId)
		}
	}
	for _, list := range []*[]string{&a.Tools.Allowed, &a.Tools.AutoRun, &a.Access.TeamIDs, &a.Access.ChannelIDs, &a.Access.Roles, &a.Access.UserIDs} {
		if *list == nil {
			*list = []string{}
		}
	}
	return a, nil
}

func aiAgentValues(a *model.AIAgent) ([]any, error) {
	tools, err := json.Marshal(a.Tools)
	if err != nil {
		return nil, err
	}
	reasoning, err := json.Marshal(a.Reasoning)
	if err != nil {
		return nil, err
	}
	access, err := json.Marshal(a.Access)
	if err != nil {
		return nil, err
	}
	return []any{
		a.UserId, a.Scope, a.External, a.ConnectionId, a.Model, a.SystemPrompt, string(tools), string(reasoning),
		a.MaxToolTurns, string(access), a.NeedsAttention, a.CreatorId, a.UpdaterId, a.CreateAt, a.UpdateAt, a.DeleteAt,
	}, nil
}

func (s *SqlAIAgentStore) Save(rctx request.CTX, agent *model.AIAgent) (*model.AIAgent, error) {
	agent.PreSave()
	if appErr := agent.IsValid(); appErr != nil {
		return nil, appErr
	}
	values, err := aiAgentValues(agent)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode the agent")
	}
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Insert("AIAgents").Columns(aiAgentColumns...).Values(values...)); err != nil {
		return nil, errors.Wrapf(err, "failed to save the agent user_id=%s", agent.UserId)
	}
	return agent, nil
}

func (s *SqlAIAgentStore) Update(rctx request.CTX, agent *model.AIAgent) (*model.AIAgent, error) {
	agent.PreUpdate()
	if appErr := agent.IsValid(); appErr != nil {
		return nil, appErr
	}
	values, err := aiAgentValues(agent)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode the agent")
	}
	builder := s.getQueryBuilder().Update("AIAgents").Where(sq.Eq{"UserId": agent.UserId, "DeleteAt": 0})
	for i, column := range aiAgentColumns {
		switch column {
		case "UserId", "Scope", "External", "CreatorId", "CreateAt", "DeleteAt":
			continue
		}
		builder = builder.Set(column, values[i])
	}
	result, err := s.GetMaster().ExecBuilder(builder)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update the agent user_id=%s", agent.UserId)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return nil, store.NewErrNotFound("AIAgent", agent.UserId)
	}
	return agent, nil
}

func (s *SqlAIAgentStore) Get(rctx request.CTX, userID string) (*model.AIAgent, error) {
	var row aiAgentRow
	query := s.getQueryBuilder().Select(aiAgentColumns...).From("AIAgents").Where(sq.Eq{"UserId": userID})
	if err := s.DBXFromContext(rctx.Context()).GetBuilder(&row, query); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.NewErrNotFound("AIAgent", userID)
		}
		return nil, errors.Wrapf(err, "failed to get the agent user_id=%s", userID)
	}
	return row.toModel()
}

func (s *SqlAIAgentStore) list(rctx request.CTX, where sq.Sqlizer) ([]*model.AIAgent, error) {
	rows := []*aiAgentRow{}
	query := s.getQueryBuilder().Select(aiAgentColumns...).From("AIAgents").Where(where).OrderBy("Scope", "CreateAt", "UserId")
	if err := s.DBXFromContext(rctx.Context()).SelectBuilder(&rows, query); err != nil {
		return nil, errors.Wrap(err, "failed to get the agents")
	}
	out := make([]*model.AIAgent, 0, len(rows))
	for _, r := range rows {
		a, err := r.toModel()
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func (s *SqlAIAgentStore) GetForScopes(rctx request.CTX, scopes []string) ([]*model.AIAgent, error) {
	if len(scopes) == 0 {
		return []*model.AIAgent{}, nil
	}
	return s.list(rctx, sq.Eq{"Scope": scopes, "DeleteAt": 0})
}

func (s *SqlAIAgentStore) GetAll(rctx request.CTX) ([]*model.AIAgent, error) {
	return s.list(rctx, sq.Eq{"DeleteAt": 0})
}

func (s *SqlAIAgentStore) Delete(rctx request.CTX, userID string, deleteAt int64) error {
	builder := s.getQueryBuilder().Update("AIAgents").
		Set("DeleteAt", deleteAt).
		Set("UpdateAt", deleteAt).
		Where(sq.Eq{"UserId": userID, "DeleteAt": 0})
	result, err := s.GetMaster().ExecBuilder(builder)
	if err != nil {
		return errors.Wrapf(err, "failed to delete the agent user_id=%s", userID)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return store.NewErrNotFound("AIAgent", userID)
	}
	return nil
}

// agentsPluginUserAgentRow is a row of the Agents plugin's Agents_UserAgents, its lists as JSON text.
type agentsPluginUserAgentRow struct {
	BotUserID          string `db:"botuserid"`
	CreatorID          string `db:"creatorid"`
	DisplayName        string `db:"displayname"`
	Username           string `db:"username"`
	ServiceID          string `db:"serviceid"`
	CustomInstructions string `db:"custominstructions"`
	ChannelAccessLevel int    `db:"channelaccesslevel"`
	ChannelIDs         string `db:"channelids"`
	UserAccessLevel    int    `db:"useraccesslevel"`
	UserIDs            string `db:"userids"`
	TeamIDs            string `db:"teamids"`
	Model              string `db:"model"`
	DisableTools       bool   `db:"disabletools"`
	ReasoningEnabled   bool   `db:"reasoningenabled"`
	ReasoningEffort    string `db:"reasoningeffort"`
	ThinkingBudget     int    `db:"thinkingbudget"`
	MaxToolTurns       int    `db:"maxtoolturns"`
}

func (s *SqlAIAgentStore) GetAgentsPluginUserAgents(rctx request.CTX) ([]*model.AIAgentsPluginAgent, error) {
	var table sql.NullString
	if err := s.GetMaster().Get(&table, "SELECT to_regclass('agents_useragents')::text"); err != nil {
		return nil, errors.Wrap(err, "failed to look for the Agents plugin's agents")
	}
	if !table.Valid || table.String == "" {
		return []*model.AIAgentsPluginAgent{}, nil
	}
	// MaxToolTurns came with a later version of the plugin's table: read it only when it's there.
	var hasMaxToolTurns bool
	if err := s.GetMaster().Get(&hasMaxToolTurns, "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'agents_useragents' AND column_name = 'maxtoolturns')"); err != nil {
		return nil, errors.Wrap(err, "failed to read the Agents plugin's agents")
	}
	maxToolTurns := "0 AS MaxToolTurns"
	if hasMaxToolTurns {
		maxToolTurns = "MaxToolTurns"
	}
	rows := []*agentsPluginUserAgentRow{}
	if err := s.GetMaster().Select(&rows, `SELECT BotUserID, CreatorID, DisplayName, Username, ServiceID, CustomInstructions,
			ChannelAccessLevel, ChannelIDs, UserAccessLevel, UserIDs, TeamIDs, Model, DisableTools,
			ReasoningEnabled, ReasoningEffort, ThinkingBudget, `+maxToolTurns+`
		FROM Agents_UserAgents WHERE DeleteAt = 0 ORDER BY CreateAt`); err != nil {
		return nil, errors.Wrap(err, "failed to read the Agents plugin's agents")
	}
	out := make([]*model.AIAgentsPluginAgent, 0, len(rows))
	for _, r := range rows {
		agent := &model.AIAgentsPluginAgent{
			BotUserID:          r.BotUserID,
			Name:               r.Username,
			DisplayName:        r.DisplayName,
			CustomInstructions: r.CustomInstructions,
			ServiceID:          r.ServiceID,
			Model:              r.Model,
			DisableTools:       r.DisableTools,
			ChannelAccessLevel: r.ChannelAccessLevel,
			UserAccessLevel:    r.UserAccessLevel,
			ReasoningEnabled:   r.ReasoningEnabled,
			ReasoningEffort:    r.ReasoningEffort,
			ThinkingBudget:     r.ThinkingBudget,
			MaxToolTurns:       r.MaxToolTurns,
			CreatorID:          r.CreatorID,
		}
		for _, list := range []struct {
			raw string
			out *[]string
		}{{r.ChannelIDs, &agent.ChannelIDs}, {r.UserIDs, &agent.UserIDs}, {r.TeamIDs, &agent.TeamIDs}} {
			if list.raw == "" || list.raw == "[]" {
				continue
			}
			if err := json.Unmarshal([]byte(list.raw), list.out); err != nil {
				// A list that can't be read gives no access rather than more: the migration makes the agent
				// need attention when its level asked for a list.
				*list.out = nil
			}
		}
		out = append(out, agent)
	}
	return out, nil
}
