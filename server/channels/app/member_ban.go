// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Antimatter bans, as Discord's: someone kept out of a community. Banning removes them from it and all its channels,
// and every way back in refuses them while the ban holds (JoinUserToTeam, which every join goes through: open join,
// invite links and codes, email invitations, group sync, being added by anyone, apps; and addUserToChannel, for the
// channel adds that don't check the membership first). Nobody adds a banned person back, admins included: whoever may
// lift the ban lifts it first, from the ban list.

const (
	// bannedFromTeamErrorID is the error of every join a ban refuses.
	bannedFromTeamErrorID = "app.team.join_user_to_team.banned.app_error"

	// banPurgeAtOnce is how many messages a ban deletes in its request; more are deleted by a job.
	banPurgeAtOnce = 100

	// banPurgeBatch is how many messages a purge reads at a time.
	banPurgeBatch = 200
)

// SessionCanSeeBans tells whether someone sees a community's bans: those who may ban, its admins and those who may
// remove its members. The mod view's other moderators (roles, messages) don't.
func (a *App) SessionCanSeeBans(session model.Session, teamID string) bool {
	return a.SessionHasPermissionToTeam(session, teamID, model.PermissionManageTeam) ||
		a.SessionHasPermissionToTeam(session, teamID, model.PermissionRemoveUserFromTeam)
}

// CheckCanBanMember tells whether someone may ban a person from a community: as for a timeout (its admins, or those
// who may remove members, for people below their own highest role; nobody bans themselves, the community's admins or
// the system's), also for someone who isn't, or is no longer, a member.
func (a *App) CheckCanBanMember(rctx request.CTX, session model.Session, teamID, userID string) *model.AppError {
	if session.UserId == userID {
		return model.NewAppError("CheckCanBanMember", "app.moderation.self.app_error", nil, "", http.StatusBadRequest)
	}
	if _, err := a.Srv().Store().User().Get(rctx, userID); err != nil {
		return model.NewAppError("CheckCanBanMember", MissingAccountError, nil, "", http.StatusNotFound).Wrap(err)
	}
	if member, err := a.Srv().Store().Team().GetMember(rctx, teamID, userID); err == nil && member.DeleteAt == 0 {
		return a.CheckCanModerateMember(rctx, session, teamID, userID)
	}
	if a.HasPermissionToTeam(rctx, userID, teamID, model.PermissionManageTeam) {
		return model.NewAppError("CheckCanBanMember", "app.moderation.admin.app_error", nil, "", http.StatusForbidden)
	}
	if a.SessionHasPermissionToTeam(session, teamID, model.PermissionManageTeam) {
		return nil
	}
	if !a.SessionHasPermissionToTeam(session, teamID, model.PermissionRemoveUserFromTeam) {
		return model.MakePermissionError(&session, []*model.Permission{model.PermissionRemoveUserFromTeam})
	}
	mine, theirs := a.highestTeamRolePosition(session.UserId, teamID), a.highestTeamRolePosition(userID, teamID)
	if mine == -1 || (theirs != -1 && theirs <= mine) {
		return model.NewAppError("CheckCanBanMember", "app.moderation.rank.app_error", nil, "", http.StatusForbidden)
	}
	return nil
}

// CheckCanLiftBan tells whether someone may lift a ban: the community's admins; or those who may remove its members,
// for their own bans and those of people who don't rank above them (never a community admin's ban): a moderator
// doesn't undo what someone above them decided.
func (a *App) CheckCanLiftBan(rctx request.CTX, session model.Session, ban *model.MemberBan) *model.AppError {
	if a.SessionHasPermissionToTeam(session, ban.TeamId, model.PermissionManageTeam) {
		return nil
	}
	if !a.SessionHasPermissionToTeam(session, ban.TeamId, model.PermissionRemoveUserFromTeam) {
		return model.MakePermissionError(&session, []*model.Permission{model.PermissionRemoveUserFromTeam})
	}
	if ban.CreatedBy == session.UserId {
		return nil
	}
	if ban.CreatedBy != "" && a.HasPermissionToTeam(rctx, ban.CreatedBy, ban.TeamId, model.PermissionManageTeam) {
		return model.NewAppError("CheckCanLiftBan", "app.moderation.ban.rank.app_error", nil, "", http.StatusForbidden)
	}
	mine, theirs := a.highestTeamRolePosition(session.UserId, ban.TeamId), a.highestTeamRolePosition(ban.CreatedBy, ban.TeamId)
	if theirs != -1 && (mine == -1 || theirs < mine) {
		return model.NewAppError("CheckCanLiftBan", "app.moderation.ban.rank.app_error", nil, "", http.StatusForbidden)
	}
	return nil
}

// GetTeamBans gives a community's bans that still hold, the latest first.
func (a *App) GetTeamBans(rctx request.CTX, teamID string) ([]*model.MemberBan, *model.AppError) {
	bans, err := a.Srv().Store().Moderation().GetBans(rctx, teamID, model.GetMillis())
	if err != nil {
		return nil, model.NewAppError("GetTeamBans", "app.moderation.bans.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return bans, nil
}

// checkNotBanned refuses someone banned from a community, and fails closed: a ban that can't be read refuses too.
func (a *App) checkNotBanned(rctx request.CTX, teamID, userID string) *model.AppError {
	if teamID == "" {
		return nil
	}
	_, err := a.Srv().Store().Moderation().GetBan(rctx, teamID, userID, model.GetMillis())
	if err == nil {
		return model.NewAppError("checkNotBanned", "app.team.join_user_to_team.banned.app_error", nil, "team_id="+teamID+", user_id="+userID, http.StatusForbidden)
	}
	var nfErr *store.ErrNotFound
	if errors.As(err, &nfErr) {
		return nil
	}
	return model.NewAppError("checkNotBanned", "app.moderation.bans.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
}

// BanMember bans someone from a community: the ban first (so that they can't come back while they're removed), then
// out of the community and all its channels, then their messages there from the time asked are deleted, at once when
// they're few, by a job otherwise. Banning someone banned again replaces their ban (its reason and end).
func (a *App) BanMember(rctx request.CTX, session model.Session, teamID, userID string, request *model.MemberBanRequest) (*model.MemberBanResult, *model.AppError) {
	if appErr := a.CheckCanBanMember(rctx, session, teamID, userID); appErr != nil {
		return nil, appErr
	}
	if appErr := request.IsValid(); appErr != nil {
		return nil, appErr
	}
	now := model.GetMillis()
	ban := &model.MemberBan{TeamId: teamID, UserId: userID, Reason: strings.TrimSpace(request.Reason), CreatedBy: session.UserId, ExpireAt: request.ExpireAt}
	if appErr := ban.IsValid(now); appErr != nil {
		return nil, appErr
	}
	// Replacing a ban that holds lifts it: only those who may lift it replace it, or a moderator would shorten an
	// admin's ban (or take it over, and then lift it as their own).
	existing, getErr := a.Srv().Store().Moderation().GetBan(rctx, teamID, userID, now)
	if getErr == nil {
		if appErr := a.CheckCanLiftBan(rctx, session, existing); appErr != nil {
			return nil, model.NewAppError("BanMember", "app.moderation.ban.replace.app_error", nil, "", http.StatusForbidden).Wrap(appErr)
		}
	} else {
		var nfErr *store.ErrNotFound
		if !errors.As(getErr, &nfErr) {
			return nil, model.NewAppError("BanMember", "app.moderation.bans.get.app_error", nil, "", http.StatusInternalServerError).Wrap(getErr)
		}
	}
	ban, err := a.Srv().Store().Moderation().SaveBan(rctx, ban)
	if err != nil {
		return nil, model.NewAppError("BanMember", "app.moderation.bans.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	result := &model.MemberBanResult{Ban: ban}

	if member, getErr := a.Srv().Store().Team().GetMember(rctx, teamID, userID); getErr == nil && member.DeleteAt == 0 {
		if appErr := a.RemoveUserFromTeam(rctx, teamID, userID, session.UserId); appErr != nil {
			return nil, appErr
		}
	}

	if request.DeleteMessagesSeconds > 0 {
		since, before := now-request.DeleteMessagesSeconds*1000, now+1
		count, countErr := a.Srv().Store().Moderation().CountPosts(rctx, teamID, userID, since)
		if countErr != nil {
			return nil, model.NewAppError("BanMember", "app.moderation.posts.app_error", nil, "", http.StatusInternalServerError).Wrap(countErr)
		}
		if count <= banPurgeAtOnce {
			deleted, appErr := a.PurgeMemberPosts(rctx, teamID, userID, since, before, session.UserId)
			result.DeletedPosts = deleted
			if appErr != nil {
				rctx.Logger().Warn("Failed to delete some of a banned person's messages", mlog.String("team_id", teamID), mlog.String("user_id", userID), mlog.Err(appErr))
			}
		} else {
			job, appErr := a.Srv().Jobs.CreateJob(rctx, model.JobTypeBanPurge, map[string]string{
				"team_id":    teamID,
				"user_id":    userID,
				"since":      strconv.FormatInt(since, 10),
				"before":     strconv.FormatInt(before, 10),
				"deleted_by": session.UserId,
			})
			if appErr != nil {
				return nil, appErr
			}
			result.PurgeJobId = job.Id
		}
	}
	return result, nil
}

// UnbanMember lifts someone's ban from a community, for those who may (CheckCanLiftBan).
func (a *App) UnbanMember(rctx request.CTX, session model.Session, teamID, userID string) *model.AppError {
	ban, err := a.Srv().Store().Moderation().GetBan(rctx, teamID, userID, model.GetMillis())
	if err != nil {
		var nfErr *store.ErrNotFound
		if errors.As(err, &nfErr) {
			return model.NewAppError("UnbanMember", "app.moderation.ban.not_found.app_error", nil, "", http.StatusNotFound)
		}
		return model.NewAppError("UnbanMember", "app.moderation.bans.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if appErr := a.CheckCanLiftBan(rctx, session, ban); appErr != nil {
		return appErr
	}
	if err := a.Srv().Store().Moderation().DeleteBan(rctx, teamID, userID); err != nil {
		return model.NewAppError("UnbanMember", "app.moderation.bans.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return nil
}

// PurgeMemberPosts deletes someone's messages in a community's channels created in [since, before), as deleting
// them one by one would (soft deletes, with their threads, files and events): only that community's, never their
// direct or group messages, nor their system messages. It stops when nothing more can be deleted.
func (a *App) PurgeMemberPosts(rctx request.CTX, teamID, userID string, since, before int64, deletedBy string) (int, *model.AppError) {
	deleted := 0
	for {
		ids, err := a.Srv().Store().Moderation().GetPostIdsInRange(rctx, teamID, userID, since, before, banPurgeBatch)
		if err != nil {
			return deleted, model.NewAppError("PurgeMemberPosts", "app.moderation.ban.purge.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		if len(ids) == 0 {
			return deleted, nil
		}
		progress := false
		for _, id := range ids {
			if _, appErr := a.DeletePost(rctx, id, deletedBy); appErr != nil {
				rctx.Logger().Debug("Failed to delete a banned person's message", mlog.String("post_id", id), mlog.Err(appErr))
				continue
			}
			deleted++
			progress = true
		}
		if !progress {
			return deleted, model.NewAppError("PurgeMemberPosts", "app.moderation.ban.purge.app_error", nil, "", http.StatusInternalServerError)
		}
	}
}
