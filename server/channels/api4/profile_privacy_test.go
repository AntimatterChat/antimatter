// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// Profile privacy, as a regression test of every route that gives out someone's profile: members of a community where
// someone hides details never get them, system admins and the person themselves do, and nobody gets the choice itself.
func TestProfilePrivacyRoutes(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.PrivacySettings.ShowEmailAddress = true
		*cfg.PrivacySettings.ShowFullName = true
	})

	// BasicUser2 hides everything from BasicTeam (where BasicUser meets them), nothing from the open community.
	open := th.CreateTeam(t)
	th.LinkUserToTeam(t, th.BasicUser2, open)
	privacy, err := json.Marshal(model.ProfilePrivacy{th.BasicTeam.Id: {
		model.ProfileFieldFullName, model.ProfileFieldNickname, model.ProfileFieldPosition, model.ProfileFieldEmail, model.ProfileFieldLocalTime,
	}})
	require.NoError(t, err)
	// A copy: updating it clears its password, which the logins below need.
	hider := th.BasicUser2.DeepCopy()
	hider.FirstName, hider.LastName, hider.Nickname, hider.Position = "Leonie", "Quartermaine", "Leo", "Captain"
	hider.Timezone = model.StringMap{"useAutomaticTimezone": "false", "manualTimezone": "Europe/Paris"}
	hider.Props = model.StringMap{model.UserPropsKeyProfilePrivacy: string(privacy)}
	hider, appErr := th.App.UpdateUser(th.Context, hider, false)
	require.Nil(t, appErr)

	assertHidden := func(t *testing.T, user *model.User) {
		t.Helper()
		require.NotNil(t, user)
		assert.Equal(t, hider.Id, user.Id)
		assert.Equal(t, hider.Username, user.Username)
		assert.Empty(t, user.FirstName)
		assert.Empty(t, user.LastName)
		assert.Empty(t, user.Nickname)
		assert.Empty(t, user.Position)
		assert.Empty(t, user.Email)
		assert.Empty(t, user.Timezone)
		assert.NotContains(t, user.Props, model.UserPropsKeyProfilePrivacy)
	}
	assertShown := func(t *testing.T, user *model.User) {
		t.Helper()
		require.NotNil(t, user)
		assert.Equal(t, "Quartermaine", user.LastName)
		assert.Equal(t, "Leo", user.Nickname)
		assert.Equal(t, "Captain", user.Position)
		assert.Equal(t, hider.Email, user.Email)
		assert.NotEmpty(t, user.Timezone)
	}
	find := func(users []*model.User) *model.User {
		for _, u := range users {
			if u.Id == hider.Id {
				return u
			}
		}
		return nil
	}

	t.Run("one profile, by id, username or email", func(t *testing.T) {
		user, _, err := th.Client.GetUser(ctx, hider.Id, "")
		require.NoError(t, err)
		assertHidden(t, user)

		user, _, err = th.Client.GetUserByUsername(ctx, hider.Username, "")
		require.NoError(t, err)
		assertHidden(t, user)

		// Nobody is found by the email they hide.
		_, resp, err := th.Client.GetUserByEmail(ctx, hider.Email, "")
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("the etag changes with what's hidden", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.GetUser(ctx, hider.Id, "")
		require.NoError(t, err)
		// A member doesn't get a 304 for the admin's copy.
		user, resp2, err := th.Client.GetUser(ctx, hider.Id, resp.Etag)
		require.NoError(t, err)
		assert.NotEqual(t, http.StatusNotModified, resp2.StatusCode)
		assertHidden(t, user)
	})

	t.Run("lists of profiles", func(t *testing.T) {
		users, _, err := th.Client.GetUsersByIds(ctx, []string{hider.Id})
		require.NoError(t, err)
		assertHidden(t, find(users))

		users, _, err = th.Client.GetUsersByUsernames(ctx, []string{hider.Username})
		require.NoError(t, err)
		assertHidden(t, find(users))

		users, _, err = th.Client.GetUsersInTeam(ctx, th.BasicTeam.Id, 0, 100, "")
		require.NoError(t, err)
		assertHidden(t, find(users))

		users, _, err = th.Client.GetUsersInChannel(ctx, th.BasicChannel.Id, 0, 100, "")
		require.NoError(t, err)
		assertHidden(t, find(users))

		gm, _, err := th.Client.CreateGroupChannel(ctx, []string{th.BasicUser.Id, hider.Id, th.SystemAdminUser.Id})
		require.NoError(t, err)
		byChannel, _, err := th.Client.GetUsersByGroupChannelIds(ctx, []string{gm.Id})
		require.NoError(t, err)
		assertHidden(t, find(byChannel[gm.Id]))
	})

	t.Run("search and autocomplete neither give the details nor find by them", func(t *testing.T) {
		users, _, err := th.Client.SearchUsers(ctx, &model.UserSearch{Term: hider.Username, TeamId: th.BasicTeam.Id})
		require.NoError(t, err)
		assertHidden(t, find(users))

		for _, term := range []string{"Quartermaine", "Leonie", "Captain"} {
			users, _, err = th.Client.SearchUsers(ctx, &model.UserSearch{Term: term, TeamId: th.BasicTeam.Id})
			require.NoError(t, err)
			assert.Nil(t, find(users), term)
		}
		users, _, err = th.Client.SearchUsers(ctx, &model.UserSearch{Term: hider.Email})
		require.NoError(t, err)
		assert.Nil(t, find(users))

		auto, _, err := th.Client.AutocompleteUsersInChannel(ctx, th.BasicTeam.Id, th.BasicChannel.Id, "quarter", 100, "")
		require.NoError(t, err)
		assert.Nil(t, find(auto.Users))
		assert.Nil(t, find(auto.OutOfChannel))

		auto, _, err = th.Client.AutocompleteUsersInTeam(ctx, th.BasicTeam.Id, hider.Username, 100, "")
		require.NoError(t, err)
		assertHidden(t, find(auto.Users))

		// A system admin finds them by name.
		users, _, err = th.SystemAdminClient.SearchUsers(ctx, &model.UserSearch{Term: "Quartermaine"})
		require.NoError(t, err)
		assertShown(t, find(users))
	})

	t.Run("system admins see everything; nobody but the person sees the choice", func(t *testing.T) {
		user, _, err := th.SystemAdminClient.GetUser(ctx, hider.Id, "")
		require.NoError(t, err)
		assertShown(t, user)

		client := th.CreateClient()
		th.LoginBasic2WithClient(t, client)
		me, _, err := client.GetMe(ctx, "")
		require.NoError(t, err)
		assertShown(t, me)
		assert.Equal(t, string(privacy), me.Props[model.UserPropsKeyProfilePrivacy])
	})

	t.Run("someone met only in the open community sees everything", func(t *testing.T) {
		stranger := th.CreateUser(t)
		th.LinkUserToTeam(t, stranger, open)
		client := th.CreateClient()
		_, _, err := client.Login(ctx, stranger.Email, stranger.Password)
		require.NoError(t, err)
		user, _, err := client.GetUser(ctx, hider.Id, "")
		require.NoError(t, err)
		assertShown(t, user)
		assert.NotContains(t, user.Props, model.UserPropsKeyProfilePrivacy)
	})

	t.Run("someone in both gets the most restrictive", func(t *testing.T) {
		th.LinkUserToTeam(t, th.BasicUser, open)
		user, _, err := th.Client.GetUser(ctx, hider.Id, "")
		require.NoError(t, err)
		assertHidden(t, user)
	})

	t.Run("websocket: a profile update reaches each person as they may see it", func(t *testing.T) {
		memberWS := th.CreateConnectedWebSocketClient(t)
		adminWS := th.CreateConnectedWebSocketClientWithClient(t, th.SystemAdminClient)

		client := th.CreateClient()
		th.LoginBasic2WithClient(t, client)
		_, _, err := client.PatchUser(ctx, hider.Id, &model.UserPatch{Position: model.NewPointer("Admiral")})
		require.NoError(t, err)

		userIn := func(ws *model.WebSocketClient) *model.User {
			t.Helper()
			deadline := time.After(5 * time.Second)
			for {
				select {
				case event := <-ws.EventChannel:
					if event.EventType() != model.WebsocketEventUserUpdated {
						continue
					}
					user, ok := event.GetData()["user"].(*model.User)
					if ok && user.Id == hider.Id {
						return user
					}
				case <-deadline:
					require.FailNow(t, "no user_updated event")
				}
			}
		}
		seen := userIn(memberWS)
		assert.Empty(t, seen.Position)
		assert.Empty(t, seen.LastName)
		assert.NotContains(t, seen.Props, model.UserPropsKeyProfilePrivacy)

		seen = userIn(adminWS)
		assert.Equal(t, "Admiral", seen.Position)
		assert.Equal(t, "Quartermaine", seen.LastName)
	})

	t.Run("a choice that can't be read is refused", func(t *testing.T) {
		client := th.CreateClient()
		th.LoginBasic2WithClient(t, client)
		props := model.StringMap{model.UserPropsKeyProfilePrivacy: `{"default":["password"]}`}
		_, resp, err := client.PatchUser(ctx, hider.Id, &model.UserPatch{Props: props})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})
}
