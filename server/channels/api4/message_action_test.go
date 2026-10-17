// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func setupMessageActionsAPI(t *testing.T) (*TestHelper, *atomic.Int32, *httptest.Server) {
	th := Setup(t).InitBasic(t)
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableOutgoingWebhooks = true
		*cfg.ServiceSettings.AllowedUntrustedInternalConnections = "localhost,127.0.0.1"
		*cfg.GuestAccountsSettings.Enable = true
	})

	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ephemeral_text": "Filed"}`))
	}))
	t.Cleanup(ts.Close)
	return th, &calls, ts
}

func createMessageActionHook(t *testing.T, th *TestHelper, channelID, callbackURL string) *model.OutgoingWebhook {
	hook, appErr := th.App.CreateOutgoingWebhook(&model.OutgoingWebhook{
		TeamId:       th.BasicTeam.Id,
		ChannelId:    channelID,
		CreatorId:    th.BasicUser.Id,
		TriggerWhen:  model.OutgoingWebhookTriggerWhenMessageAction,
		CallbackURLs: []string{callbackURL},
		DisplayName:  "Archive",
		ContentType:  "application/json",
	})
	require.Nil(t, appErr)
	return hook
}

func loginNewClient(t *testing.T, th *TestHelper, user *model.User) *model.Client4 {
	client := th.CreateClient()
	_, _, err := client.Login(context.Background(), user.Email, user.Password)
	require.NoError(t, err)
	return client
}

func messageActionIDs(actions []*model.MessageAction) []string {
	ids := []string{}
	for _, a := range actions {
		ids = append(ids, a.Id)
	}
	return ids
}

func TestMessageActionsAsMember(t *testing.T) {
	mainHelper.Parallel(t)
	th, calls, ts := setupMessageActionsAPI(t)
	bound := createMessageActionHook(t, th, th.BasicChannel.Id, ts.URL)
	teamWide := createMessageActionHook(t, th, "", ts.URL+"/team")

	t.Run("a member lists the channel's message actions, without tokens nor URLs", func(t *testing.T) {
		actions, resp, err := th.Client.GetMessageActionsForChannel(context.Background(), th.BasicChannel.Id)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		assert.ElementsMatch(t, []string{bound.Id, teamWide.Id}, messageActionIDs(actions))
		js := string(model.ToJSON(actions))
		assert.NotContains(t, js, bound.Token)
		assert.NotContains(t, js, ts.URL)
	})

	t.Run("a member who isn't in a public channel can still read it, and send its messages", func(t *testing.T) {
		outsider := th.CreateUser(t)
		th.LinkUserToTeam(t, outsider, th.BasicTeam)
		client := loginNewClient(t, th, outsider)

		actions, _, err := client.GetMessageActionsForChannel(context.Background(), th.BasicChannel.Id)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{bound.Id, teamWide.Id}, messageActionIDs(actions))
	})

	t.Run("a member runs one, and gets the hook's ephemeral text", func(t *testing.T) {
		before := calls.Load()
		result, resp, err := th.Client.RunMessageAction(context.Background(), th.BasicPost.Id, bound.Id)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		assert.Equal(t, "Filed", result.EphemeralText)
		assert.Equal(t, before+1, calls.Load())
	})

	t.Run("a message action can't be run for another channel's message", func(t *testing.T) {
		post, _, err := th.Client.CreatePost(context.Background(), &model.Post{ChannelId: th.BasicChannel2.Id, Message: "elsewhere"})
		require.NoError(t, err)
		_, resp, err := th.Client.RunMessageAction(context.Background(), post.Id, bound.Id)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("not when outgoing webhooks are off", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOutgoingWebhooks = false })
		defer th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOutgoingWebhooks = true })
		_, resp, err := th.Client.RunMessageAction(context.Background(), th.BasicPost.Id, bound.Id)
		require.Error(t, err)
		CheckNotImplementedStatus(t, resp)
	})
}

func TestMessageActionsAsNonMember(t *testing.T) {
	mainHelper.Parallel(t)
	th, calls, ts := setupMessageActionsAPI(t)
	bound := createMessageActionHook(t, th, th.BasicChannel.Id, ts.URL)

	// Someone outside the community can't read its channels: they neither see nor run its actions.
	stranger := th.CreateUser(t)
	client := loginNewClient(t, th, stranger)

	_, resp, err := client.GetMessageActionsForChannel(context.Background(), th.BasicChannel.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, resp, err = client.RunMessageAction(context.Background(), th.BasicPost.Id, bound.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	t.Run("nor can a member of the community send a private channel's message they aren't in", func(t *testing.T) {
		member := th.CreateUser(t)
		th.LinkUserToTeam(t, member, th.BasicTeam)
		memberClient := loginNewClient(t, th, member)
		private, _, err := th.Client.CreatePost(context.Background(), &model.Post{ChannelId: th.BasicPrivateChannel.Id, Message: "private"})
		require.NoError(t, err)

		_, resp, err := memberClient.RunMessageAction(context.Background(), private.Id, bound.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		_, resp, err = memberClient.GetMessageActionsForChannel(context.Background(), th.BasicPrivateChannel.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	assert.Equal(t, int32(0), calls.Load(), "nothing was sent")
}

func TestMessageActionsAsGuest(t *testing.T) {
	mainHelper.Parallel(t)
	th, calls, ts := setupMessageActionsAPI(t)
	bound := createMessageActionHook(t, th, th.BasicChannel.Id, ts.URL)
	teamWide := createMessageActionHook(t, th, "", ts.URL+"/team")

	// CreateGuestAndClient adds the guest to BasicChannel.
	_, guestClient := th.CreateGuestAndClient(t)

	actions, _, err := guestClient.GetMessageActionsForChannel(context.Background(), th.BasicChannel.Id)
	require.NoError(t, err)
	assert.Equal(t, []string{bound.Id}, messageActionIDs(actions), "a guest only gets the actions bound to their channel")

	_, resp, err := guestClient.RunMessageAction(context.Background(), th.BasicPost.Id, teamWide.Id)
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)
	assert.Equal(t, int32(0), calls.Load())

	_, resp, err = guestClient.RunMessageAction(context.Background(), th.BasicPost.Id, bound.Id)
	require.NoError(t, err)
	CheckOKStatus(t, resp)
	assert.Equal(t, int32(1), calls.Load())
}

func TestMessageActionsInArchivedChannel(t *testing.T) {
	mainHelper.Parallel(t)
	th, calls, ts := setupMessageActionsAPI(t)

	channel := th.CreatePublicChannel(t)
	post := th.CreatePostWithClient(t, th.Client, channel)
	// Archiving a channel deletes the webhooks bound to it: a community-wide action still covers it.
	hook := createMessageActionHook(t, th, "", ts.URL)
	_, err := th.SystemAdminClient.DeleteChannel(context.Background(), channel.Id)
	require.NoError(t, err)

	// The message only goes out: sending it from an archived channel is allowed.
	_, resp, err := th.Client.RunMessageAction(context.Background(), post.Id, hook.Id)
	require.NoError(t, err)
	CheckOKStatus(t, resp)
	assert.Equal(t, int32(1), calls.Load())
}

func testPNG(t *testing.T) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for x := range 32 {
		for y := range 32 {
			img.Set(x, y, color.RGBA{R: 200, G: 40, B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestOutgoingWebhookIcon(t *testing.T) {
	mainHelper.Parallel(t)
	th, _, ts := setupMessageActionsAPI(t)
	hook := createMessageActionHook(t, th, th.BasicChannel.Id, ts.URL)

	t.Run("someone who may not manage the hook can't change its icon", func(t *testing.T) {
		client := loginNewClient(t, th, th.BasicUser2)
		_, resp, err := client.SetOutgoingWebhookIcon(context.Background(), hook.Id, testPNG(t))
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("an admin uploads it, and the members of the community see it", func(t *testing.T) {
		updated, resp, err := th.SystemAdminClient.SetOutgoingWebhookIcon(context.Background(), hook.Id, testPNG(t))
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		assert.True(t, updated.HasUploadedIcon())

		data, resp, err := th.Client.GetOutgoingWebhookIcon(context.Background(), hook.Id)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		assert.Equal(t, "image/png", resp.Header.Get("Content-Type"))
		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		require.NoError(t, err)
		assert.Equal(t, "png", format)
		assert.Equal(t, 128, cfg.Width)

		actions, _, err := th.Client.GetMessageActionsForChannel(context.Background(), th.BasicChannel.Id)
		require.NoError(t, err)
		require.Len(t, actions, 1)
		assert.Equal(t, updated.IconURL, actions[0].IconURL)
	})

	t.Run("not to people outside the community", func(t *testing.T) {
		client := loginNewClient(t, th, th.CreateUser(t))
		_, resp, err := client.GetOutgoingWebhookIcon(context.Background(), hook.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("a file that isn't an image is refused", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.SetOutgoingWebhookIcon(context.Background(), hook.Id, []byte("<svg onload=alert(1)></svg>"))
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})
}
