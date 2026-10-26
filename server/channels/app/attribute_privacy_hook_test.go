// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/app/platform"
)

// Attribute value events honour the "attributes" profile privacy setting, as the REST reads do.
func TestAttributeValuesEventHonoursProfilePrivacy(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	guild := th.CreateTeam(t)
	workspace := th.CreateTeam(t)

	// Sofia hides her attributes from the guild and shows them in the workspace.
	sofia := th.CreateUser(t)
	th.LinkUserToTeam(t, sofia, guild)
	th.LinkUserToTeam(t, sofia, workspace)
	privacy, err := json.Marshal(model.ProfilePrivacy{
		guild.Id:     {model.ProfileFieldAttributes},
		workspace.Id: {},
	})
	require.NoError(t, err)
	sofia.Props = model.StringMap{model.UserPropsKeyProfilePrivacy: string(privacy)}
	sofia, appErr := th.App.UpdateUser(th.Context, sofia, false)
	require.Nil(t, appErr)

	member := func(teams ...*model.Team) *model.User {
		t.Helper()
		user := th.CreateUser(t)
		for _, team := range teams {
			th.LinkUserToTeam(t, user, team)
		}
		return user
	}

	fieldID := model.NewId()
	values := map[string]json.RawMessage{fieldID: json.RawMessage(`"+33 6 12 34 56 78"`)}
	withheld := map[string]json.RawMessage{fieldID: json.RawMessage(model.PropertyValueWithheldJSON)}

	message := model.NewWebSocketEvent(model.WebsocketEventCPAValuesUpdated, "", "", "", nil, "")
	message.Add("user_id", sofia.Id)
	th.App.addUserAttributeValues(th.Context, message, sofia.Id, values, withheld)

	// What goes out if the hook can't run: every value withheld.
	assert.Equal(t, withheld, message.GetData()["values"])

	_, hooks, args := message.WithoutBroadcastHooks()
	require.Equal(t, []string{broadcastAttributePrivacy}, hooks)
	hook := &attributePrivacyBroadcastHook{}
	process := func(viewer *model.User, roles string) string {
		t.Helper()
		wc := &platform.WebConn{Platform: th.Server.Platform(), Suite: th.App, UserId: viewer.Id}
		wc.SetSession(&model.Session{UserId: viewer.Id, Roles: roles})
		msg := platform.MakeHookedWebSocketEvent(message)
		require.NoError(t, hook.Process(msg, wc, args[0]))
		seen, err := json.Marshal(msg.Get("values"))
		require.NoError(t, err)
		return string(seen)
	}

	t.Run("someone met in the guild gets the values withheld", func(t *testing.T) {
		seen := process(member(guild), model.SystemUserRoleId)
		assert.NotContains(t, seen, "+33")
		assert.Contains(t, seen, "withheld")
	})

	t.Run("someone met only in the workspace gets them", func(t *testing.T) {
		assert.Contains(t, process(member(workspace), model.SystemUserRoleId), "+33")
	})

	t.Run("someone met in both gets them withheld", func(t *testing.T) {
		assert.NotContains(t, process(member(guild, workspace), model.SystemUserRoleId), "+33")
	})

	t.Run("she and system admins get them", func(t *testing.T) {
		assert.Contains(t, process(sofia, model.SystemUserRoleId), "+33")
		assert.Contains(t, process(th.SystemAdminUser, model.SystemAdminRoleId+" "+model.SystemUserRoleId), "+33")
	})

	t.Run("someone who hides nothing goes out as before, without the hook", func(t *testing.T) {
		user := th.CreateUser(t)
		plain := model.NewWebSocketEvent(model.WebsocketEventCPAValuesUpdated, "", "", "", nil, "")
		th.App.addUserAttributeValues(th.Context, plain, user.Id, values, withheld)
		_, hooks, _ := plain.WithoutBroadcastHooks()
		assert.Empty(t, hooks)
		assert.Equal(t, values, plain.GetData()["values"])
	})

	t.Run("someone who hides only other details goes out as before", func(t *testing.T) {
		user := th.CreateUser(t)
		onlyName, err := json.Marshal(model.ProfilePrivacy{model.ProfilePrivacyDefault: {model.ProfileFieldFullName}})
		require.NoError(t, err)
		user.Props = model.StringMap{model.UserPropsKeyProfilePrivacy: string(onlyName)}
		user, appErr := th.App.UpdateUser(th.Context, user, false)
		require.Nil(t, appErr)
		plain := model.NewWebSocketEvent(model.WebsocketEventCPAValuesUpdated, "", "", "", nil, "")
		th.App.addUserAttributeValues(th.Context, plain, user.Id, values, withheld)
		_, hooks, _ := plain.WithoutBroadcastHooks()
		assert.Empty(t, hooks)
		assert.Equal(t, values, plain.GetData()["values"])
	})
}
