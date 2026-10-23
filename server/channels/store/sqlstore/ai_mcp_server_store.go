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

// SqlAIMCPServerStore keeps the MCP servers agents may use (Antimatter), and the people's sign-ins to them. Their
// secrets come sealed.
type SqlAIMCPServerStore struct {
	*SqlStore
}

func newSqlAIMCPServerStore(sqlStore *SqlStore) store.AIMCPServerStore {
	return &SqlAIMCPServerStore{SqlStore: sqlStore}
}

var aiMCPServerColumns = []string{
	"Id", "Name", "Slug", "Scope", "URL", "Auth", "OAuthClientId", "OAuthClientSecretSealed", "OAuthScopes",
	"OAuthRegistered", "HeadersSealed", "HeaderNames", "Sharing", "NeedsAttention", "CreatorId", "UpdaterId",
	"CreateAt", "UpdateAt", "DeleteAt",
}

// aiMCPServerRow is an MCP server as stored: its lists as JSON text.
type aiMCPServerRow struct {
	Id                      string
	Name                    string
	Slug                    string
	Scope                   string
	URL                     string `db:"url"`
	Auth                    string
	OAuthClientId           string `db:"oauthclientid"`
	OAuthClientSecretSealed string `db:"oauthclientsecretsealed"`
	OAuthScopes             string `db:"oauthscopes"`
	OAuthRegistered         bool   `db:"oauthregistered"`
	HeadersSealed           string
	HeaderNames             string
	Sharing                 string
	NeedsAttention          string
	CreatorId               string
	UpdaterId               string
	CreateAt                int64
	UpdateAt                int64
	DeleteAt                int64
}

func (r *aiMCPServerRow) toModel() (*model.AIMCPServer, error) {
	s := &model.AIMCPServer{
		Id:                      r.Id,
		Name:                    r.Name,
		Slug:                    r.Slug,
		Scope:                   r.Scope,
		URL:                     r.URL,
		Auth:                    r.Auth,
		OAuthClientId:           r.OAuthClientId,
		OAuthClientSecretSealed: r.OAuthClientSecretSealed,
		OAuthClientSecretSet:    r.OAuthClientSecretSealed != "",
		OAuthScopes:             r.OAuthScopes,
		OAuthRegistered:         r.OAuthRegistered,
		HeadersSealed:           r.HeadersSealed,
		NeedsAttention:          r.NeedsAttention,
		CreatorId:               r.CreatorId,
		UpdaterId:               r.UpdaterId,
		CreateAt:                r.CreateAt,
		UpdateAt:                r.UpdateAt,
		DeleteAt:                r.DeleteAt,
	}
	if r.HeaderNames != "" {
		if err := json.Unmarshal([]byte(r.HeaderNames), &s.HeaderNames); err != nil {
			return nil, errors.Wrapf(err, "failed to read the MCP server id=%s", r.Id)
		}
	}
	if r.Sharing != "" {
		if err := json.Unmarshal([]byte(r.Sharing), &s.Sharing); err != nil {
			return nil, errors.Wrapf(err, "failed to read the MCP server id=%s", r.Id)
		}
	}
	if s.HeaderNames == nil {
		s.HeaderNames = []string{}
	}
	if s.Sharing.TeamIDs == nil {
		s.Sharing.TeamIDs = []string{}
	}
	return s, nil
}

func aiMCPServerValues(s *model.AIMCPServer) ([]any, error) {
	headerNames, err := json.Marshal(s.HeaderNames)
	if err != nil {
		return nil, err
	}
	sharing, err := json.Marshal(s.Sharing)
	if err != nil {
		return nil, err
	}
	return []any{
		s.Id, s.Name, s.Slug, s.Scope, s.URL, s.Auth, s.OAuthClientId, s.OAuthClientSecretSealed, s.OAuthScopes,
		s.OAuthRegistered, s.HeadersSealed, string(headerNames), string(sharing), s.NeedsAttention, s.CreatorId,
		s.UpdaterId, s.CreateAt, s.UpdateAt, s.DeleteAt,
	}, nil
}

func (s *SqlAIMCPServerStore) Save(rctx request.CTX, server *model.AIMCPServer) (*model.AIMCPServer, error) {
	server.PreSave()
	if appErr := server.IsValid(); appErr != nil {
		return nil, appErr
	}
	values, err := aiMCPServerValues(server)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode the MCP server")
	}
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Insert("AIMCPServers").Columns(aiMCPServerColumns...).Values(values...)); err != nil {
		if IsUniqueConstraintError(err, []string{"idx_aimcpservers_slug", "aimcpservers_pkey"}) {
			return nil, store.NewErrConflict("AIMCPServer", err, "slug="+server.Slug)
		}
		return nil, errors.Wrapf(err, "failed to save the MCP server id=%s", server.Id)
	}
	return server, nil
}

func (s *SqlAIMCPServerStore) Update(rctx request.CTX, server *model.AIMCPServer) (*model.AIMCPServer, error) {
	server.PreUpdate()
	if appErr := server.IsValid(); appErr != nil {
		return nil, appErr
	}
	values, err := aiMCPServerValues(server)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode the MCP server")
	}
	builder := s.getQueryBuilder().Update("AIMCPServers").Where(sq.Eq{"Id": server.Id, "DeleteAt": 0})
	for i, column := range aiMCPServerColumns {
		switch column {
		case "Id", "Slug", "Scope", "CreatorId", "CreateAt", "DeleteAt":
			continue
		}
		builder = builder.Set(column, values[i])
	}
	result, err := s.GetMaster().ExecBuilder(builder)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update the MCP server id=%s", server.Id)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return nil, store.NewErrNotFound("AIMCPServer", server.Id)
	}
	return server, nil
}

func (s *SqlAIMCPServerStore) Get(rctx request.CTX, id string) (*model.AIMCPServer, error) {
	var row aiMCPServerRow
	query := s.getQueryBuilder().Select(aiMCPServerColumns...).From("AIMCPServers").Where(sq.Eq{"Id": id})
	if err := s.DBXFromContext(rctx.Context()).GetBuilder(&row, query); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.NewErrNotFound("AIMCPServer", id)
		}
		return nil, errors.Wrapf(err, "failed to get the MCP server id=%s", id)
	}
	return row.toModel()
}

func (s *SqlAIMCPServerStore) list(rctx request.CTX, where sq.Sqlizer) ([]*model.AIMCPServer, error) {
	rows := []*aiMCPServerRow{}
	query := s.getQueryBuilder().Select(aiMCPServerColumns...).From("AIMCPServers").Where(where).OrderBy("Scope", "Name", "Id")
	if err := s.DBXFromContext(rctx.Context()).SelectBuilder(&rows, query); err != nil {
		return nil, errors.Wrap(err, "failed to get the MCP servers")
	}
	out := make([]*model.AIMCPServer, 0, len(rows))
	for _, r := range rows {
		server, err := r.toModel()
		if err != nil {
			return nil, err
		}
		out = append(out, server)
	}
	return out, nil
}

func (s *SqlAIMCPServerStore) GetForScopes(rctx request.CTX, scopes []string) ([]*model.AIMCPServer, error) {
	if len(scopes) == 0 {
		return []*model.AIMCPServer{}, nil
	}
	return s.list(rctx, sq.Eq{"Scope": scopes, "DeleteAt": 0})
}

func (s *SqlAIMCPServerStore) GetAll(rctx request.CTX) ([]*model.AIMCPServer, error) {
	return s.list(rctx, sq.Eq{"DeleteAt": 0})
}

func (s *SqlAIMCPServerStore) Delete(rctx request.CTX, id string, deleteAt int64) (err error) {
	transaction, err := s.GetMaster().Begin()
	if err != nil {
		return errors.Wrap(err, "begin_transaction")
	}
	defer finalizeTransactionX(transaction, &err)

	result, err := transaction.ExecBuilder(s.getQueryBuilder().Update("AIMCPServers").
		Set("DeleteAt", deleteAt).
		Set("UpdateAt", deleteAt).
		// A deleted server's secrets are forgotten with it.
		Set("OAuthClientSecretSealed", "").
		Set("HeadersSealed", "").
		Where(sq.Eq{"Id": id, "DeleteAt": 0}))
	if err != nil {
		return errors.Wrapf(err, "failed to delete the MCP server id=%s", id)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return store.NewErrNotFound("AIMCPServer", id)
	}
	if _, err = transaction.ExecBuilder(s.getQueryBuilder().Delete("AIMCPUserTokens").Where(sq.Eq{"ServerId": id})); err != nil {
		return errors.Wrapf(err, "failed to forget the sign-ins to the MCP server id=%s", id)
	}
	if err = transaction.Commit(); err != nil {
		return errors.Wrap(err, "commit_transaction")
	}
	return nil
}

func (s *SqlAIMCPServerStore) SaveUserToken(rctx request.CTX, token *model.AIMCPUserToken) error {
	if !model.IsValidId(token.ServerId) || !model.IsValidId(token.UserId) || token.TokenSealed == "" {
		return store.NewErrInvalidInput("AIMCPUserToken", "ServerId/UserId/TokenSealed", "")
	}
	now := model.GetMillis()
	if token.CreateAt == 0 {
		token.CreateAt = now
	}
	token.UpdateAt = now
	query := s.getQueryBuilder().Insert("AIMCPUserTokens").
		Columns("ServerId", "UserId", "TokenSealed", "ExpiresAt", "CreateAt", "UpdateAt").
		Values(token.ServerId, token.UserId, token.TokenSealed, token.ExpiresAt, token.CreateAt, token.UpdateAt).
		SuffixExpr(sq.Expr("ON CONFLICT (ServerId, UserId) DO UPDATE SET TokenSealed = EXCLUDED.TokenSealed, ExpiresAt = EXCLUDED.ExpiresAt, UpdateAt = EXCLUDED.UpdateAt"))
	if _, err := s.GetMaster().ExecBuilder(query); err != nil {
		return errors.Wrapf(err, "failed to save a sign-in to the MCP server id=%s", token.ServerId)
	}
	return nil
}

func (s *SqlAIMCPServerStore) GetUserToken(rctx request.CTX, serverID, userID string) (*model.AIMCPUserToken, error) {
	var token model.AIMCPUserToken
	query := s.getQueryBuilder().
		Select("ServerId", "UserId", "TokenSealed", "ExpiresAt", "CreateAt", "UpdateAt").
		From("AIMCPUserTokens").
		Where(sq.Eq{"ServerId": serverID, "UserId": userID})
	// The person's own sign-in only: both keys, always.
	if err := s.GetMaster().GetBuilder(&token, query); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.NewErrNotFound("AIMCPUserToken", serverID)
		}
		return nil, errors.Wrapf(err, "failed to get a sign-in to the MCP server id=%s", serverID)
	}
	return &token, nil
}

func (s *SqlAIMCPServerStore) GetUserTokenServerIds(rctx request.CTX, userID string) ([]string, error) {
	ids := []string{}
	query := s.getQueryBuilder().Select("ServerId").From("AIMCPUserTokens").Where(sq.Eq{"UserId": userID}).OrderBy("ServerId")
	if err := s.DBXFromContext(rctx.Context()).SelectBuilder(&ids, query); err != nil {
		return nil, errors.Wrap(err, "failed to get a person's sign-ins to MCP servers")
	}
	return ids, nil
}

func (s *SqlAIMCPServerStore) DeleteUserToken(rctx request.CTX, serverID, userID string) error {
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("AIMCPUserTokens").Where(sq.Eq{"ServerId": serverID, "UserId": userID})); err != nil {
		return errors.Wrapf(err, "failed to forget a sign-in to the MCP server id=%s", serverID)
	}
	return nil
}

func (s *SqlAIMCPServerStore) DeleteUserTokensForServer(rctx request.CTX, serverID string) error {
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("AIMCPUserTokens").Where(sq.Eq{"ServerId": serverID})); err != nil {
		return errors.Wrapf(err, "failed to forget the sign-ins to the MCP server id=%s", serverID)
	}
	return nil
}

func (s *SqlAIMCPServerStore) DeleteUserTokensForUser(rctx request.CTX, userID string) error {
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("AIMCPUserTokens").Where(sq.Eq{"UserId": userID})); err != nil {
		return errors.Wrap(err, "failed to forget a person's sign-ins to MCP servers")
	}
	return nil
}
