// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Filling a channel with its community: everyone in it (but guests and bots), as for General, a default channel the
// community chose, or every channel of a community that has everyone in all its channels (community_channels.go).

// communityFillBatchSize is how many people a channel is filled with at once; communityFillPause lets the database
// breathe between batches of a big community.
const (
	communityFillBatchSize = 200
	communityFillPause     = 20 * time.Millisecond
)

// isCommunityWideUser tells whether someone is in the channels a whole community is in: active people, not guests nor
// bots.
func isCommunityWideUser(user *model.User) bool {
	return user != nil && user.DeleteAt == 0 && !user.IsBot && !user.IsGuest()
}

// isCommunityWideChannel tells whether a channel of a community is one all its members may be in: a public channel, or
// a private one gated by roles (or groups), for their people. Channels under an access policy have their own rules.
func isCommunityWideChannel(channel *model.Channel) bool {
	if channel == nil || channel.TeamId == "" || channel.DeleteAt != 0 || channel.PolicyEnforced {
		return false
	}
	switch channel.Type {
	case model.ChannelTypeOpen:
		return true
	case model.ChannelTypePrivate:
		return channel.IsGroupConstrained()
	}
	return false
}

// joinChannelForCommunity adds someone to a channel of their community, quietly: no "joined the channel" message,
// those would flood a community's channels. Their own clients hear of it; with announce, the channel's members too.
func (a *App) joinChannelForCommunity(rctx request.CTX, user *model.User, channel *model.Channel, announce bool) *model.AppError {
	if _, appErr := a.addUserToChannel(rctx, user, channel); appErr != nil {
		return appErr
	}
	a.addChannelToDefaultCategory(rctx, user.Id, channel)

	if announce {
		message := model.NewWebSocketEvent(model.WebsocketEventUserAdded, "", channel.Id, "", map[string]bool{user.Id: true}, "")
		message.Add("user_id", user.Id)
		message.Add("team_id", channel.TeamId)
		a.Publish(message)
	}
	userMessage := model.NewWebSocketEvent(model.WebsocketEventUserAdded, "", channel.Id, user.Id, nil, "")
	userMessage.Add("user_id", user.Id)
	userMessage.Add("team_id", channel.TeamId)
	a.Publish(userMessage)
	return nil
}

// fillChannelWithCommunity brings every member of a community (but guests and bots) into one of its channels (for a
// private channel, those of its roles), in batches.
func (a *App) fillChannelWithCommunity(rctx request.CTX, team *model.Team, channel *model.Channel) {
	if team == nil || !isCommunityWideChannel(channel) || channel.TeamId != team.Id {
		return
	}

	skipped := 0
	for {
		users, err := a.Srv().Store().User().GetProfilesNotInChannel(team.Id, channel.Id, channel.IsGroupConstrained(), skipped, communityFillBatchSize, nil)
		if err != nil {
			rctx.Logger().Warn("Failed to get the members of a community to fill a channel with", mlog.String("channel_id", channel.Id), mlog.Err(err))
			return
		}
		for _, user := range users {
			if !isCommunityWideUser(user) {
				skipped++
				continue
			}
			if appErr := a.joinChannelForCommunity(rctx, user, channel, false); appErr != nil {
				rctx.Logger().Warn("Failed to add a member of the community to a channel", mlog.String("channel_id", channel.Id), mlog.String("user_id", user.Id), mlog.Err(appErr))
				skipped++
			}
		}
		if len(users) < communityFillBatchSize {
			break
		}
		time.Sleep(communityFillPause)
	}

	a.invalidateCacheForChannelMembers(channel.Id)
}

// Default channels. A community's default channel is where newcomers land and its join and leave messages go: the
// public channel its admins chose (Team.DefaultChannelId), else General (named town-square, Town Square, before an
// upgrade renamed it). Either can be archived or deleted like any channel: the community then has no default channel,
// and people land in the first channel they're in.

// GetDefaultChannelForTeam gives a community's default channel, nil when it has none open to all (the chosen one, or
// General, archived, deleted or private).
func (a *App) GetDefaultChannelForTeam(rctx request.CTX, team *model.Team) *model.Channel {
	if team == nil {
		return nil
	}
	var channel *model.Channel
	var err error
	if team.DefaultChannelId != "" {
		channel, err = a.Srv().Store().Channel().Get(team.DefaultChannelId, true)
	} else {
		channel, err = a.Srv().Store().Channel().GetByName(team.Id, model.DefaultChannelName, true)
		var nfErr *store.ErrNotFound
		if err != nil && errors.As(err, &nfErr) {
			// Not renamed yet: a community imported after the upgrade, or created by a server of an older version.
			channel, err = a.Srv().Store().Channel().GetByName(team.Id, model.LegacyDefaultChannelName, true)
		}
	}
	if err != nil || channel.TeamId != team.Id || channel.DeleteAt != 0 || channel.Type != model.ChannelTypeOpen {
		return nil
	}
	return channel
}

// isDefaultChannel tells whether a channel is its community's default channel.
func (a *App) isDefaultChannel(rctx request.CTX, channel *model.Channel) bool {
	if channel == nil || channel.TeamId == "" || channel.Type != model.ChannelTypeOpen || channel.DeleteAt != 0 {
		return false
	}
	team, appErr := a.GetTeam(channel.TeamId)
	if appErr != nil {
		// As before default channels could be chosen.
		return channel.Name == model.DefaultChannelName || channel.Name == model.LegacyDefaultChannelName
	}
	defaultChannel := a.GetDefaultChannelForTeam(rctx, team)
	return defaultChannel != nil && defaultChannel.Id == channel.Id
}

// IsDefaultChannel tells whether a channel is its community's default channel.
func (a *App) IsDefaultChannel(rctx request.CTX, channel *model.Channel) bool {
	return a.isDefaultChannel(rctx, channel)
}

// checkDefaultChannelChoice accepts a community's choice of default channel: none, or one of its public channels.
func (a *App) checkDefaultChannelChoice(teamID, channelID string) *model.AppError {
	if channelID == "" {
		return nil
	}
	channel, err := a.Srv().Store().Channel().Get(channelID, true)
	if err != nil || channel.TeamId != teamID || channel.DeleteAt != 0 || channel.Type != model.ChannelTypeOpen {
		return model.NewAppError("PatchTeam", "app.team.default_channel.app_error", nil, "", http.StatusBadRequest)
	}
	return nil
}

// FillDefaultChannelInBackground brings everyone into the channel a community chose as its default channel: as General,
// everyone is in it.
func (a *App) FillDefaultChannelInBackground(rctx request.CTX, team *model.Team) {
	channel := a.GetDefaultChannelForTeam(rctx, team)
	if channel == nil || team.DefaultChannelId == "" {
		return
	}
	logger := rctx.Logger()
	a.Srv().Go(func() {
		a.fillChannelWithCommunity(request.EmptyContext(logger), team, channel)
	})
}

// The default channel's old name. It was town-square (Town Square) before it became general (General), and links,
// incoming webhooks, slash commands, apps and scripts still ask for it by that name. Once no channel of the community
// has it, they get the community's default channel instead: General, or the channel its admins chose. A community that
// had a general channel of its own kept its town-square (doGeneralChannelMigration), and still gets that one.

// legacyDefaultChannel gives the community's default channel for a lookup of town-square that found no channel, and
// nil for any other name, outside communities, or when the community has no default channel.
func (a *App) legacyDefaultChannel(rctx request.CTX, teamID, channelName string) *model.Channel {
	if channelName != model.LegacyDefaultChannelName || teamID == "" {
		return nil
	}
	team, appErr := a.GetTeam(teamID)
	if appErr != nil {
		return nil
	}
	return a.GetDefaultChannelForTeam(rctx, team)
}

// getChannelByNameOrLegacy looks a community's channel up by name as the store does, town-square giving the community's
// default channel when no channel has that name (legacyDefaultChannel).
func (a *App) getChannelByNameOrLegacy(rctx request.CTX, teamID, channelName string, includeDeleted, allowFromCache bool) (*model.Channel, error) {
	var channel *model.Channel
	var err error
	if includeDeleted {
		channel, err = a.Srv().Store().Channel().GetByNameIncludeDeleted(teamID, channelName, allowFromCache)
	} else {
		channel, err = a.Srv().Store().Channel().GetByName(teamID, channelName, allowFromCache)
	}
	var nfErr *store.ErrNotFound
	if err != nil && errors.As(err, &nfErr) {
		if legacy := a.legacyDefaultChannel(rctx, teamID, channelName); legacy != nil {
			return legacy, nil
		}
	}
	return channel, err
}
