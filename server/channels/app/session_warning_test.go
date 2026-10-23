// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
	"github.com/mattermost/mattermost/server/v8/channels/app/platform"
)

func TestSessionCreatedEventContent(t *testing.T) {
	session := &model.Session{
		Id:           model.NewId(),
		Token:        model.NewId(),
		UserId:       model.NewId(),
		CreateAt:     1761210000000,
		DeviceId:     "android_rn-v2:secret-push-token",
		VoIPDeviceId: "apple_voip:secret-voip-token",
		Roles:        model.SystemUserRoleId,
		Props: model.StringMap{
			model.SessionPropPlatform:     "Linux",
			model.SessionPropOs:           "Android",
			model.SessionPropBrowser:      "Mobile App/2.30",
			model.UserAuthServiceIsMobile: "true",
		},
	}
	session.GenerateCSRF()
	csrf := session.GetCSRF()
	require.NotEmpty(t, csrf)

	event := sessionCreatedEvent(session)
	assert.Equal(t, model.WebsocketEventSessionCreated, event.EventType())
	assert.Equal(t, session.UserId, event.GetBroadcast().UserId, "to the user's own sessions only")
	assert.Empty(t, event.GetBroadcast().ChannelId)
	assert.Empty(t, event.GetBroadcast().TeamId)

	data := event.GetData()
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	assert.ElementsMatch(t, []string{"session_id", "create_at", "platform", "os", "browser", "mobile"}, keys, "only the safe metadata")
	assert.Equal(t, session.Id, data["session_id"])
	assert.Equal(t, session.CreateAt, data["create_at"])
	assert.Equal(t, "Linux", data["platform"])
	assert.Equal(t, "Android", data["os"])
	assert.Equal(t, "Mobile App/2.30", data["browser"])
	assert.Equal(t, true, data["mobile"])

	// Nothing secret, anywhere in what goes over the wire: not the token, the CSRF token, nor the push tokens.
	raw, err := event.ToJSON()
	require.NoError(t, err)
	for _, secret := range []string{session.Token, csrf, session.DeviceId, session.VoIPDeviceId, "secret-push-token", "secret-voip-token"} {
		assert.NotContains(t, string(raw), secret)
	}
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	assert.NotContains(t, decoded["data"], "token")
	assert.NotContains(t, decoded["data"], "csrf")
	assert.NotContains(t, decoded["data"], "device_id")
	assert.NotContains(t, decoded["data"], "props")
}

func TestWarnsOfSignIn(t *testing.T) {
	signIn := &model.Session{Id: model.NewId(), UserId: model.NewId()}
	assert.True(t, warnsOfSignIn(signIn), "a sign-in")

	mobile := &model.Session{Id: model.NewId(), UserId: model.NewId(), DeviceId: "android_rn-v2:x", Props: model.StringMap{model.UserAuthServiceIsMobile: "true"}}
	assert.True(t, warnsOfSignIn(mobile), "the mobile app signing in")

	sso := &model.Session{Id: model.NewId(), UserId: model.NewId(), Props: model.StringMap{model.UserAuthServiceIsOAuth: "true"}}
	assert.True(t, warnsOfSignIn(sso), "an SSO sign-in")

	pat := &model.Session{Id: model.NewId(), UserId: model.NewId(), Props: model.StringMap{model.SessionPropType: model.SessionTypeUserAccessToken}}
	assert.False(t, warnsOfSignIn(pat), "not a personal access token")

	oauthApp := &model.Session{Id: model.NewId(), UserId: model.NewId(), IsOAuth: true}
	assert.False(t, warnsOfSignIn(oauthApp), "not an OAuth app's token")

	bot := &model.Session{Id: model.NewId(), UserId: model.NewId(), Props: model.StringMap{model.SessionPropIsBot: model.SessionPropIsBotValue}}
	assert.False(t, warnsOfSignIn(bot), "not a bot")

	assert.False(t, warnsOfSignIn(nil))
	assert.False(t, warnsOfSignIn(&model.Session{UserId: model.NewId()}), "not a session without an id")
}

func TestSessionWarningDelivery(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	conn := func(user *model.User) (*platform.WebConn, *model.Session) {
		session, appErr := th.App.CreateSession(th.Context, &model.Session{UserId: user.Id, Roles: user.GetRawRoles(), Props: model.StringMap{model.SessionPropIsGuest: "false"}})
		require.Nil(t, appErr)
		wc := &platform.WebConn{Platform: th.Server.Platform(), Suite: th.App, UserId: user.Id, T: i18n.T}
		wc.SetConnectionID(model.NewId())
		wc.SetSession(session)
		wc.SetSessionToken(session.Token)
		wc.SetSessionExpiresAt(session.ExpiresAt)
		return wc, session
	}

	mine, _ := conn(th.BasicUser)
	newConn, newSession := conn(th.BasicUser)
	other, _ := conn(th.BasicUser2)
	admin, _ := conn(th.SystemAdminUser)

	event := sessionCreatedEvent(newSession)
	assert.True(t, mine.ShouldSendEvent(event), "the user's other session is warned")
	assert.False(t, newConn.ShouldSendEvent(event), "not the session that signed in")
	assert.False(t, other.ShouldSendEvent(event), "nobody else")
	assert.False(t, admin.ShouldSendEvent(event), "not a system admin either")

	// The same once through the cluster (decoded from JSON).
	raw, err := event.ToJSON()
	require.NoError(t, err)
	decoded, err := model.WebSocketEventFromJSON(bytes.NewReader(raw))
	require.NoError(t, err)
	assert.True(t, mine.ShouldSendEvent(decoded))
	assert.False(t, newConn.ShouldSendEvent(decoded))
	assert.False(t, other.ShouldSendEvent(decoded))
	assert.False(t, admin.ShouldSendEvent(decoded))

	// An event without a session id goes nowhere.
	empty := model.NewWebSocketEvent(model.WebsocketEventSessionCreated, "", "", th.BasicUser.Id, nil, "")
	assert.False(t, mine.ShouldSendEvent(empty))

	// Revoked or confirmed: the user's sessions hear of it, nobody else.
	for _, kind := range []model.WebsocketEventType{model.WebsocketEventSessionRevoked, model.WebsocketEventSessionAcknowledged} {
		handled := model.NewWebSocketEvent(kind, "", "", th.BasicUser.Id, nil, "")
		handled.Add("session_id", newSession.Id)
		assert.True(t, mine.ShouldSendEvent(handled), kind)
		assert.False(t, other.ShouldSendEvent(handled), kind)
		assert.False(t, admin.ShouldSendEvent(handled), kind)
	}
}

func TestAcknowledgeSession(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	mine, appErr := th.App.CreateSession(th.Context, &model.Session{UserId: th.BasicUser.Id, Roles: th.BasicUser.GetRawRoles()})
	require.Nil(t, appErr)
	theirs, appErr := th.App.CreateSession(th.Context, &model.Session{UserId: th.BasicUser2.Id, Roles: th.BasicUser2.GetRawRoles()})
	require.Nil(t, appErr)

	assert.Nil(t, th.App.AcknowledgeSession(th.Context, th.BasicUser.Id, mine.Id), "one's own session")

	appErr = th.App.AcknowledgeSession(th.Context, th.BasicUser.Id, theirs.Id)
	require.NotNil(t, appErr, "not someone else's")
	missing := th.App.AcknowledgeSession(th.Context, th.BasicUser.Id, model.NewId())
	require.NotNil(t, missing, "nor one that doesn't exist")
	assert.Equal(t, appErr.Id, missing.Id, "refused the same way: no telling whose a session is")
	assert.Equal(t, appErr.StatusCode, missing.StatusCode)
}
