// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// setupAssistantAPI turns the assistant on with two patterns over the server's own tools: "who is @someone" looks
// them up (an insight), "say hello" posts (an action). Windows run as soon as a message is saved.
func setupAssistantAPI(t *testing.T) (*TestHelper, aiTestClients, *model.Client4, *model.Channel) {
	th, clients := setupAIConnectionsAPI(t)
	th.App.UpdateConfig(func(cfg *model.Config) {
		s := cfg.AISettings.Assistant
		s.Enable = model.NewPointer(true)
		s.DebounceSeconds = model.NewPointer(0)
		s.Patterns = json.RawMessage(`[
			{"id":"who","kind":"insight","match":["\\bwho is {person}"],"tools":["antimatter.lookup_user"]},
			{"id":"say","kind":"action","match":["\\bsay hello\\b"],"tools":["antimatter.post_message"],"arguments":{"message":"hello"}}
		]`)
	})
	channel := th.CreatePublicChannel(t)
	th.AddUserToChannel(t, th.BasicUser2, channel)
	client2 := th.CreateClient()
	th.LoginBasic2WithClient(t, client2)
	return th, clients, client2, channel
}

func waitForAssistantCards(t *testing.T, client *model.Client4, channelID string, count int) []*model.AssistantCard {
	t.Helper()
	var cards []*model.AssistantCard
	require.Eventually(t, func() bool {
		feed, _, err := client.GetAssistantFeed(context.Background(), channelID)
		if err != nil {
			return false
		}
		cards = feed.Cards
		return len(cards) >= count
	}, 10*time.Second, 50*time.Millisecond)
	return cards
}

// collectAssistantEvents gathers the assistant's events a websocket client gets.
func collectAssistantEvents(t *testing.T, ws *model.WebSocketClient) func() []*model.WebSocketEvent {
	var mu sync.Mutex
	var events []*model.WebSocketEvent
	go func() {
		for event := range ws.EventChannel {
			if strings.HasPrefix(string(event.EventType()), "assistant_") {
				mu.Lock()
				events = append(events, event)
				mu.Unlock()
			}
		}
	}()
	return func() []*model.WebSocketEvent {
		mu.Lock()
		defer mu.Unlock()
		return append([]*model.WebSocketEvent(nil), events...)
	}
}

func eventCardOwner(event *model.WebSocketEvent) string {
	card, _ := event.GetData()["card"].(map[string]any)
	owner, _ := card["user_id"].(string)
	return owner
}

func TestAssistantAPI(t *testing.T) {
	th, clients, client2, channel := setupAssistantAPI(t)
	ctx := context.Background()
	client := th.Client

	ws1 := th.CreateConnectedWebSocketClientWithClient(t, client)
	ws2 := th.CreateConnectedWebSocketClientWithClient(t, client2)
	events1, events2 := collectAssistantEvents(t, ws1), collectAssistantEvents(t, ws2)

	for _, c := range []*model.Client4{client, client2} {
		presence, _, err := c.SetAssistantPresence(ctx, channel.Id, true)
		require.NoError(t, err)
		assert.True(t, presence.Following)
		assert.Equal(t, model.AssistantPresenceTTLSeconds, presence.TTLSeconds)
	}

	_, _, err := client.CreatePost(ctx, &model.Post{ChannelId: channel.Id, Message: "who is @" + th.BasicUser2.Username + "?"})
	require.NoError(t, err)
	mine := waitForAssistantCards(t, client, channel.Id, 1)
	theirs := waitForAssistantCards(t, client2, channel.Id, 1)
	require.Len(t, mine, 1)
	assert.Equal(t, model.AssistantCardKindInsight, mine[0].Kind)
	assert.Equal(t, "antimatter.lookup_user", mine[0].Tool)
	assert.Equal(t, th.BasicUser.Id, mine[0].UserId)
	assert.Equal(t, th.BasicUser2.Id, theirs[0].UserId)

	t.Run("websocket: each card to its owner only", func(t *testing.T) {
		require.Eventually(t, func() bool {
			for _, e := range events1() {
				if e.EventType() == model.WebsocketEventAssistantCardAdded && eventCardOwner(e) == th.BasicUser.Id {
					return true
				}
			}
			return false
		}, 5*time.Second, 50*time.Millisecond)
		time.Sleep(200 * time.Millisecond)
		for _, e := range events2() {
			assert.NotEqual(t, th.BasicUser.Id, eventCardOwner(e), "someone else's card never reaches this person")
		}
		for _, e := range events1() {
			if owner := eventCardOwner(e); owner != "" {
				assert.Equal(t, th.BasicUser.Id, owner)
			}
		}
	})

	t.Run("RBAC: only one's own feed, in channels one may read", func(t *testing.T) {
		other := th.CreateUser(t)
		th.LinkUserToTeam(t, other, th.BasicTeam)
		dm := th.CreateDmChannel(t, other)
		_, resp, err := client2.GetAssistantFeed(ctx, dm.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		_, resp, err = client2.SetAssistantPresence(ctx, dm.Id, true)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		resp, err = client2.SetAssistantChannelOn(ctx, dm.Id, false)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		for name, call := range map[string]func() (*model.Response, error){
			"dismiss": func() (*model.Response, error) {
				_, r, e := client2.DismissAssistantCard(ctx, channel.Id, mine[0].Id)
				return r, e
			},
			"pin": func() (*model.Response, error) {
				_, r, e := client2.PinAssistantCard(ctx, channel.Id, mine[0].Id, true)
				return r, e
			},
			"mute": func() (*model.Response, error) {
				_, r, e := client2.MuteAssistantCard(ctx, channel.Id, mine[0].Id)
				return r, e
			},
			"post": func() (*model.Response, error) {
				_, r, e := client2.PostAssistantCard(ctx, channel.Id, mine[0].Id)
				return r, e
			},
		} {
			got, callErr := call()
			require.Error(t, callErr, name)
			CheckNotFoundStatus(t, got)
		}

		anonymous := th.CreateClient()
		_, resp, err = anonymous.GetAssistantFeed(ctx, channel.Id)
		require.Error(t, err)
		CheckUnauthorizedStatus(t, resp)
	})

	t.Run("post an insight to the channel", func(t *testing.T) {
		post, resp, err := client.PostAssistantCard(ctx, channel.Id, mine[0].Id)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		assert.Equal(t, th.BasicUser.Id, post.UserId)
		assert.Equal(t, mine[0].Id, post.GetProp(model.PostPropAssistantCard))
	})

	t.Run("actions: the author's to run, once", func(t *testing.T) {
		_, _, err := client.CreatePost(ctx, &model.Post{ChannelId: channel.Id, Message: "say hello please"})
		require.NoError(t, err)
		var mineAction, theirAction *model.AssistantCard
		require.Eventually(t, func() bool {
			mineAction, theirAction = nil, nil
			for _, c := range waitForAssistantCards(t, client, channel.Id, 1) {
				if c.Kind == model.AssistantCardKindAction {
					mineAction = c
				}
			}
			for _, c := range waitForAssistantCards(t, client2, channel.Id, 1) {
				if c.Kind == model.AssistantCardKindAction {
					theirAction = c
				}
			}
			return mineAction != nil && theirAction != nil && mineAction.Offer.HolderId != ""
		}, 10*time.Second, 50*time.Millisecond)
		assert.Equal(t, th.BasicUser.Id, mineAction.Offer.HolderId)
		assert.True(t, mineAction.Offer.CanRun)
		assert.False(t, theirAction.Offer.CanRun)

		_, resp, err := client2.RunAssistantCard(ctx, channel.Id, theirAction.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		_, resp, err = client2.ClaimAssistantCard(ctx, channel.Id, theirAction.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		result, _, err := client.RunAssistantCard(ctx, channel.Id, mineAction.Id)
		require.NoError(t, err)
		assert.False(t, result.Executed)
		assert.Equal(t, model.AssistantOfferStatusDone, result.Card.Offer.Status)
		_, resp, err = client.RunAssistantCard(ctx, channel.Id, mineAction.Id)
		require.Error(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusConflict, resp.StatusCode, "a second run")

		_, resp, err = client.PostAssistantCard(ctx, channel.Id, mineAction.Id)
		require.Error(t, err, "an action has nothing to post")
		CheckBadRequestStatus(t, resp)
	})

	t.Run("muted tools are one's own", func(t *testing.T) {
		muted, _, err := client.SetAssistantMutedTools(ctx, &model.AssistantMutedTools{Tools: []string{"antimatter.lookup_user"}})
		require.NoError(t, err)
		assert.Equal(t, []string{"antimatter.lookup_user"}, muted.Tools)
		theirs, _, err := client2.GetAssistantMutedTools(ctx)
		require.NoError(t, err)
		assert.Empty(t, theirs.Tools)
		_, resp, err := client.SetAssistantMutedTools(ctx, &model.AssistantMutedTools{Tools: []string{"has space"}})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("the person's switch", func(t *testing.T) {
		_, err := client2.SetAssistantChannelOn(ctx, channel.Id, false)
		require.NoError(t, err)
		feed, _, err := client2.GetAssistantFeed(ctx, channel.Id)
		require.NoError(t, err)
		assert.False(t, feed.On)
		assert.Empty(t, feed.Cards, "turning it off forgets the feed")
		presence, _, err := client2.SetAssistantPresence(ctx, channel.Id, true)
		require.NoError(t, err)
		assert.False(t, presence.Following)
	})

	t.Run("RBAC: a community's settings are its admins'", func(t *testing.T) {
		_, resp, err := clients.member.GetAssistantCommunitySettings(ctx, th.BasicTeam.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		_, resp, err = clients.otherAdmin.SetAssistantCommunitySettings(ctx, th.BasicTeam.Id, &model.AssistantCommunitySettings{Engine: model.AssistantEngineRules})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		info, _, err := clients.communityAdmin.GetAssistantCommunitySettings(ctx, th.BasicTeam.Id)
		require.NoError(t, err)
		assert.True(t, info.ServerEnabled)
		assert.False(t, info.MayOverride)
		_, resp, err = clients.communityAdmin.SetAssistantCommunitySettings(ctx, th.BasicTeam.Id, &model.AssistantCommunitySettings{Enabled: model.NewPointer(false)})
		require.Error(t, err, "not unless the server lets communities")
		CheckForbiddenStatus(t, resp)

		th.App.UpdateConfig(func(cfg *model.Config) { cfg.AISettings.Assistant.AllowCommunityOverride = model.NewPointer(true) })
		info, _, err = clients.communityAdmin.SetAssistantCommunitySettings(ctx, th.BasicTeam.Id, &model.AssistantCommunitySettings{Enabled: model.NewPointer(false)})
		require.NoError(t, err)
		require.NotNil(t, info.Settings.Enabled)
		assert.False(t, *info.Settings.Enabled)
		feed, _, err := client.GetAssistantFeed(ctx, channel.Id)
		require.NoError(t, err)
		assert.False(t, feed.Enabled, "the community turned it off")

		_, resp, err = clients.communityAdmin.SetAssistantCommunitySettings(ctx, th.BasicTeam.Id, &model.AssistantCommunitySettings{ConnectionId: model.NewId()})
		require.Error(t, err, "a decision model the community may not use")
		CheckBadRequestStatus(t, resp)
		_, _, err = clients.systemAdmin.SetAssistantCommunitySettings(ctx, th.BasicTeam.Id, &model.AssistantCommunitySettings{})
		require.NoError(t, err)
	})
}
