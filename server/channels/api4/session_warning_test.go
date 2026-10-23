// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// waitForSessionEvent is the next event of the kind on the connection, or fails.
func waitForSessionEvent(t *testing.T, ws *model.WebSocketClient, kind model.WebsocketEventType) *model.WebSocketEvent {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case event := <-ws.EventChannel:
			if event.EventType() == kind {
				return event
			}
		case <-timeout:
			require.FailNow(t, "no "+string(kind)+" event")
			return nil
		}
	}
}

// noSessionEvent checks the connection gets no sign-in event (of any kind) for a second.
func noSessionEvent(t *testing.T, ws *model.WebSocketClient, why string) {
	t.Helper()
	timeout := time.After(time.Second)
	for {
		select {
		case event := <-ws.EventChannel:
			switch event.EventType() {
			case model.WebsocketEventSessionCreated, model.WebsocketEventSessionRevoked, model.WebsocketEventSessionAcknowledged:
				require.FailNow(t, why, "got %s", event.EventType())
			}
		case <-timeout:
			return
		}
	}
}

func sessionOf(t *testing.T, th *TestHelper, client *model.Client4) *model.Session {
	t.Helper()
	session, appErr := th.App.GetSession(client.AuthToken)
	require.Nil(t, appErr)
	return session
}

func TestSignInWarnings(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()
	me := th.BasicUser

	t.Run("a sign-in warns the user's other sessions, without its token, and nobody else", func(t *testing.T) {
		wsMine := th.CreateConnectedWebSocketClient(t)
		other := th.CreateClient()
		th.LoginBasic2WithClient(t, other)
		wsOther := th.CreateConnectedWebSocketClientWithClient(t, other)
		wsAdmin := th.CreateConnectedWebSocketClientWithClient(t, th.SystemAdminClient)

		second := th.CreateClient()
		second.HTTPHeader = map[string]string{"User-Agent": "Mozilla/5.0 (X11; Linux x86_64; rv:131.0) Gecko/20100101 Firefox/131.0"}
		th.LoginBasicWithClient(t, second)
		signedIn := sessionOf(t, th, second)

		event := waitForSessionEvent(t, wsMine, model.WebsocketEventSessionCreated)
		assert.Equal(t, me.Id, event.GetBroadcast().UserId)
		data := event.GetData()
		assert.Equal(t, signedIn.Id, data["session_id"])
		assert.EqualValues(t, signedIn.CreateAt, data["create_at"])
		assert.Equal(t, "Firefox/131.0", data["browser"])
		assert.Equal(t, "Linux", data["os"])
		assert.Equal(t, false, data["mobile"])
		for k, v := range data {
			assert.NotContains(t, []string{"token", "csrf", "device_id", "voip_device_id", "props"}, k)
			assert.NotEqual(t, signedIn.Token, v, k)
			assert.NotEqual(t, signedIn.GetCSRF(), v, k)
		}

		noSessionEvent(t, wsOther, "another user isn't told")
		noSessionEvent(t, wsAdmin, "nor a system admin")

		// The session that signed in, once connected, has nothing to be warned of.
		wsSecond := th.CreateConnectedWebSocketClientWithClient(t, second)
		noSessionEvent(t, wsSecond, "the new session isn't warned of itself")
	})

	t.Run("a system admin's sign-in warns only their own sessions", func(t *testing.T) {
		wsMine := th.CreateConnectedWebSocketClient(t)
		wsAdmin := th.CreateConnectedWebSocketClientWithClient(t, th.SystemAdminClient)

		admin2 := th.CreateClient()
		th.LoginSystemAdminWithClient(t, admin2)
		event := waitForSessionEvent(t, wsAdmin, model.WebsocketEventSessionCreated)
		assert.Equal(t, sessionOf(t, th, admin2).Id, event.GetData()["session_id"])
		noSessionEvent(t, wsMine, "nobody else hears of an admin's sign-in")
	})

	t.Run("a personal access token is no sign-in", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })
		_, appErr := th.App.UpdateUserRoles(th.Context, me.Id, model.SystemUserRoleId+" "+model.SystemUserAccessTokenRoleId, false)
		require.Nil(t, appErr)
		token, _, err := th.SystemAdminClient.CreateUserAccessToken(ctx, me.Id, "script", 0)
		require.NoError(t, err)

		wsMine := th.CreateConnectedWebSocketClient(t)
		script := th.CreateClient()
		script.SetToken(token.Token)
		_, _, err = script.GetMe(ctx, "")
		require.NoError(t, err)
		noSessionEvent(t, wsMine, "no warning for a personal access token")
	})

	t.Run("revoking the session takes its warning off the user's sessions", func(t *testing.T) {
		second := th.CreateClient()
		th.LoginBasicWithClient(t, second)
		signedIn := sessionOf(t, th, second)

		wsMine := th.CreateConnectedWebSocketClient(t)
		wsOther := th.CreateConnectedWebSocketClientWithClient(t, th.SystemAdminClient)

		_, err := th.Client.RevokeSession(ctx, model.Me, signedIn.Id)
		require.NoError(t, err)
		event := waitForSessionEvent(t, wsMine, model.WebsocketEventSessionRevoked)
		assert.Equal(t, signedIn.Id, event.GetData()["session_id"])
		assert.Len(t, event.GetData(), 1, "the session's id only")
		noSessionEvent(t, wsOther, "nobody else hears of it")

		_, _, err = second.GetMe(ctx, "")
		require.Error(t, err, "the session is gone")
	})

	t.Run("a user can revoke only their own sessions", func(t *testing.T) {
		second := th.CreateClient()
		th.LoginBasicWithClient(t, second)
		signedIn := sessionOf(t, th, second)

		other := th.CreateClient()
		th.LoginBasic2WithClient(t, other)

		resp, err := other.RevokeSession(ctx, me.Id, signedIn.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		resp, err = other.RevokeSession(ctx, model.Me, signedIn.Id)
		require.Error(t, err, "not someone else's session through one's own id either")
		CheckBadRequestStatus(t, resp)

		_, _, err = second.GetMe(ctx, "")
		require.NoError(t, err, "the session is still there")
	})

	t.Run("\"It's me\": only the user, for their own session; the warning goes from their sessions", func(t *testing.T) {
		second := th.CreateClient()
		th.LoginBasicWithClient(t, second)
		signedIn := sessionOf(t, th, second)

		wsMine := th.CreateConnectedWebSocketClient(t)
		wsAdmin := th.CreateConnectedWebSocketClientWithClient(t, th.SystemAdminClient)
		other := th.CreateClient()
		th.LoginBasic2WithClient(t, other)

		// Nobody else may confirm it, a system admin included.
		resp, err := other.AcknowledgeSession(ctx, me.Id, signedIn.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		resp, err = th.SystemAdminClient.AcknowledgeSession(ctx, me.Id, signedIn.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		resp, err = other.AcknowledgeSession(ctx, model.Me, signedIn.Id)
		require.Error(t, err, "not someone else's session through one's own id either")
		CheckBadRequestStatus(t, resp)
		noSessionEvent(t, wsMine, "nothing confirmed")

		resp, err = th.Client.AcknowledgeSession(ctx, model.Me, "not-an-id")
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		resp, err = th.Client.AcknowledgeSession(ctx, model.Me, model.NewId())
		require.Error(t, err, "no such session")
		CheckBadRequestStatus(t, resp)

		resp, err = th.Client.AcknowledgeSession(ctx, model.Me, signedIn.Id)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		event := waitForSessionEvent(t, wsMine, model.WebsocketEventSessionAcknowledged)
		assert.Equal(t, signedIn.Id, event.GetData()["session_id"])
		assert.Len(t, event.GetData(), 1, "the session's id only")
		noSessionEvent(t, wsAdmin, "nobody else hears of it")

		_, _, err = second.GetMe(ctx, "")
		require.NoError(t, err, "confirming leaves the session be")
	})

	t.Run("signed out: no confirming", func(t *testing.T) {
		client := th.CreateClient()
		resp, err := client.AcknowledgeSession(ctx, model.Me, model.NewId())
		require.Error(t, err)
		CheckUnauthorizedStatus(t, resp)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}
