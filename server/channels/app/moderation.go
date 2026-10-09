// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Antimatter moderation, as Discord's mod view: what a team's moderators see of someone, and timeouts, which keep
// someone from speaking in the team's channels for a while.

type cachedTimeouts struct {
	timeouts []*model.MemberTimeout
	loadedAt time.Time
}

var (
	timeoutPermissions     map[string]bool
	timeoutPermissionsOnce sync.Once
)

func isTimeoutPermission(permissionID string) bool {
	timeoutPermissionsOnce.Do(func() {
		timeoutPermissions = map[string]bool{}
		for _, p := range model.MemberTimeoutPermissions() {
			timeoutPermissions[p] = true
		}
	})
	return timeoutPermissions[permissionID]
}

// GetTeamTimeouts gives a team's timeouts that still hold, from the cache when fresh.
func (a *App) GetTeamTimeouts(rctx request.CTX, teamID string) ([]*model.MemberTimeout, *model.AppError) {
	now := model.GetMillis()
	if cached, ok := a.Channels().memberTimeouts.Load(teamID); ok {
		entry := cached.(cachedTimeouts)
		if time.Since(entry.loadedAt) < channelOverridesTTL {
			return slices.DeleteFunc(slices.Clone(entry.timeouts), func(t *model.MemberTimeout) bool { return !t.IsActive(now) }), nil
		}
	}
	timeouts, err := a.Srv().Store().Moderation().GetTimeouts(rctx, teamID, now)
	if err != nil {
		return nil, model.NewAppError("GetTeamTimeouts", "app.moderation.timeouts.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	a.Channels().memberTimeouts.Store(teamID, cachedTimeouts{timeouts: timeouts, loadedAt: time.Now()})
	return timeouts, nil
}

func (a *App) memberTimeout(rctx request.CTX, teamID, userID string) *model.MemberTimeout {
	timeouts, appErr := a.GetTeamTimeouts(rctx, teamID)
	if appErr != nil {
		rctx.Logger().Warn("Failed to get a team's timeouts", mlog.String("team_id", teamID), mlog.Err(appErr))
		return nil
	}
	for _, t := range timeouts {
		if t.UserId == userID {
			return t
		}
	}
	return nil
}

func (a *App) isTimedOut(rctx request.CTX, teamID, userID string) bool {
	return a.memberTimeout(rctx, teamID, userID).IsActive(model.GetMillis())
}

// SessionCanModerateTeam tells whether someone sees a team's mod view: its admins, and those who may remove its
// members, manage its roles or manage its messages.
func (a *App) SessionCanModerateTeam(session model.Session, teamID string) bool {
	for _, p := range []*model.Permission{model.PermissionManageTeam, model.PermissionRemoveUserFromTeam, model.PermissionManageTeamRoles, model.PermissionDeleteOthersPosts} {
		if a.SessionHasPermissionToTeam(session, teamID, p) {
			return true
		}
	}
	return false
}

// CheckCanModerateMember tells whether someone may time out (or remove) one of a team's members: the team's admins,
// or those who may remove members, for people below their own highest role, as Discord's hierarchy. Nobody
// moderates themselves, the team's admins or the system's.
func (a *App) CheckCanModerateMember(rctx request.CTX, session model.Session, teamID, userID string) *model.AppError {
	if session.UserId == userID {
		return model.NewAppError("CheckCanModerateMember", "app.moderation.self.app_error", nil, "", http.StatusBadRequest)
	}
	member, err := a.Srv().Store().Team().GetMember(rctx, teamID, userID)
	if err != nil || member.DeleteAt != 0 {
		return model.NewAppError("CheckCanModerateMember", "app.moderation.not_member.app_error", nil, "", http.StatusBadRequest)
	}
	if a.HasPermissionToTeam(rctx, userID, teamID, model.PermissionManageTeam) {
		return model.NewAppError("CheckCanModerateMember", "app.moderation.admin.app_error", nil, "", http.StatusForbidden)
	}
	if a.SessionHasPermissionToTeam(session, teamID, model.PermissionManageTeam) {
		return nil
	}
	if !a.SessionHasPermissionToTeam(session, teamID, model.PermissionRemoveUserFromTeam) {
		return model.MakePermissionError(&session, []*model.Permission{model.PermissionRemoveUserFromTeam})
	}
	mine, theirs := a.highestTeamRolePosition(session.UserId, teamID), a.highestTeamRolePosition(userID, teamID)
	if mine == -1 || (theirs != -1 && theirs <= mine) {
		return model.NewAppError("CheckCanModerateMember", "app.moderation.rank.app_error", nil, "", http.StatusForbidden)
	}
	return nil
}

// SetMemberTimeout times someone of a team out until a time, or lifts their timeout (until 0).
func (a *App) SetMemberTimeout(rctx request.CTX, session model.Session, teamID, userID string, request *model.MemberTimeoutRequest) (*model.MemberTimeout, *model.AppError) {
	if appErr := a.CheckCanModerateMember(rctx, session, teamID, userID); appErr != nil {
		return nil, appErr
	}
	var timeout *model.MemberTimeout
	if request.Until == 0 {
		if err := a.Srv().Store().Moderation().DeleteTimeout(rctx, teamID, userID); err != nil {
			return nil, model.NewAppError("SetMemberTimeout", "app.moderation.timeouts.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
	} else {
		timeout = &model.MemberTimeout{TeamId: teamID, UserId: userID, Until: request.Until, Reason: strings.TrimSpace(request.Reason), CreatedBy: session.UserId}
		if appErr := timeout.IsValid(model.GetMillis()); appErr != nil {
			return nil, appErr
		}
		var err error
		if timeout, err = a.Srv().Store().Moderation().SaveTimeout(rctx, timeout); err != nil {
			return nil, model.NewAppError("SetMemberTimeout", "app.moderation.timeouts.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
	}

	a.Channels().memberTimeouts.Delete(teamID)
	if _, appErr := a.GetTeamTimeouts(RequestContextWithMaster(rctx), teamID); appErr != nil {
		rctx.Logger().Warn("Failed to reload a team's timeouts", mlog.String("team_id", teamID), mlog.Err(appErr))
	}
	a.sendChannelOverridesInvalidation("")
	message := model.NewWebSocketEvent(model.WebsocketEventMemberTimeoutsUpdated, teamID, "", "", nil, "")
	message.Add("team_id", teamID)
	a.Publish(message)
	return timeout, nil
}

// GetMemberModeration gives what a team's moderators see of one of its members.
func (a *App) GetMemberModeration(rctx request.CTX, teamID, userID string) (*model.MemberModeration, *model.AppError) {
	member, err := a.Srv().Store().Team().GetMember(rctx, teamID, userID)
	if err != nil {
		return nil, model.NewAppError("GetMemberModeration", "app.moderation.not_member.app_error", nil, "", http.StatusNotFound).Wrap(err)
	}
	now := model.GetMillis()
	moderation := &model.MemberModeration{TeamId: teamID, UserId: userID, JoinedAt: member.CreateAt}
	if timeout := a.memberTimeout(rctx, teamID, userID); timeout.IsActive(now) {
		moderation.Timeout = timeout
	}
	day := 24 * time.Hour.Milliseconds()
	if moderation.PostsLastDay, err = a.Srv().Store().Moderation().CountPosts(rctx, teamID, userID, now-day); err != nil {
		return nil, model.NewAppError("GetMemberModeration", "app.moderation.posts.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if moderation.PostsLastWeek, err = a.Srv().Store().Moderation().CountPosts(rctx, teamID, userID, now-7*day); err != nil {
		return nil, model.NewAppError("GetMemberModeration", "app.moderation.posts.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return moderation, nil
}
