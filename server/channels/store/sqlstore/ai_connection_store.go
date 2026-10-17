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

// SqlAIConnectionStore keeps the model connections (Antimatter) and their usage. Keys and extra headers arrive
// sealed by the app: the store never sees them in clear.
type SqlAIConnectionStore struct {
	*SqlStore
}

func newSqlAIConnectionStore(sqlStore *SqlStore) store.AIConnectionStore {
	return &SqlAIConnectionStore{SqlStore: sqlStore}
}

var aiConnectionColumns = []string{
	"Id", "Name", "Scope", "Provider", "BaseURL", "APIKeySealed", "APIKeyLast4", "ExtraHeadersSealed",
	"ExtraHeaderNames", "Models", "DecisionsDialect", "Limits", "Sharing", "Audience", "NeedsAttention",
	"CreatorId", "UpdaterId", "CreateAt", "UpdateAt", "DeleteAt",
}

// aiConnectionRow is a connection as stored: its structured fields as JSON text.
type aiConnectionRow struct {
	Id                 string
	Name               string
	Scope              string
	Provider           string
	BaseURL            string
	APIKeySealed       string
	APIKeyLast4        string
	ExtraHeadersSealed string
	ExtraHeaderNames   string
	Models             string
	DecisionsDialect   string
	Limits             string
	Sharing            string
	Audience           string
	NeedsAttention     string
	CreatorId          string
	UpdaterId          string
	CreateAt           int64
	UpdateAt           int64
	DeleteAt           int64
}

func (r *aiConnectionRow) toModel() (*model.AIConnection, error) {
	c := &model.AIConnection{
		Id:                 r.Id,
		Name:               r.Name,
		Scope:              r.Scope,
		Provider:           r.Provider,
		BaseURL:            r.BaseURL,
		APIKeySealed:       r.APIKeySealed,
		APIKeyLast4:        r.APIKeyLast4,
		ExtraHeadersSealed: r.ExtraHeadersSealed,
		DecisionsDialect:   r.DecisionsDialect,
		NeedsAttention:     r.NeedsAttention,
		CreatorId:          r.CreatorId,
		UpdaterId:          r.UpdaterId,
		CreateAt:           r.CreateAt,
		UpdateAt:           r.UpdateAt,
		DeleteAt:           r.DeleteAt,
	}
	for _, field := range []struct {
		raw string
		out any
	}{
		{r.ExtraHeaderNames, &c.ExtraHeaderNames},
		{r.Models, &c.Models},
		{r.Limits, &c.Limits},
		{r.Sharing, &c.Sharing},
		{r.Audience, &c.Audience},
	} {
		if field.raw == "" {
			continue
		}
		if err := json.Unmarshal([]byte(field.raw), field.out); err != nil {
			return nil, errors.Wrapf(err, "failed to read the model connection id=%s", r.Id)
		}
	}
	if c.ExtraHeaderNames == nil {
		c.ExtraHeaderNames = []string{}
	}
	if c.Sharing.TeamIDs == nil {
		c.Sharing.TeamIDs = []string{}
	}
	if c.Audience.GroupIDs == nil {
		c.Audience.GroupIDs = []string{}
	}
	if c.Audience.Roles == nil {
		c.Audience.Roles = []string{}
	}
	c.APIKeySet = c.APIKeySealed != ""
	return c, nil
}

func aiConnectionValues(c *model.AIConnection) ([]any, error) {
	encode := func(v any) (string, error) {
		data, err := json.Marshal(v)
		return string(data), err
	}
	headerNames, err := encode(c.ExtraHeaderNames)
	if err != nil {
		return nil, err
	}
	models, err := encode(c.Models)
	if err != nil {
		return nil, err
	}
	limits, err := encode(c.Limits)
	if err != nil {
		return nil, err
	}
	sharing, err := encode(c.Sharing)
	if err != nil {
		return nil, err
	}
	audience, err := encode(c.Audience)
	if err != nil {
		return nil, err
	}
	return []any{
		c.Id, c.Name, c.Scope, c.Provider, c.BaseURL, c.APIKeySealed, c.APIKeyLast4, c.ExtraHeadersSealed,
		headerNames, models, c.DecisionsDialect, limits, sharing, audience, c.NeedsAttention,
		c.CreatorId, c.UpdaterId, c.CreateAt, c.UpdateAt, c.DeleteAt,
	}, nil
}

func (s *SqlAIConnectionStore) Save(rctx request.CTX, connection *model.AIConnection) (*model.AIConnection, error) {
	connection.PreSave()
	if appErr := connection.IsValid(); appErr != nil {
		return nil, appErr
	}
	values, err := aiConnectionValues(connection)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode the model connection")
	}
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Insert("AIConnections").Columns(aiConnectionColumns...).Values(values...)); err != nil {
		return nil, errors.Wrapf(err, "failed to save the model connection id=%s", connection.Id)
	}
	return connection, nil
}

func (s *SqlAIConnectionStore) Update(rctx request.CTX, connection *model.AIConnection) (*model.AIConnection, error) {
	connection.PreUpdate()
	if appErr := connection.IsValid(); appErr != nil {
		return nil, appErr
	}
	values, err := aiConnectionValues(connection)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode the model connection")
	}
	builder := s.getQueryBuilder().Update("AIConnections").Where(sq.Eq{"Id": connection.Id, "DeleteAt": 0})
	// Everything but the ID, the scope and who made it when.
	for i, column := range aiConnectionColumns {
		switch column {
		case "Id", "Scope", "CreatorId", "CreateAt":
			continue
		}
		builder = builder.Set(column, values[i])
	}
	result, err := s.GetMaster().ExecBuilder(builder)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update the model connection id=%s", connection.Id)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return nil, store.NewErrNotFound("AIConnection", connection.Id)
	}
	return connection, nil
}

func (s *SqlAIConnectionStore) Get(rctx request.CTX, id string) (*model.AIConnection, error) {
	var row aiConnectionRow
	query := s.getQueryBuilder().Select(aiConnectionColumns...).From("AIConnections").Where(sq.Eq{"Id": id})
	if err := s.DBXFromContext(rctx.Context()).GetBuilder(&row, query); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.NewErrNotFound("AIConnection", id)
		}
		return nil, errors.Wrapf(err, "failed to get the model connection id=%s", id)
	}
	return row.toModel()
}

func (s *SqlAIConnectionStore) list(rctx request.CTX, where sq.Sqlizer) ([]*model.AIConnection, error) {
	rows := []*aiConnectionRow{}
	query := s.getQueryBuilder().Select(aiConnectionColumns...).From("AIConnections").Where(where).OrderBy("Scope", "Name", "Id")
	if err := s.DBXFromContext(rctx.Context()).SelectBuilder(&rows, query); err != nil {
		return nil, errors.Wrap(err, "failed to get the model connections")
	}
	out := make([]*model.AIConnection, 0, len(rows))
	for _, r := range rows {
		c, err := r.toModel()
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func (s *SqlAIConnectionStore) GetForScopes(rctx request.CTX, scopes []string) ([]*model.AIConnection, error) {
	if len(scopes) == 0 {
		return []*model.AIConnection{}, nil
	}
	return s.list(rctx, sq.Eq{"Scope": scopes, "DeleteAt": 0})
}

func (s *SqlAIConnectionStore) GetAll(rctx request.CTX) ([]*model.AIConnection, error) {
	return s.list(rctx, sq.Eq{"DeleteAt": 0})
}

func (s *SqlAIConnectionStore) Delete(rctx request.CTX, id string, deleteAt int64) error {
	builder := s.getQueryBuilder().Update("AIConnections").
		Set("DeleteAt", deleteAt).
		Set("UpdateAt", deleteAt).
		Set("APIKeySealed", "").
		Set("ExtraHeadersSealed", "").
		Where(sq.Eq{"Id": id, "DeleteAt": 0})
	result, err := s.GetMaster().ExecBuilder(builder)
	if err != nil {
		return errors.Wrapf(err, "failed to delete the model connection id=%s", id)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return store.NewErrNotFound("AIConnection", id)
	}
	return nil
}

func (s *SqlAIConnectionStore) AddUsage(rctx request.CTX, usage *model.AIUsage) error {
	builder := s.getQueryBuilder().
		Insert("AIUsage").
		Columns("ConnectionId", "UserId", "Purpose", "Day", "Requests", "InputTokens", "OutputTokens").
		Values(usage.ConnectionId, usage.UserId, usage.Purpose, usage.Day, usage.Requests, usage.InputTokens, usage.OutputTokens).
		SuffixExpr(sq.Expr("ON CONFLICT (ConnectionId, UserId, Purpose, Day) DO UPDATE SET " +
			"Requests = AIUsage.Requests + EXCLUDED.Requests, " +
			"InputTokens = AIUsage.InputTokens + EXCLUDED.InputTokens, " +
			"OutputTokens = AIUsage.OutputTokens + EXCLUDED.OutputTokens"))
	if _, err := s.GetMaster().ExecBuilder(builder); err != nil {
		return errors.Wrapf(err, "failed to count the usage of the model connection id=%s", usage.ConnectionId)
	}
	return nil
}

func (s *SqlAIConnectionStore) totals(rctx request.CTX, where sq.Sqlizer) (model.AIUsageTotals, error) {
	var totals model.AIUsageTotals
	query := s.getQueryBuilder().
		Select("COALESCE(SUM(Requests), 0) AS Requests", "COALESCE(SUM(InputTokens), 0) AS InputTokens", "COALESCE(SUM(OutputTokens), 0) AS OutputTokens").
		From("AIUsage").
		Where(where)
	// The budgets are checked before each call: read the master, so that a call just made counts.
	if err := s.GetMaster().GetBuilder(&totals, query); err != nil {
		return totals, errors.Wrap(err, "failed to add up the usage of a model connection")
	}
	return totals, nil
}

func (s *SqlAIConnectionStore) GetUsageTotals(rctx request.CTX, connectionID string, sinceDay int) (model.AIUsageTotals, error) {
	return s.totals(rctx, sq.And{sq.Eq{"ConnectionId": connectionID}, sq.GtOrEq{"Day": sinceDay}})
}

func (s *SqlAIConnectionStore) GetUserUsageTotals(rctx request.CTX, connectionID, userID string, sinceDay int) (model.AIUsageTotals, error) {
	return s.totals(rctx, sq.And{sq.Eq{"ConnectionId": connectionID, "UserId": userID}, sq.GtOrEq{"Day": sinceDay}})
}

func (s *SqlAIConnectionStore) GetUsage(rctx request.CTX, connectionID string, sinceDay int) ([]*model.AIUsage, error) {
	usage := []*model.AIUsage{}
	query := s.getQueryBuilder().
		Select("ConnectionId", "UserId", "Purpose", "Day", "Requests", "InputTokens", "OutputTokens").
		From("AIUsage").
		Where(sq.And{sq.Eq{"ConnectionId": connectionID}, sq.GtOrEq{"Day": sinceDay}}).
		OrderBy("Day", "Purpose", "UserId")
	if err := s.DBXFromContext(rctx.Context()).SelectBuilder(&usage, query); err != nil {
		return nil, errors.Wrapf(err, "failed to get the usage of the model connection id=%s", connectionID)
	}
	return usage, nil
}

func (s *SqlAIConnectionStore) GetAgentsPluginConfig(rctx request.CTX) (string, error) {
	var table sql.NullString
	if err := s.GetMaster().Get(&table, "SELECT to_regclass('agents_confighistory')::text"); err != nil {
		return "", errors.Wrap(err, "failed to look for the Agents plugin's configuration")
	}
	if !table.Valid || table.String == "" {
		return "", nil
	}
	var config string
	err := s.GetMaster().Get(&config, "SELECT Config FROM Agents_ConfigHistory WHERE Active = true LIMIT 1")
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", errors.Wrap(err, "failed to read the Agents plugin's configuration")
	}
	return config, nil
}
