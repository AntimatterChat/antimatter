// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Antimatter communities can have everyone in all their channels, as in Discord (Team.AllChannelsMembership): each
// member is in every public channel of the community, and in every private channel their roles open (a role-gated
// channel: see SetChannelRoleAccess). Members can't leave those channels, nor be removed from them: they mute them;
// moderation goes through the community (removal, timeouts) or the channel's permissions. Guests and bots aren't
// concerned: they're in the channels they're added to. Invite-only private channels stay as they are.
//
// Memberships follow: someone joining the community joins its channels, a channel created, made public or unarchived
// gets everyone, and turning the setting on fills every channel in the background. Roles already bring their members
// into the channels they gate, whatever the setting. Turning it off keeps the memberships; people can then leave.

// allChannelsFills are the communities being filled in the background, so that turning the setting off and on again
// doesn't run two fills at once.
var allChannelsFills sync.Map

// IsAllChannelsMembership tells whether someone is in a channel because they're in its community, which has everyone
// in all its channels: they can't leave it, nor be removed from it.
func (a *App) IsAllChannelsMembership(rctx request.CTX, channel *model.Channel, userID string) (bool, *model.AppError) {
	if !isCommunityWideChannel(channel) {
		return false, nil
	}
	team, appErr := a.GetTeam(channel.TeamId)
	if appErr != nil {
		return false, appErr
	}
	if !team.AllChannelsMembership || team.DeleteAt != 0 {
		return false, nil
	}
	user, appErr := a.GetUser(rctx, userID)
	if appErr != nil {
		return false, appErr
	}
	if !isCommunityWideUser(user) {
		return false, nil
	}
	member, err := a.Srv().Store().Team().GetMember(rctx, team.Id, userID)
	if err != nil {
		var nfErr *store.ErrNotFound
		if errors.As(err, &nfErr) {
			return false, nil
		}
		return false, model.NewAppError("IsAllChannelsMembership", "app.team.get_member.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if member.DeleteAt != 0 {
		return false, nil
	}

	// A private channel is theirs through its roles only.
	if channel.IsGroupConstrained() {
		outside, err := a.FilterNonGroupChannelMembers(rctx, []string{userID}, channel)
		if err != nil {
			return false, model.NewAppError("IsAllChannelsMembership", "app.channel.all_channels.groups.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		return len(outside) == 0, nil
	}
	return true, nil
}

// checkAllChannelsLeave refuses someone leaving, or being removed from, a channel they're in through their community.
// Removals by the server itself (no remover: role and policy syncs) go through.
func (a *App) checkAllChannelsLeave(rctx request.CTX, channel *model.Channel, userID, removerUserID string) *model.AppError {
	if removerUserID == "" {
		return nil
	}
	held, appErr := a.IsAllChannelsMembership(rctx, channel, userID)
	if appErr != nil {
		rctx.Logger().Warn("Failed to check a community's all-channels membership", mlog.String("channel_id", channel.Id), mlog.String("user_id", userID), mlog.Err(appErr))
		return nil
	}
	if !held {
		return nil
	}
	if userID == removerUserID {
		return model.NewAppError("LeaveChannel", "api.channel.leave.all_channels.app_error", nil, "", http.StatusBadRequest)
	}
	return model.NewAppError("RemoveUserFromChannel", "api.channel.remove.all_channels.app_error", nil, "", http.StatusBadRequest)
}

// joinCommunityChannels brings someone joining a community that has everyone in all its channels into its public
// channels. Its private ones come with the roles they'll get.
func (a *App) joinCommunityChannels(rctx request.CTX, team *model.Team, user *model.User) {
	if team == nil || !team.AllChannelsMembership || !isCommunityWideUser(user) {
		return
	}

	// The list leaves out the channels joined, so it only moves past those skipped.
	skipped := 0
	for {
		channels, err := a.Srv().Store().Channel().GetMoreChannels(team.Id, user.Id, skipped, communityFillBatchSize)
		if err != nil {
			rctx.Logger().Warn("Failed to get the channels of a community to join", mlog.String("team_id", team.Id), mlog.String("user_id", user.Id), mlog.Err(err))
			return
		}
		for _, channel := range channels {
			if !isCommunityWideChannel(channel) {
				skipped++
				continue
			}
			if appErr := a.joinChannelForCommunity(rctx, user, channel, true); appErr != nil {
				rctx.Logger().Warn("Failed to join a channel of the community", mlog.String("channel_id", channel.Id), mlog.String("user_id", user.Id), mlog.Err(appErr))
				skipped++
			}
		}
		if len(channels) < communityFillBatchSize {
			return
		}
	}
}

// fillCommunityChannel brings every member of a community that has everyone in all its channels into one of its
// channels (for a private channel, those of its roles), in batches.
func (a *App) fillCommunityChannel(rctx request.CTX, team *model.Team, channel *model.Channel) {
	if team == nil || !team.AllChannelsMembership {
		return
	}
	a.fillChannelWithCommunity(rctx, team, channel)
}

// FillCommunityChannelInBackground fills a channel with its community's members, when the community has everyone in
// all its channels: for a channel created, made public or unarchived.
func (a *App) FillCommunityChannelInBackground(rctx request.CTX, channel *model.Channel) {
	if !isCommunityWideChannel(channel) {
		return
	}
	team, appErr := a.GetTeam(channel.TeamId)
	if appErr != nil || !team.AllChannelsMembership {
		return
	}
	logger := rctx.Logger()
	a.Srv().Go(func() {
		bg := request.EmptyContext(logger)
		a.fillCommunityChannel(bg, team, channel)
	})
}

// FillCommunityChannelsInBackground fills all the channels of a community with its members, once it has everyone in
// all its channels. A community already being filled isn't filled twice at once.
func (a *App) FillCommunityChannelsInBackground(rctx request.CTX, team *model.Team) {
	if team == nil || !team.AllChannelsMembership {
		return
	}
	if _, running := allChannelsFills.LoadOrStore(team.Id, true); running {
		return
	}
	logger := rctx.Logger()
	a.Srv().Go(func() {
		defer allChannelsFills.Delete(team.Id)
		bg := request.EmptyContext(logger)
		start := time.Now()

		channels, err := a.Srv().Store().Channel().GetTeamChannels(team.Id)
		if err != nil {
			bg.Logger().Warn("Failed to get the channels of a community to fill", mlog.String("team_id", team.Id), mlog.Err(err))
			return
		}
		filled := 0
		for _, channel := range channels {
			// The setting may have been turned off meanwhile.
			current, appErr := a.GetTeam(team.Id)
			if appErr != nil || !current.AllChannelsMembership {
				return
			}
			if isCommunityWideChannel(channel) {
				a.fillCommunityChannel(bg, current, channel)
				filled++
			}
		}
		bg.Logger().Info("Filled the channels of a community with its members", mlog.String("team_id", team.Id), mlog.Int("channels", filled), mlog.Duration("duration", time.Since(start)))
	})
}
