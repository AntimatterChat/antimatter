// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"strings"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app/platform"
)

const broadcastAttributePrivacy = "attribute_privacy"

// attributePrivacyBroadcastHook gives each person the attribute values of an attribute value event
// (custom_profile_attributes_values_updated, property_values_updated) as they may see them (Antimatter's profile
// privacy, see profile_privacy.go): the event itself carries every value withheld, which is what goes out should
// anything fail, and clients refetch withheld values through the privacy-checked REST read. The hook puts the values
// back for the user themselves, system admins, and those the communities they share with the user don't hide the
// attributes from.
type attributePrivacyBroadcastHook struct{}

// PublishUserAttributeValues publishes message, an event about userID's attribute values, with values (already
// passed through model.BroadcastUserAttributeValue) under "values" for whoever may see userID's attributes and
// withheld (the same values, each withheld) for the others. It's the plain event when userID hides their attributes
// nowhere.
func (a *App) PublishUserAttributeValues(rctx request.CTX, message *model.WebSocketEvent, userID string, values, withheld any) {
	a.addUserAttributeValues(rctx, message, userID, values, withheld)
	a.Publish(message)
}

// addUserAttributeValues puts the values in message as PublishUserAttributeValues sends them.
func (a *App) addUserAttributeValues(rctx request.CTX, message *model.WebSocketEvent, userID string, values, withheld any) {
	user, err := a.Srv().Store().User().Get(rctx, userID)
	if err != nil {
		// Nobody gets values that might be hidden from them.
		message.Add("values", withheld)
		return
	}
	hidesAttributes := user.HasProfilePrivacy()
	if hidesAttributes {
		if privacy, ok := user.GetProfilePrivacy(); ok {
			hidesAttributes = privacy.AllHidden()[model.ProfileFieldAttributes]
		}
	}
	if !hidesAttributes {
		message.Add("values", values)
		return
	}
	message.Add("values", withheld)
	teamIDs, err := a.Srv().Store().Team().GetUserTeamIds(userID, true)
	if err != nil {
		return
	}
	message.GetBroadcast().AddHook(broadcastAttributePrivacy, map[string]any{
		"user_id": userID,
		"privacy": user.Props[model.UserPropsKeyProfilePrivacy],
		"teams":   model.StringArray(teamIDs),
		"values":  values,
	})
}

func (h *attributePrivacyBroadcastHook) Process(msg *platform.HookedWebSocketEvent, webConn *platform.WebConn, args map[string]any) error {
	userID, err := getTypedArg[string](args, "user_id")
	if err != nil || userID == "" {
		return errors.Wrap(err, "Invalid user_id value passed to attributePrivacyBroadcastHook")
	}
	rawPrivacy, err := getTypedArg[string](args, "privacy")
	if err != nil {
		return errors.Wrap(err, "Invalid privacy value passed to attributePrivacyBroadcastHook")
	}
	teams, err := getTypedArg[model.StringArray](args, "teams")
	if err != nil {
		return errors.Wrap(err, "Invalid teams value passed to attributePrivacyBroadcastHook")
	}
	values, ok := args["values"]
	if !ok {
		return errors.New("No values passed to attributePrivacyBroadcastHook")
	}

	if webConn.UserId == userID {
		msg.Add("values", values)
		return nil
	}
	if session := webConn.GetSession(); session != nil && webConn.Suite.RolesGrantPermission(strings.Fields(session.Roles), model.PermissionManageSystem.Id) {
		msg.Add("values", values)
		return nil
	}

	privacy, ok := model.ParseProfilePrivacy(rawPrivacy)
	if !ok {
		// The values stay withheld.
		return nil
	}
	mine, err := webConn.Platform.Store.Team().GetUserTeamIds(webConn.UserId, true)
	if err != nil {
		return errors.Wrap(err, "attributePrivacyBroadcastHook couldn't read the recipient's communities")
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
	if !privacy.HiddenFrom(shared)[model.ProfileFieldAttributes] {
		msg.Add("values", values)
	}
	return nil
}
