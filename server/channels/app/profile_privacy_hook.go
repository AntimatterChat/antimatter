// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"strings"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/app/platform"
)

const broadcastProfilePrivacy = "profile_privacy"

// profilePrivacyBroadcastHook gives each person the profile of a user_updated event as they may see it (Antimatter's
// profile privacy, see profile_privacy.go). The event itself carries the profile with every detail hidden somewhere
// blanked, which is what goes out should anything fail; the hook puts back what this person sees: everything for the
// user themselves and system admins, else what the communities they share with the user show.
type profilePrivacyBroadcastHook struct{}

// useProfilePrivacyHook puts user (a sanitised copy, its profile privacy prop included) in message, as each recipient
// may see it. teamIDs are the user's communities.
func useProfilePrivacyHook(message *model.WebSocketEvent, user *model.User, teamIDs []string) {
	if !user.HasProfilePrivacy() {
		message.Add("user", user)
		return
	}
	privacy, ok := user.GetProfilePrivacy()
	hidden := model.AllProfileFields()
	if ok {
		hidden = privacy.AllHidden()
	}
	base := user.DeepCopy()
	base.HideProfileFields(hidden)
	message.Add("user", base)
	message.GetBroadcast().AddHook(broadcastProfilePrivacy, map[string]any{
		"user":  user,
		"teams": model.StringArray(teamIDs),
	})
}

// publishUserProfile puts user in message for each recipient as they may see it (see useProfilePrivacyHook).
func (a *App) publishUserProfile(message *model.WebSocketEvent, user *model.User) {
	if !user.HasProfilePrivacy() {
		message.Add("user", user)
		return
	}
	teamIDs, err := a.Srv().Store().Team().GetUserTeamIds(user.Id, true)
	if err != nil {
		// Nobody gets more than what no community hides.
		base := user.DeepCopy()
		base.HideProfileFields(model.AllProfileFields())
		message.Add("user", base)
		return
	}
	useProfilePrivacyHook(message, user, teamIDs)
}

func (h *profilePrivacyBroadcastHook) Process(msg *platform.HookedWebSocketEvent, webConn *platform.WebConn, args map[string]any) error {
	user, err := getTypedArg[*model.User](args, "user")
	if err != nil || user == nil {
		return errors.Wrap(err, "Invalid user value passed to profilePrivacyBroadcastHook")
	}
	teams, err := getTypedArg[model.StringArray](args, "teams")
	if err != nil {
		return errors.Wrap(err, "Invalid teams value passed to profilePrivacyBroadcastHook")
	}

	if webConn.UserId == user.Id {
		msg.Add("user", user)
		return nil
	}
	if session := webConn.GetSession(); session != nil && webConn.Suite.RolesGrantPermission(strings.Fields(session.Roles), model.PermissionManageSystem.Id) {
		msg.Add("user", user)
		return nil
	}

	mine, err := webConn.Platform.Store.Team().GetUserTeamIds(webConn.UserId, true)
	if err != nil {
		// The event keeps the profile with everything hidden somewhere blanked.
		return errors.Wrap(err, "profilePrivacyBroadcastHook couldn't read the recipient's communities")
	}
	theirs := make(map[string]bool, len(teams))
	for _, id := range teams {
		theirs[id] = true
	}
	shared := make([]string, 0, len(mine))
	for _, id := range mine {
		if theirs[id] {
			shared = append(shared, id)
		}
	}

	seen := user.DeepCopy()
	seen.HideProfileFields(user.ProfileFieldsHiddenFrom(shared))
	msg.Add("user", seen)
	return nil
}
