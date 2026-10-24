// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"

	sq "github.com/mattermost/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// SqlModerationStore keeps what a team's moderators and those who invite use (Antimatter): timeouts, post counts,
// bans and invite links.
type SqlModerationStore struct {
	*SqlStore
}

func newSqlModerationStore(sqlStore *SqlStore) store.ModerationStore {
	return &SqlModerationStore{SqlStore: sqlStore}
}

// GetTimeouts gives a team's timeouts that still hold at a time.
func (s *SqlModerationStore) GetTimeouts(rctx request.CTX, teamID string, now int64) ([]*model.MemberTimeout, error) {
	query := s.getQueryBuilder().
		Select("TeamId", "UserId", "Until", "Reason", "CreatedBy", "CreateAt").
		From("MemberTimeouts").
		Where(sq.And{sq.Eq{"TeamId": teamID}, sq.Gt{"Until": now}}).
		OrderBy("UserId")
	timeouts := []*model.MemberTimeout{}
	if err := s.DBXFromContext(rctx.Context()).SelectBuilder(&timeouts, query); err != nil {
		return nil, errors.Wrapf(err, "failed to get timeouts for team=%s", teamID)
	}
	return timeouts, nil
}

// SaveTimeout times someone out, in place of a timeout they had.
func (s *SqlModerationStore) SaveTimeout(rctx request.CTX, timeout *model.MemberTimeout) (*model.MemberTimeout, error) {
	timeout.CreateAt = model.GetMillis()
	builder := s.getQueryBuilder().
		Insert("MemberTimeouts").
		Columns("TeamId", "UserId", "Until", "Reason", "CreatedBy", "CreateAt").
		Values(timeout.TeamId, timeout.UserId, timeout.Until, timeout.Reason, timeout.CreatedBy, timeout.CreateAt).
		SuffixExpr(sq.Expr("ON CONFLICT (TeamId, UserId) DO UPDATE SET Until = EXCLUDED.Until, Reason = EXCLUDED.Reason, CreatedBy = EXCLUDED.CreatedBy, CreateAt = EXCLUDED.CreateAt"))
	if _, err := s.GetMaster().ExecBuilder(builder); err != nil {
		return nil, errors.Wrapf(err, "failed to save timeout for team=%s user=%s", timeout.TeamId, timeout.UserId)
	}
	return timeout, nil
}

func (s *SqlModerationStore) DeleteTimeout(rctx request.CTX, teamID, userID string) error {
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("MemberTimeouts").Where(sq.Eq{"TeamId": teamID, "UserId": userID})); err != nil {
		return errors.Wrapf(err, "failed to delete timeout for team=%s user=%s", teamID, userID)
	}
	return nil
}

func (s *SqlModerationStore) DeleteTimeoutsForTeam(rctx request.CTX, teamID string) error {
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("MemberTimeouts").Where(sq.Eq{"TeamId": teamID})); err != nil {
		return errors.Wrapf(err, "failed to delete timeouts for team=%s", teamID)
	}
	return nil
}

func (s *SqlModerationStore) PermanentDeleteTimeoutsByUser(rctx request.CTX, userID string) error {
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("MemberTimeouts").Where(sq.Eq{"UserId": userID})); err != nil {
		return errors.Wrapf(err, "failed to delete the timeouts of user=%s", userID)
	}
	return nil
}

// CountPosts counts someone's messages in a team's channels since a time, their system messages left out.
func (s *SqlModerationStore) CountPosts(rctx request.CTX, teamID, userID string, since int64) (int64, error) {
	query := s.getQueryBuilder().
		Select("COUNT(*)").
		From("Posts p").
		Join("Channels c ON c.Id = p.ChannelId").
		Where(sq.And{
			sq.Eq{"p.UserId": userID, "c.TeamId": teamID, "p.DeleteAt": 0},
			sq.GtOrEq{"p.CreateAt": since},
			sq.NotLike{"p.Type": model.PostSystemMessagePrefix + "%"},
		})
	var count int64
	if err := s.DBXFromContext(rctx.Context()).GetBuilder(&count, query); err != nil {
		return 0, errors.Wrapf(err, "failed to count posts for team=%s user=%s", teamID, userID)
	}
	return count, nil
}

// GetPostIdsInRange gives up to limit of someone's messages in a team's channels created in [since, before), the
// oldest first, their system messages and deleted ones left out. Direct and group messages have no team: never
// theirs.
func (s *SqlModerationStore) GetPostIdsInRange(rctx request.CTX, teamID, userID string, since, before int64, limit int) ([]string, error) {
	query := s.getQueryBuilder().
		Select("p.Id").
		From("Posts p").
		Join("Channels c ON c.Id = p.ChannelId").
		Where(sq.And{
			sq.Eq{"p.UserId": userID, "c.TeamId": teamID, "p.DeleteAt": 0},
			sq.GtOrEq{"p.CreateAt": since},
			sq.Lt{"p.CreateAt": before},
			sq.NotLike{"p.Type": model.PostSystemMessagePrefix + "%"},
		}).
		OrderBy("p.CreateAt", "p.Id").
		Limit(uint64(limit))
	ids := []string{}
	if err := s.GetMaster().SelectBuilder(&ids, query); err != nil {
		return nil, errors.Wrapf(err, "failed to get posts for team=%s user=%s", teamID, userID)
	}
	return ids, nil
}

var memberBanColumns = []string{"TeamId", "UserId", "Reason", "CreatedBy", "CreateAt", "ExpireAt"}

// activeBan is where a ban still holds at a time: it has no end, or ends later.
func activeBan(now int64) sq.Sqlizer {
	return sq.Or{sq.Eq{"ExpireAt": 0}, sq.Gt{"ExpireAt": now}}
}

// GetBan gives someone's ban from a team that still holds, from the master: it decides who may come back in, right
// after the ban.
func (s *SqlModerationStore) GetBan(rctx request.CTX, teamID, userID string, now int64) (*model.MemberBan, error) {
	query := s.getQueryBuilder().
		Select(memberBanColumns...).
		From("MemberBans").
		Where(sq.And{sq.Eq{"TeamId": teamID, "UserId": userID}, activeBan(now)})
	var ban model.MemberBan
	if err := s.GetMaster().GetBuilder(&ban, query); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.NewErrNotFound("MemberBan", teamID+"/"+userID)
		}
		return nil, errors.Wrapf(err, "failed to get the ban for team=%s user=%s", teamID, userID)
	}
	return &ban, nil
}

func (s *SqlModerationStore) GetBans(rctx request.CTX, teamID string, now int64) ([]*model.MemberBan, error) {
	query := s.getQueryBuilder().
		Select(memberBanColumns...).
		From("MemberBans").
		Where(sq.And{sq.Eq{"TeamId": teamID}, activeBan(now)}).
		OrderBy("CreateAt DESC", "UserId")
	bans := []*model.MemberBan{}
	if err := s.DBXFromContext(rctx.Context()).SelectBuilder(&bans, query); err != nil {
		return nil, errors.Wrapf(err, "failed to get bans for team=%s", teamID)
	}
	return bans, nil
}

func (s *SqlModerationStore) SaveBan(rctx request.CTX, ban *model.MemberBan) (*model.MemberBan, error) {
	ban.CreateAt = model.GetMillis()
	builder := s.getQueryBuilder().
		Insert("MemberBans").
		Columns(memberBanColumns...).
		Values(ban.TeamId, ban.UserId, ban.Reason, ban.CreatedBy, ban.CreateAt, ban.ExpireAt).
		SuffixExpr(sq.Expr("ON CONFLICT (TeamId, UserId) DO UPDATE SET Reason = EXCLUDED.Reason, CreatedBy = EXCLUDED.CreatedBy, CreateAt = EXCLUDED.CreateAt, ExpireAt = EXCLUDED.ExpireAt"))
	if _, err := s.GetMaster().ExecBuilder(builder); err != nil {
		return nil, errors.Wrapf(err, "failed to save ban for team=%s user=%s", ban.TeamId, ban.UserId)
	}
	return ban, nil
}

func (s *SqlModerationStore) DeleteBan(rctx request.CTX, teamID, userID string) error {
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("MemberBans").Where(sq.Eq{"TeamId": teamID, "UserId": userID})); err != nil {
		return errors.Wrapf(err, "failed to delete ban for team=%s user=%s", teamID, userID)
	}
	return nil
}

func (s *SqlModerationStore) DeleteBansForTeam(rctx request.CTX, teamID string) error {
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("MemberBans").Where(sq.Eq{"TeamId": teamID})); err != nil {
		return errors.Wrapf(err, "failed to delete bans for team=%s", teamID)
	}
	return nil
}

func (s *SqlModerationStore) PermanentDeleteBansByUser(rctx request.CTX, userID string) error {
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("MemberBans").Where(sq.Eq{"UserId": userID})); err != nil {
		return errors.Wrapf(err, "failed to delete the bans of user=%s", userID)
	}
	return nil
}

var teamInviteLinkColumns = []string{"Id", "TeamId", "CreatorId", "CreateAt", "ExpireAt", "MaxUses", "Uses"}

func (s *SqlModerationStore) SaveInviteLink(rctx request.CTX, link *model.TeamInviteLink) (*model.TeamInviteLink, error) {
	builder := s.getQueryBuilder().
		Insert("TeamInviteLinks").
		Columns(teamInviteLinkColumns...).
		Values(link.Id, link.TeamId, link.CreatorId, link.CreateAt, link.ExpireAt, link.MaxUses, link.Uses)
	if _, err := s.GetMaster().ExecBuilder(builder); err != nil {
		return nil, errors.Wrapf(err, "failed to save invite link for team=%s", link.TeamId)
	}
	return link, nil
}

// GetInviteLink gives a link from the master: a link just revoked or used up must not let one more person in.
func (s *SqlModerationStore) GetInviteLink(rctx request.CTX, id string) (*model.TeamInviteLink, error) {
	query := s.getQueryBuilder().Select(teamInviteLinkColumns...).From("TeamInviteLinks").Where(sq.Eq{"Id": id})
	var link model.TeamInviteLink
	if err := s.GetMaster().GetBuilder(&link, query); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.NewErrNotFound("TeamInviteLink", id)
		}
		return nil, errors.Wrapf(err, "failed to get the invite link id=%s", id)
	}
	return &link, nil
}

func (s *SqlModerationStore) GetInviteLinks(rctx request.CTX, teamID string) ([]*model.TeamInviteLink, error) {
	query := s.getQueryBuilder().
		Select(teamInviteLinkColumns...).
		From("TeamInviteLinks").
		Where(sq.Eq{"TeamId": teamID}).
		OrderBy("CreateAt DESC", "Id")
	links := []*model.TeamInviteLink{}
	if err := s.DBXFromContext(rctx.Context()).SelectBuilder(&links, query); err != nil {
		return nil, errors.Wrapf(err, "failed to get invite links for team=%s", teamID)
	}
	return links, nil
}

func (s *SqlModerationStore) CountInviteLinks(rctx request.CTX, teamID string) (int64, error) {
	var count int64
	if err := s.GetMaster().GetBuilder(&count, s.getQueryBuilder().Select("COUNT(*)").From("TeamInviteLinks").Where(sq.Eq{"TeamId": teamID})); err != nil {
		return 0, errors.Wrapf(err, "failed to count invite links for team=%s", teamID)
	}
	return count, nil
}

// UseInviteLink takes one of a link's uses in a single statement, so that its last use goes to one person only.
func (s *SqlModerationStore) UseInviteLink(rctx request.CTX, id string, now int64) (bool, error) {
	builder := s.getQueryBuilder().
		Update("TeamInviteLinks").
		Set("Uses", sq.Expr("Uses + 1")).
		Where(sq.And{
			sq.Eq{"Id": id},
			sq.Or{sq.Eq{"MaxUses": 0}, sq.Expr("Uses < MaxUses")},
			sq.Or{sq.Eq{"ExpireAt": 0}, sq.Gt{"ExpireAt": now}},
		})
	result, err := s.GetMaster().ExecBuilder(builder)
	if err != nil {
		return false, errors.Wrapf(err, "failed to use the invite link id=%s", id)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, errors.Wrapf(err, "failed to use the invite link id=%s", id)
	}
	return count == 1, nil
}

func (s *SqlModerationStore) ReleaseInviteLinkUse(rctx request.CTX, id string) error {
	builder := s.getQueryBuilder().
		Update("TeamInviteLinks").
		Set("Uses", sq.Expr("Uses - 1")).
		Where(sq.And{sq.Eq{"Id": id}, sq.Gt{"Uses": 0}})
	if _, err := s.GetMaster().ExecBuilder(builder); err != nil {
		return errors.Wrapf(err, "failed to give back a use of the invite link id=%s", id)
	}
	return nil
}

func (s *SqlModerationStore) DeleteInviteLink(rctx request.CTX, id string) error {
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("TeamInviteLinks").Where(sq.Eq{"Id": id})); err != nil {
		return errors.Wrapf(err, "failed to delete the invite link id=%s", id)
	}
	return nil
}

func (s *SqlModerationStore) DeleteInviteLinksForTeam(rctx request.CTX, teamID string) error {
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("TeamInviteLinks").Where(sq.Eq{"TeamId": teamID})); err != nil {
		return errors.Wrapf(err, "failed to delete invite links for team=%s", teamID)
	}
	return nil
}

func (s *SqlModerationStore) PermanentDeleteInviteLinksByCreator(rctx request.CTX, userID string) error {
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("TeamInviteLinks").Where(sq.Eq{"CreatorId": userID})); err != nil {
		return errors.Wrapf(err, "failed to delete the invite links of user=%s", userID)
	}
	return nil
}
