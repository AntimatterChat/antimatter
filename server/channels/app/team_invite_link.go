// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Antimatter invite links: a community has its permanent invite code (Teams.InviteId, as upstream, which its admins
// regenerate) and as many invite links as those allowed to invite make, each with an optional end and number of uses.
// Both are codes for the same URL (/signup_user_complete/?id=<code>) and the same endpoints (joining, signing up,
// the invite's details), which take either (resolveInviteId). A link works while it hasn't ended, isn't used up, and
// its creator may still invite in the community; revoking it deletes it. A ban wins over any link.

// SessionCanInvite tells whether someone may make invite links in a community: those who may invite to it (who see
// its permanent invite code, too).
func (a *App) SessionCanInvite(session model.Session, teamID string) bool {
	return a.SessionHasPermissionToTeam(session, teamID, model.PermissionInviteUser)
}

// GetTeamInviteLinks gives a community's invite links, the latest first: all of them to its admins, their own to the
// others who may invite.
func (a *App) GetTeamInviteLinks(rctx request.CTX, session model.Session, teamID string) ([]*model.TeamInviteLink, *model.AppError) {
	admin := a.SessionHasPermissionToTeam(session, teamID, model.PermissionManageTeam)
	if !admin && !a.SessionCanInvite(session, teamID) {
		return nil, model.MakePermissionError(&session, []*model.Permission{model.PermissionInviteUser})
	}
	links, err := a.Srv().Store().Moderation().GetInviteLinks(rctx, teamID)
	if err != nil {
		return nil, model.NewAppError("GetTeamInviteLinks", "app.team_invite_link.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if admin {
		return links, nil
	}
	own := make([]*model.TeamInviteLink, 0, len(links))
	for _, link := range links {
		if link.CreatorId == session.UserId {
			own = append(own, link)
		}
	}
	return own, nil
}

// CreateTeamInviteLink makes one of a community's invite links, for someone allowed to invite to it. A community whose
// members come from groups has none (no invite code lets anyone in there).
func (a *App) CreateTeamInviteLink(rctx request.CTX, session model.Session, teamID string, request *model.TeamInviteLinkRequest) (*model.TeamInviteLink, *model.AppError) {
	if !a.SessionCanInvite(session, teamID) {
		return nil, model.MakePermissionError(&session, []*model.Permission{model.PermissionInviteUser})
	}
	team, appErr := a.GetTeam(teamID)
	if appErr != nil {
		return nil, appErr
	}
	if team.DeleteAt != 0 {
		return nil, model.NewAppError("CreateTeamInviteLink", "app.team.get.find.app_error", nil, "", http.StatusNotFound)
	}
	if team.IsGroupConstrained() {
		return nil, model.NewAppError("CreateTeamInviteLink", "app.team.invite_id.group_constrained.error", nil, "", http.StatusBadRequest)
	}
	count, err := a.Srv().Store().Moderation().CountInviteLinks(rctx, teamID)
	if err != nil {
		return nil, model.NewAppError("CreateTeamInviteLink", "app.team_invite_link.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if count >= model.TeamInviteLinksMaxPerTeam {
		return nil, model.NewAppError("CreateTeamInviteLink", "app.team_invite_link.too_many.app_error", map[string]any{"Max": model.TeamInviteLinksMaxPerTeam}, "", http.StatusBadRequest)
	}
	now := model.GetMillis()
	link := &model.TeamInviteLink{Id: model.NewId(), TeamId: teamID, CreatorId: session.UserId, CreateAt: now, ExpireAt: request.ExpireAt, MaxUses: request.MaxUses}
	if appErr := link.IsValid(now); appErr != nil {
		return nil, appErr
	}
	if link, err = a.Srv().Store().Moderation().SaveInviteLink(rctx, link); err != nil {
		return nil, model.NewAppError("CreateTeamInviteLink", "app.team_invite_link.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return link, nil
}

// RevokeTeamInviteLink revokes one of a community's links: its creator, or the community's admins.
func (a *App) RevokeTeamInviteLink(rctx request.CTX, session model.Session, teamID, linkID string) *model.AppError {
	link, err := a.Srv().Store().Moderation().GetInviteLink(rctx, linkID)
	if err != nil {
		var nfErr *store.ErrNotFound
		if errors.As(err, &nfErr) {
			return model.NewAppError("RevokeTeamInviteLink", "app.team_invite_link.not_found.app_error", nil, "", http.StatusNotFound)
		}
		return model.NewAppError("RevokeTeamInviteLink", "app.team_invite_link.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if link.TeamId != teamID {
		return model.NewAppError("RevokeTeamInviteLink", "app.team_invite_link.not_found.app_error", nil, "", http.StatusNotFound)
	}
	if link.CreatorId != session.UserId && !a.SessionHasPermissionToTeam(session, teamID, model.PermissionManageTeam) {
		return model.MakePermissionError(&session, []*model.Permission{model.PermissionManageTeam})
	}
	if err := a.Srv().Store().Moderation().DeleteInviteLink(rctx, linkID); err != nil {
		return model.NewAppError("RevokeTeamInviteLink", "app.team_invite_link.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return nil
}

// resolveInviteId finds the community an invite code lets one into: its permanent invite code, or one of its invite
// links (then given too), which must still work. An unknown or revoked code isn't found, as upstream's.
func (a *App) resolveInviteId(rctx request.CTX, inviteId string) (*model.Team, *model.TeamInviteLink, *model.AppError) {
	return a.resolveInviteIdFor(rctx, inviteId, "")
}

// resolveInviteIdFor is resolveInviteId for someone joining (userID, or "" for anyone): a used-up link still finds
// its community for someone already in it, who takes no use (joinByInviteLink). Everything else must still hold.
func (a *App) resolveInviteIdFor(rctx request.CTX, inviteId, userID string) (*model.Team, *model.TeamInviteLink, *model.AppError) {
	notFound := func(err error) *model.AppError {
		return model.NewAppError("resolveInviteId", "app.team.get_by_invite_id.finding.app_error", nil, "", http.StatusNotFound).Wrap(err)
	}
	team, err := a.Srv().Store().Team().GetByInviteId(inviteId)
	if err == nil {
		return team, nil, nil
	}
	var nfErr *store.ErrNotFound
	if !errors.As(err, &nfErr) {
		return nil, nil, model.NewAppError("resolveInviteId", "app.team.get_by_invite_id.finding.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if !model.IsValidId(inviteId) {
		return nil, nil, notFound(err)
	}
	link, linkErr := a.Srv().Store().Moderation().GetInviteLink(rctx, inviteId)
	if linkErr != nil {
		if errors.As(linkErr, &nfErr) {
			return nil, nil, notFound(linkErr)
		}
		return nil, nil, model.NewAppError("resolveInviteId", "app.team_invite_link.get.app_error", nil, "", http.StatusInternalServerError).Wrap(linkErr)
	}
	team, err = a.Srv().Store().Team().Get(link.TeamId)
	if err != nil || team.DeleteAt != 0 {
		return nil, nil, notFound(err)
	}
	alreadyIn := false
	if userID != "" {
		if member, memberErr := a.Srv().Store().Team().GetMember(rctx, team.Id, userID); memberErr == nil && member.DeleteAt == 0 {
			alreadyIn = true
		}
	}
	if appErr := a.checkInviteLinkWorks(rctx, link, alreadyIn); appErr != nil {
		return nil, nil, appErr
	}
	return team, link, nil
}

// checkInviteLinkWorks tells why a link doesn't let anyone in: it has ended, it's used up (unless ignoreUses, for
// someone already in, who takes none), or its creator may no longer invite to the community (they left it, were
// banned or deactivated, or lost the right).
func (a *App) checkInviteLinkWorks(rctx request.CTX, link *model.TeamInviteLink, ignoreUses bool) *model.AppError {
	if link.IsExpired(model.GetMillis()) {
		return model.NewAppError("checkInviteLinkWorks", "app.team_invite_link.expired.app_error", nil, "", http.StatusBadRequest)
	}
	if !ignoreUses && link.IsUsedUp() {
		return model.NewAppError("checkInviteLinkWorks", "app.team_invite_link.used_up.app_error", nil, "", http.StatusBadRequest)
	}
	creator, err := a.Srv().Store().User().Get(rctx, link.CreatorId)
	if err != nil || creator.DeleteAt != 0 || !a.HasPermissionToTeam(rctx, link.CreatorId, link.TeamId, model.PermissionInviteUser) {
		return model.NewAppError("checkInviteLinkWorks", "app.team_invite_link.invalid.app_error", nil, "", http.StatusBadRequest)
	}
	return nil
}

// useInviteLink takes one of a link's uses, at once (two people never share its last use).
func (a *App) useInviteLink(rctx request.CTX, link *model.TeamInviteLink) *model.AppError {
	ok, err := a.Srv().Store().Moderation().UseInviteLink(rctx, link.Id, model.GetMillis())
	if err != nil {
		return model.NewAppError("useInviteLink", "app.team_invite_link.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if !ok {
		// Used up (or ended, or revoked) since it was read.
		return model.NewAppError("useInviteLink", "app.team_invite_link.used_up.app_error", nil, "", http.StatusBadRequest)
	}
	return nil
}

// releaseInviteLinkUse gives a use back when the join it was taken for failed.
func (a *App) releaseInviteLinkUse(rctx request.CTX, link *model.TeamInviteLink) {
	if err := a.Srv().Store().Moderation().ReleaseInviteLinkUse(rctx, link.Id); err != nil {
		rctx.Logger().Warn("Failed to give back an invite link's use", mlog.String("link_id", link.Id), mlog.Err(err))
	}
}

// joinByInviteLink joins someone to a community by one of its links: a ban refuses them before a use is taken,
// someone already in takes none, and a join that fails gives its use back.
func (a *App) joinByInviteLink(rctx request.CTX, team *model.Team, link *model.TeamInviteLink, user *model.User) (*model.TeamMember, *model.AppError) {
	if appErr := a.checkNotBanned(rctx, team.Id, user.Id); appErr != nil {
		return nil, appErr
	}
	if member, err := a.Srv().Store().Team().GetMember(rctx, team.Id, user.Id); err == nil && member.DeleteAt == 0 {
		return member, nil
	}
	if appErr := a.useInviteLink(rctx, link); appErr != nil {
		return nil, appErr
	}
	member, appErr := a.JoinUserToTeam(rctx, team, user, "")
	if appErr != nil {
		a.releaseInviteLinkUse(rctx, link)
		return nil, appErr
	}
	return member, nil
}
