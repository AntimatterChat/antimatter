// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Antimatter community rules, as Discord's rules screening: a community's admins write rules and turn them on, and
// its members agree to them before they may post in its channels. Saving them can ask everyone to agree again (a new
// version). The server holds people back, not only the clients: every way of posting as oneself goes through
// checkCommunityRules (App.CreatePost), and the API's edits and scheduled posts check it too.
//
// Not held back: direct and group messages (no community), reading, reactions (an emoji says nothing the rules are
// about, and acknowledging an announcement shouldn't need it), bots (apps acting as themselves), incoming webhooks,
// system messages, messages of other servers (shared channels), copies of messages moved by someone else, and the
// community's admins (manage_team, which system admins have too).

// clusterEventInvalidateCommunityRules tells the other nodes a team's rules changed; its data is the team.
const clusterEventInvalidateCommunityRules = model.ClusterEvent("inv_community_rules")

type cachedCommunityRules struct {
	rules    *model.CommunityRules
	loadedAt time.Time
}

func communityRulesNotAcceptedError(where string) *model.AppError {
	return model.NewAppError(where, "api.post.community_rules_not_accepted", nil, "", http.StatusForbidden)
}

// GetCommunityRules gives a team's rules, from the cache when fresh; rules that are off (version 0) when it never had
// any.
func (a *App) GetCommunityRules(rctx request.CTX, teamID string) (*model.CommunityRules, *model.AppError) {
	if cached, ok := a.Channels().communityRules.Load(teamID); ok {
		entry := cached.(cachedCommunityRules)
		if time.Since(entry.loadedAt) < channelOverridesTTL {
			rules := *entry.rules
			return &rules, nil
		}
	}
	rules, err := a.Srv().Store().CommunityRules().Get(rctx, teamID)
	if err != nil {
		var nfErr *store.ErrNotFound
		if !errors.As(err, &nfErr) {
			return nil, model.NewAppError("GetCommunityRules", "app.community_rules.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		rules = &model.CommunityRules{TeamId: teamID}
	}
	a.Channels().communityRules.Store(teamID, cachedCommunityRules{rules: rules, loadedAt: time.Now()})
	copied := *rules
	return &copied, nil
}

// GetCommunityRulesForUser gives a team's rules as someone sees them: with the latest version they agreed to, and
// whether they must agree before they may post.
func (a *App) GetCommunityRulesForUser(rctx request.CTX, teamID, userID string) (*model.CommunityRules, *model.AppError) {
	rules, appErr := a.GetCommunityRules(rctx, teamID)
	if appErr != nil {
		return nil, appErr
	}
	accepted, err := a.Srv().Store().CommunityRules().GetAcceptedVersion(rctx, teamID, userID)
	if err != nil {
		return nil, model.NewAppError("GetCommunityRulesForUser", "app.community_rules.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	rules.AcceptedVersion = accepted
	if rules.IsActive() && accepted < rules.Version {
		user, userErr := a.GetUser(rctx, userID)
		rules.MustAccept = userErr != nil || !a.isCommunityRulesExempt(rctx, user, teamID)
	}
	return rules, nil
}

// SaveCommunityRules puts in place a team's rules, and tells its members' clients. The rules' first version, and
// each save that asks again, need everyone to agree (again) before they post.
func (a *App) SaveCommunityRules(rctx request.CTX, teamID, updatedBy string, request *model.CommunityRulesRequest) (*model.CommunityRules, *model.AppError) {
	request.PreSave()
	if appErr := request.IsValid(); appErr != nil {
		return nil, appErr
	}
	rules, err := a.Srv().Store().CommunityRules().Save(rctx, &model.CommunityRules{
		TeamId:    teamID,
		Enabled:   request.Enabled,
		Text:      request.Text,
		UpdatedBy: updatedBy,
	}, request.AskAgain)
	if err != nil {
		return nil, model.NewAppError("SaveCommunityRules", "app.community_rules.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	a.invalidateCommunityRules(rctx, teamID)
	message := model.NewWebSocketEvent(model.WebsocketEventCommunityRulesUpdated, teamID, "", "", nil, "")
	message.Add("team_id", teamID)
	message.Add("enabled", rules.Enabled)
	message.Add("version", rules.Version)
	a.Publish(message)
	return rules, nil
}

// AcceptCommunityRules keeps a member's agreement to a team's rules, which must be on. version is the one they read:
// when the rules changed since (asking again), the agreement is refused so they read the new ones first.
func (a *App) AcceptCommunityRules(rctx request.CTX, teamID, userID string, version int64) (*model.CommunityRules, *model.AppError) {
	member, appErr := a.GetTeamMember(rctx, teamID, userID)
	if appErr != nil || member.DeleteAt != 0 {
		return nil, model.NewAppError("AcceptCommunityRules", "app.community_rules.accept.not_member.app_error", nil, "", http.StatusForbidden)
	}
	rules, appErr := a.GetCommunityRules(RequestContextWithMaster(rctx), teamID)
	if appErr != nil {
		return nil, appErr
	}
	if !rules.IsActive() {
		return nil, model.NewAppError("AcceptCommunityRules", "app.community_rules.accept.off.app_error", nil, "", http.StatusBadRequest)
	}
	if version != 0 && version != rules.Version {
		return nil, model.NewAppError("AcceptCommunityRules", "app.community_rules.accept.changed.app_error", nil, "", http.StatusConflict)
	}
	if err := a.Srv().Store().CommunityRules().Accept(rctx, &model.CommunityRulesAcceptance{TeamId: teamID, UserId: userID, Version: rules.Version}); err != nil {
		return nil, model.NewAppError("AcceptCommunityRules", "app.community_rules.accept.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	// Their other sessions follow (the composer comes back everywhere); nobody else learns who agreed.
	message := model.NewWebSocketEvent(model.WebsocketEventCommunityRulesAccepted, "", "", userID, nil, "")
	message.Add("team_id", teamID)
	message.Add("version", rules.Version)
	a.Publish(message)

	return a.GetCommunityRulesForUser(RequestContextWithMaster(rctx), teamID, userID)
}

// GetCommunityRulesStats counts, for a team's admins, how many of its members agreed to the current rules.
func (a *App) GetCommunityRulesStats(rctx request.CTX, teamID string) (*model.CommunityRulesStats, *model.AppError) {
	rules, appErr := a.GetCommunityRules(rctx, teamID)
	if appErr != nil {
		return nil, appErr
	}
	stats, err := a.Srv().Store().CommunityRules().CountAcceptances(rctx, teamID, rules.Version)
	if err != nil {
		return nil, model.NewAppError("GetCommunityRulesStats", "app.community_rules.stats.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return stats, nil
}

// isCommunityRulesExempt tells whether someone posts in a team's channels without agreeing to its rules: bots, people
// of other servers, and the team's admins (system admins included).
func (a *App) isCommunityRulesExempt(rctx request.CTX, user *model.User, teamID string) bool {
	return user.IsBot || user.IsRemote() || a.HasPermissionToTeam(rctx, user.Id, teamID, model.PermissionManageTeam)
}

// acceptedVersion gives the latest version of a team's rules someone agreed to, from a replica, or from the master
// when asked to (a replica may not have caught up with an agreement just made).
func (a *App) acceptedVersion(rctx request.CTX, teamID, userID string, master bool) (int64, *model.AppError) {
	if master {
		rctx = RequestContextWithMaster(rctx)
	}
	accepted, err := a.Srv().Store().CommunityRules().GetAcceptedVersion(rctx, teamID, userID)
	if err != nil {
		return 0, model.NewAppError("checkCommunityRules", "app.community_rules.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return accepted, nil
}

// checkCommunityRules refuses a message someone posts as themselves in a team channel whose rules are on, until they
// agreed to the current version. Direct and group messages, and those exempt, pass. The master database is only
// asked when a replica says they haven't agreed and they aren't an admin, so posting right after agreeing works.
func (a *App) checkCommunityRules(rctx request.CTX, user *model.User, channel *model.Channel) *model.AppError {
	if user == nil || channel == nil || channel.TeamId == "" {
		return nil
	}
	rules, appErr := a.GetCommunityRules(rctx, channel.TeamId)
	if appErr != nil {
		return appErr
	}
	if !rules.IsActive() || user.IsBot || user.IsRemote() {
		return nil
	}
	accepted, appErr := a.acceptedVersion(rctx, channel.TeamId, user.Id, false)
	if appErr != nil {
		return appErr
	}
	if accepted >= rules.Version || a.HasPermissionToTeam(rctx, user.Id, channel.TeamId, model.PermissionManageTeam) {
		return nil
	}
	if accepted, appErr = a.acceptedVersion(rctx, channel.TeamId, user.Id, true); appErr != nil {
		return appErr
	}
	if accepted >= rules.Version {
		return nil
	}
	return communityRulesNotAcceptedError("checkCommunityRules")
}

// CheckCommunityRulesForUser refuses what someone does as themselves in a channel that counts as posting (editing a
// message, scheduling one, running a command that posts), until they agreed to its team's rules. Nobody (a local or
// system call) and direct and group messages pass.
func (a *App) CheckCommunityRulesForUser(rctx request.CTX, userID string, channel *model.Channel) *model.AppError {
	if userID == "" || channel == nil || channel.TeamId == "" {
		return nil
	}
	// Most communities have no rules: they cost a cache lookup, not the user's.
	rules, appErr := a.GetCommunityRules(rctx, channel.TeamId)
	if appErr != nil {
		return appErr
	}
	if !rules.IsActive() {
		return nil
	}
	user, appErr := a.GetUser(rctx, userID)
	if appErr != nil {
		return appErr
	}
	return a.checkCommunityRules(rctx, user, channel)
}

// CheckCommunityRulesForUserInChannel is CheckCommunityRulesForUser for a channel known by its ID.
func (a *App) CheckCommunityRulesForUserInChannel(rctx request.CTX, userID, channelID string) *model.AppError {
	if userID == "" || channelID == "" {
		return nil
	}
	channel, appErr := a.GetChannel(rctx, channelID)
	if appErr != nil {
		// The callers check the channel themselves, and say what's wrong with it.
		return nil
	}
	return a.CheckCommunityRulesForUser(rctx, userID, channel)
}

// SendCommunityRulesReminder tells someone, where they tried to post, how to read and agree to the community's rules
// (/rules, then /rules agree): the clients without a screen for them (the classic web app, the mobile app) only show
// the message as failed.
func (a *App) SendCommunityRulesReminder(rctx request.CTX, userID, channelID, rootID string) {
	user, appErr := a.GetUser(rctx, userID)
	if appErr != nil {
		return
	}
	T := i18n.GetUserTranslations(user.Locale)
	a.SendEphemeralPost(rctx, userID, &model.Post{
		ChannelId: channelID,
		RootId:    rootID,
		Message:   T("api.post.community_rules_not_accepted.reminder"),
	})
}

// invalidateCommunityRules forgets a team's cached rules here and on the other nodes, and reloads them from the
// master database, which a replica may lag behind.
func (a *App) invalidateCommunityRules(rctx request.CTX, teamID string) {
	a.Channels().communityRules.Delete(teamID)
	if _, appErr := a.GetCommunityRules(RequestContextWithMaster(rctx), teamID); appErr != nil {
		rctx.Logger().Warn("Failed to reload a team's rules", mlog.String("team_id", teamID), mlog.Err(appErr))
	}
	if cluster := a.Srv().Platform().Cluster(); cluster != nil {
		cluster.SendClusterMessage(&model.ClusterMessage{
			Event:    clusterEventInvalidateCommunityRules,
			SendType: model.ClusterSendReliable,
			Data:     []byte(teamID),
		})
	}
}

func (ch *Channels) clusterInvalidateCommunityRulesHandler(msg *model.ClusterMessage) {
	if len(msg.Data) == 0 {
		ch.communityRules.Clear()
		return
	}
	ch.communityRules.Delete(string(msg.Data))
}

// deleteCommunityRulesForTeam forgets a team's rules and its members' agreements, as the team goes.
func (a *App) deleteCommunityRulesForTeam(rctx request.CTX, teamID string) *model.AppError {
	if err := a.Srv().Store().CommunityRules().DeleteForTeam(rctx, teamID); err != nil {
		return model.NewAppError("deleteCommunityRulesForTeam", "app.community_rules.delete.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	a.invalidateCommunityRules(rctx, teamID)
	return nil
}
