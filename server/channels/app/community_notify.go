// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Notification settings per community (Antimatter, see model/community_notify.go): someone's settings for a community
// are set over their own for the notifications of its channels. The channels' own settings still win, since they're
// read before the person's (a channel left at "default" takes the person's, here with the community's over them).

// communityNotifyProps gives, by person, the notification settings they set for a community: one query, by the
// preferences' name.
func (a *App) communityNotifyProps(teamID string) (map[string]model.StringMap, error) {
	prefs, err := a.Srv().Store().Preference().GetCategoryAndName(model.PreferenceCategoryCommunityNotify, teamID)
	if err != nil {
		return nil, err
	}
	byUser := make(map[string]model.StringMap, len(prefs))
	for _, pref := range prefs {
		if props, ok := model.ParseCommunityNotifyProps(pref.Value); ok && len(props) > 0 {
			byUser[pref.UserId] = props
		}
	}
	return byUser, nil
}

// withCommunityNotifyProps gives the profiles of a channel's members (by id) with their settings for its community
// over their own: copies, the map too, when someone set some; the profiles themselves otherwise. Channels of no
// community (direct and group messages) follow the person's own settings.
func (a *App) withCommunityNotifyProps(rctx request.CTX, teamID string, profileMap map[string]*model.User) map[string]*model.User {
	if teamID == "" || len(profileMap) == 0 {
		return profileMap
	}
	overrides, err := a.communityNotifyProps(teamID)
	if err != nil {
		rctx.Logger().Warn("Failed to get the notification settings of a community", mlog.String("team_id", teamID), mlog.Err(err))
		return profileMap
	}
	if len(overrides) == 0 {
		return profileMap
	}
	updated := make(map[string]*model.User, len(profileMap))
	for id, user := range profileMap {
		if props, ok := overrides[id]; ok && user != nil {
			updated[id] = user.WithCommunityNotifyProps(props)
		} else {
			updated[id] = user
		}
	}
	return updated
}
