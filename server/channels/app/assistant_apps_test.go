// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// assistantTestApp is an app that does nothing but run: what's under test is what the server does with its bot's
// posts.
const assistantTestApp = `
package main

import (
	"github.com/mattermost/mattermost/server/public/plugin"
)

type MyPlugin struct {
	plugin.MattermostPlugin
}

func main() {
	plugin.ClientMain(&MyPlugin{})
}
`

const assistantTestAlertmanager = "com.antimatterchat.alertmanager"

// alertAttachment is an alert as the Alertmanager app posts it: no text, the alert in an attachment (title, a
// summary naming the host, fields).
func alertAttachment(alert, summary, host string) *model.MessageAttachment {
	return &model.MessageAttachment{
		Fallback: "Firing: " + alert,
		Color:    "#d24b4e",
		Title:    "Firing: " + alert,
		Text:     "`" + host + "`\n" + summary + "\n_Started 10:02 · [Source](https://prometheus.example.com/graph)_",
		Footer:   "Alertmanager · prod",
		Fields: []*model.MessageAttachmentField{
			{Title: "Severity", Value: "warning", Short: true},
			{Title: "Instance", Value: host, Short: true},
		},
	}
}

// appPost saves a bot's post as the apps' API saves it (from_bot, from_plugin), without the hooks; props adds or (nil)
// removes props.
func (at *assistantTest) appPost(bot *model.User, channel *model.Channel, attachment *model.MessageAttachment, props map[string]any) *model.Post {
	at.t.Helper()
	at.mu.Lock()
	at.clock += 10
	createAt := at.clock
	at.mu.Unlock()
	post := &model.Post{UserId: bot.Id, ChannelId: channel.Id, CreateAt: createAt}
	if attachment != nil {
		model.ParseMessageAttachment(post, []*model.MessageAttachment{attachment})
	}
	post.AddProp(model.PostPropsFromBot, "true")
	post.AddProp(model.PostPropsFromPlugin, "true")
	for key, value := range props {
		if value == nil {
			post.DelProp(key)
		} else {
			post.AddProp(key, value)
		}
	}
	saved, err := at.th.App.Srv().Store().Post().Save(at.th.Context, post)
	require.NoError(at.t, err)
	return saved
}

// postWithProps saves a person's message with props, without the hooks.
func (at *assistantTest) postWithProps(u *model.User, channel *model.Channel, message string, props map[string]any) *model.Post {
	at.t.Helper()
	at.mu.Lock()
	at.clock += 10
	createAt := at.clock
	at.mu.Unlock()
	post := &model.Post{UserId: u.Id, ChannelId: channel.Id, Message: message, CreateAt: createAt}
	for key, value := range props {
		post.AddProp(key, value)
	}
	saved, err := at.th.App.Srv().Store().Post().Save(at.th.Context, post)
	require.NoError(at.t, err)
	return saved
}

// newChannel is a community channel with maya, sam and olivia.
func (at *assistantTest) newChannel() *model.Channel {
	channel := at.th.CreateChannel(at.t, at.th.BasicTeam)
	at.th.AddUserToChannel(at.t, at.sam, channel)
	at.th.AddUserToChannel(at.t, at.olivia, channel)
	return channel
}

// queued gives the messages waiting in a channel's window.
func (at *assistantTest) queued(channel *model.Channel) []string {
	st := at.th.App.assistant()
	st.mu.Lock()
	defer st.mu.Unlock()
	if w, ok := st.windows[channel.Id]; ok {
		return slices.Clone(w.posts)
	}
	return nil
}

// dropWindow forgets a channel's window without running it.
func (at *assistantTest) dropWindow(channel *model.Channel) {
	st := at.th.App.assistant()
	st.mu.Lock()
	defer st.mu.Unlock()
	if w, ok := st.windows[channel.Id]; ok {
		w.timer.Stop()
		delete(st.windows, channel.Id)
	}
}

// A trusted app's post (a bot the app owns, while the app runs) starts the assistant like a person's message: the
// engines read its attachment, so an alert about www-1 gives each viewer their own card for www-1. Nothing else that
// looks like it does: an app not in the list, the app turned off, a webhook, a person's message with the props, a
// bot of a person's, an override of the name.
func TestAssistantTrustedApps(t *testing.T) {
	at := setupAssistantTest(t)
	th := at.th
	th.App.UpdateConfig(func(cfg *model.Config) {
		cfg.AssistantSettings.CardsPerUserHour = model.NewPointer(1000)
		cfg.ServiceSettings.EnableIncomingWebhooks = model.NewPointer(true)
		cfg.ServiceSettings.EnablePostUsernameOverride = model.NewPointer(true)
	})
	require.Equal(t, []string{assistantTestAlertmanager}, th.App.Config().AssistantSettings.TriggeringApps, "the Alertmanager app by default")
	setupPluginAPITest(t, assistantTestApp, `{"id": "`+assistantTestAlertmanager+`", "server": {"executable": "backend.exe"}}`, assistantTestAlertmanager, th.App, th.Context)
	require.True(t, th.App.GetPluginsEnvironment().IsActive(assistantTestAlertmanager))
	bot, appErr := th.App.CreateBot(th.Context, &model.Bot{Username: "alertmanager", DisplayName: "Alertmanager", OwnerId: assistantTestAlertmanager})
	require.Nil(t, appErr)
	alertBot, appErr := th.App.GetUser(th.Context, bot.UserId)
	require.Nil(t, appErr)
	highCPU := alertAttachment("HighCPU", "CPU load on www-1 is above 90%", "www-1")

	t.Run("an alert's attachment gives each viewer their own card for its host", func(t *testing.T) {
		channel := at.newChannel()
		at.open(at.maya, channel)
		at.open(at.sam, channel)
		before := at.postCount(channel)
		post := at.appPost(alertBot, channel, highCPU, nil)
		require.Empty(t, post.Message, "the alert is in the attachment only")
		at.run(channel, post)

		maya, sam := at.feed(at.maya, channel), at.feed(at.sam, channel)
		require.Len(t, maya, 1)
		require.Len(t, sam, 1)
		assert.Equal(t, model.AssistantCardKindInsight, maya[0].Kind)
		assert.Equal(t, at.tool("host_metrics"), maya[0].Tool)
		assert.JSONEq(t, `{"host":"www-1","metric":"cpu"}`, string(maya[0].Arguments), "the host the attachment names")
		assert.Equal(t, `host_metrics {"host":"www-1","metric":"cpu"} as `+at.maya.Username, maya[0].Result, "run as maya, with her sign-in")
		assert.Equal(t, `host_metrics {"host":"www-1","metric":"cpu"} as `+at.sam.Username, sam[0].Result, "run as sam, with his own")
		assert.Equal(t, []string{post.Id}, maya[0].PostIds)

		t.Run("the why names the app", func(t *testing.T) {
			assert.Equal(t, assistantTestAlertmanager, maya[0].Why.App)
			assert.Equal(t, assistantTestAlertmanager, sam[0].Why.App)
			assert.Equal(t, model.AssistantEngineRules, maya[0].Why.Engine)
			assert.Len(t, maya[0].Why.Words, 1)
			assert.Contains(t, strings.ToLower(maya[0].Why.Words[0]), "www-1")
		})

		t.Run("privacy: per viewer, as for a person's message", func(t *testing.T) {
			assert.Empty(t, at.feed(at.olivia, channel), "olivia isn't viewing: nothing for her")
			assert.Empty(t, th.App.assistantFeed(alertBot.Id, channel.Id).Cards, "the app's bot gets nothing")
			for _, e := range at.sentEvents() {
				assert.Empty(t, e.GetBroadcast().ChannelId, "no event goes to a channel")
				assert.Empty(t, e.GetBroadcast().TeamId, "no event goes to a community")
				if card, ok := e.GetData()["card"].(*model.AssistantCard); ok {
					assert.Equal(t, card.UserId, e.GetBroadcast().UserId, "a card goes to its owner's clients only")
				}
			}
			assert.Equal(t, before+1, at.postCount(channel), "only the alert: nothing posted")
			_, appErr := th.App.GetAssistantFeed(th.Context, alertBot.Id, channel.Id)
			require.NotNil(t, appErr, "a bot has no feed")
		})

		t.Run("a private channel's alert: only its members viewing", func(t *testing.T) {
			private := th.CreatePrivateChannel(t, th.BasicTeam)
			th.AddUserToChannel(t, at.sam, private)
			at.open(at.maya, private)
			at.open(at.sam, private)
			require.Nil(t, th.RemoveUserFromChannel(t, at.sam, private))
			at.run(private, at.appPost(alertBot, private, highCPU, nil))
			assert.Len(t, at.feed(at.maya, private), 1)
			assert.Empty(t, th.App.assistantFeed(at.sam.Id, private.Id).Cards, "sam left the channel: no card, though his panel was open")
		})
	})

	t.Run("a bot of an app not in the list doesn't start it", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.AssistantSettings.TriggeringApps = []string{"com.example.uptime"} })
		defer th.App.UpdateConfig(func(cfg *model.Config) { cfg.AssistantSettings.TriggeringApps = []string{assistantTestAlertmanager} })
		channel := at.newChannel()
		at.open(at.maya, channel)
		at.run(channel, at.appPost(alertBot, channel, highCPU, nil))
		assert.Empty(t, at.feed(at.maya, channel), "the Alertmanager app runs, but isn't in the list")

		th.App.UpdateConfig(func(cfg *model.Config) { cfg.AssistantSettings.TriggeringApps = []string{} })
		at.run(channel, at.appPost(alertBot, channel, highCPU, nil))
		assert.Empty(t, at.feed(at.maya, channel), "no apps at all")
	})

	t.Run("other bots don't start it", func(t *testing.T) {
		channel := at.newChannel()
		at.open(at.maya, channel)
		personal, appErr := th.App.CreateBot(th.Context, &model.Bot{Username: "mayas-bot", OwnerId: at.maya.Id})
		require.Nil(t, appErr)
		personalBot, appErr := th.App.GetUser(th.Context, personal.UserId)
		require.Nil(t, appErr)
		at.run(channel, at.appPost(personalBot, channel, highCPU, nil))
		assert.Empty(t, at.feed(at.maya, channel), "a person's bot, even posting through an app")

		deactivated, appErr := th.App.CreateBot(th.Context, &model.Bot{Username: "old-alerts", OwnerId: assistantTestAlertmanager})
		require.Nil(t, appErr)
		oldBot, appErr := th.App.GetUser(th.Context, deactivated.UserId)
		require.Nil(t, appErr)
		post := at.appPost(oldBot, channel, highCPU, nil)
		_, appErr = th.App.UpdateBotActive(th.Context, deactivated.UserId, false)
		require.Nil(t, appErr)
		at.run(channel, post)
		assert.Empty(t, at.feed(at.maya, channel), "the app's deactivated bot")
	})

	t.Run("the app's bot with an override of its name or icon doesn't start it", func(t *testing.T) {
		channel := at.newChannel()
		at.open(at.maya, channel)
		for _, prop := range []string{model.PostPropsOverrideUsername, model.PostPropsOverrideIconURL, model.PostPropsOverrideIconEmoji, model.PostPropsWebhookDisplayName} {
			at.run(channel, at.appPost(alertBot, channel, highCPU, map[string]any{prop: "maya"}))
			assert.Empty(t, at.feed(at.maya, channel), prop)
		}
		at.run(channel, at.appPost(alertBot, channel, highCPU, map[string]any{model.PostPropsFromPlugin: nil}))
		assert.Empty(t, at.feed(at.maya, channel), "the bot's post not through the apps' API (its token, the REST API)")
		at.run(channel, at.appPost(alertBot, channel, highCPU, map[string]any{model.PostPropsFromOAuthApp: "true"}))
		assert.Empty(t, at.feed(at.maya, channel), "through an OAuth app")
	})

	t.Run("an incoming webhook's post with the same attachment doesn't start it", func(t *testing.T) {
		channel := at.newChannel()
		at.open(at.maya, channel)
		th.LinkUserToTeam(t, alertBot, th.BasicTeam)
		th.AddUserToChannel(t, alertBot, channel)
		for _, creator := range []*model.User{at.maya, alertBot} {
			hook, appErr := th.App.CreateIncomingWebhookForChannel(creator.Id, channel, &model.IncomingWebhook{ChannelId: channel.Id})
			require.Nil(t, appErr)
			before := at.postCount(channel)
			require.Nil(t, th.App.HandleIncomingWebhook(th.Context, hook.Id, &model.IncomingWebhookRequest{
				Username:    "Alertmanager",
				Attachments: []*model.MessageAttachment{alertAttachment("HighCPU", "CPU load on www-1 is above 90%", "www-1")},
				Props:       model.StringInterface{model.PostPropsFromPlugin: "true", model.PostPropsFromBot: "true"},
			}))
			require.Equal(t, before+1, at.postCount(channel))
			assert.Empty(t, at.queued(channel), "the webhook's post doesn't join the window (made by %s)", creator.Username)
			list, appErr := th.App.GetPostsPage(th.Context, model.GetPostsOptions{ChannelId: channel.Id, PerPage: 1})
			require.Nil(t, appErr)
			at.run(channel, list.Posts[list.Order[0]])
			assert.Empty(t, at.feed(at.maya, channel), "nor gives a card when its window runs (made by %s)", creator.Username)
		}
	})

	t.Run("a person's message with an app's props is a person's, as before", func(t *testing.T) {
		channel := at.newChannel()
		at.open(at.maya, channel)
		at.run(channel, at.postWithProps(at.sam, channel, "cpu on www-1?", map[string]any{model.PostPropsFromPlugin: "true"}))
		assert.Empty(t, at.feed(at.maya, channel), "a person's message from an app doesn't start it, as before")
		at.run(channel, at.postWithProps(at.sam, channel, "cpu on www-1?", map[string]any{model.PostPropsFromPlugin: "true", model.PostPropsFromBot: "true"}))
		assert.Empty(t, at.feed(at.maya, channel), "nor with from_bot forged: sam isn't a bot")

		post := at.postWithProps(at.sam, channel, "cpu on www-1?", map[string]any{model.PostPropsOverrideUsername: "alertmanager", model.PostPropsOverrideIconURL: "https://example.com/am.png"})
		at.run(channel, post)
		cards := at.feed(at.maya, channel)
		require.Len(t, cards, 1, "an override on a person's message: still sam's message")
		assert.Empty(t, cards[0].Why.App, "not an app's")
		require.Len(t, at.feed(at.sam, channel), 0, "sam isn't viewing")
	})

	t.Run("an action from an app's post goes to whoever opened first", func(t *testing.T) {
		channel := at.newChannel()
		at.open(at.olivia, channel)
		at.open(at.sam, channel)
		at.open(at.maya, channel)
		post := at.appPost(alertBot, channel, &model.MessageAttachment{
			Title: "Canary healthy: web-app 2.4",
			Text:  "Ready to roll out web-app to production",
		}, nil)
		at.run(channel, post)
		for _, u := range []*model.User{at.maya, at.sam, at.olivia} {
			card := at.offerCard(u, channel)
			assert.Equal(t, at.tool("deploy"), card.Tool)
			assert.Equal(t, assistantTestAlertmanager, card.Why.App)
			assert.Equal(t, at.olivia.Id, card.Offer.HolderId, "%s sees it offered to olivia, who opened first", u.Username)
		}
		offer := th.App.assistantOffer(at.offerCard(at.maya, channel).OfferId)
		require.NotNil(t, offer)
		assert.Empty(t, offer.AuthorId, "no author to offer it to first")
		_, appErr := th.App.RunAssistantCard(th.Context, at.maya.Id, channel.Id, at.offerCard(at.maya, channel).Id)
		require.NotNil(t, appErr, "only the holder runs it")
		result, appErr := th.App.RunAssistantCard(th.Context, at.olivia.Id, channel.Id, at.offerCard(at.olivia, channel).Id)
		require.Nil(t, appErr)
		assert.Equal(t, model.AssistantOfferStatusDone, result.Card.Offer.Status)
	})

	t.Run("windows with apps' posts are capped per channel and hour", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.AssistantSettings.AppWindowsPerChannelHour = model.NewPointer(2) })
		defer th.App.UpdateConfig(func(cfg *model.Config) { cfg.AssistantSettings.AppWindowsPerChannelHour = model.NewPointer(30) })
		channel := at.newChannel()
		at.open(at.maya, channel)
		for _, alert := range [][2]string{{"HighCPU", "CPU load on www-1 is above 90%"}, {"DiskFull", "Disk use on www-1 is above 95%"}, {"HighMemory", "Memory use on www-1 is above 90%"}} {
			at.run(channel, at.appPost(alertBot, channel, alertAttachment(alert[0], alert[1], "www-1"), nil))
		}
		assert.Len(t, at.feed(at.maya, channel), 2, "the third alert's window is past the cap")

		at.run(channel, at.appPost(alertBot, channel, alertAttachment("HighLatency", "Latency on www-1 is above 2s", "www-1"), nil), at.post(at.sam, channel, "traffic on www-1?"))
		cards := at.feed(at.maya, channel)
		require.Len(t, cards, 3, "past the cap, a person's message in the window still counts, the app's post doesn't")
		assert.JSONEq(t, `{"host":"www-1","metric":"traffic"}`, string(cards[0].Arguments))
		assert.Empty(t, cards[0].Why.App)

		other := at.newChannel()
		at.open(at.maya, other)
		at.run(other, at.appPost(alertBot, other, highCPU, nil))
		assert.Len(t, at.feed(at.maya, other), 1, "the cap is the channel's")
	})

	t.Run("only a new post starts it: the app updating its post doesn't", func(t *testing.T) {
		channel := at.newChannel()
		at.open(at.maya, channel)
		api := th.App.NewPluginAPI(th.Context, &model.Manifest{Id: assistantTestAlertmanager})
		post := &model.Post{UserId: alertBot.Id, ChannelId: channel.Id}
		model.ParseMessageAttachment(post, []*model.MessageAttachment{alertAttachment("HighCPU", "CPU load on www-1 is above 90%", "www-1")})
		created, appErr := api.CreatePost(post)
		require.Nil(t, appErr)
		assert.Equal(t, "true", created.GetProp(model.PostPropsFromPlugin))
		assert.Equal(t, "true", created.GetProp(model.PostPropsFromBot))
		assert.Equal(t, []string{created.Id}, at.queued(channel), "the new alert joins the window")
		at.dropWindow(channel)

		resolved := created.Clone()
		model.ParseMessageAttachment(resolved, []*model.MessageAttachment{{Title: "Resolved: HighCPU", Text: "`www-1` · resolved\nCPU load on www-1 is above 90%"}})
		_, appErr = api.UpdatePost(resolved)
		require.Nil(t, appErr)
		assert.Empty(t, at.queued(channel), "the edit doesn't")
	})

	// Last: it turns the app off.
	t.Run("the app turned off doesn't start it", func(t *testing.T) {
		channel := at.newChannel()
		at.open(at.maya, channel)
		require.Nil(t, th.App.DisablePlugin(assistantTestAlertmanager))
		require.False(t, th.App.GetPluginsEnvironment().IsActive(assistantTestAlertmanager))
		at.run(channel, at.appPost(alertBot, channel, highCPU, nil))
		assert.Empty(t, at.feed(at.maya, channel), "its bot's post, while the app is off")
	})
}

// The shape of an app's post: through the apps' API, by a bot, plain or with attachments, nothing that marks a
// webhook, a command, an OAuth app, a system message, another server or an override.
func TestAssistantAppPostShape(t *testing.T) {
	shaped := func(change func(p *model.Post)) *model.Post {
		p := &model.Post{Id: model.NewId(), UserId: model.NewId(), ChannelId: model.NewId()}
		model.ParseMessageAttachment(p, []*model.MessageAttachment{{Title: "Firing: HighCPU"}})
		p.AddProp(model.PostPropsFromBot, "true")
		p.AddProp(model.PostPropsFromPlugin, "true")
		if change != nil {
			change(p)
		}
		return p
	}
	assert.True(t, assistantAppPostShape(shaped(nil)))
	assert.True(t, assistantAppPostShape(shaped(func(p *model.Post) { p.Type = model.PostTypeDefault })), "a plain post")
	for name, change := range map[string]func(p *model.Post){
		"no from_plugin":   func(p *model.Post) { p.DelProp(model.PostPropsFromPlugin) },
		"no from_bot":      func(p *model.Post) { p.DelProp(model.PostPropsFromBot) },
		"a webhook's":      func(p *model.Post) { p.AddProp(model.PostPropsFromWebhook, "true") },
		"an OAuth app's":   func(p *model.Post) { p.AddProp(model.PostPropsFromOAuthApp, "true") },
		"a system message": func(p *model.Post) { p.Type = model.PostTypeJoinChannel },
		"another type":     func(p *model.Post) { p.Type = "custom_poll" },
		"another server's": func(p *model.Post) { p.RemoteId = model.NewPointer(model.NewId()) },
		"deleted":          func(p *model.Post) { p.DeleteAt = 1 },
		"from a card":      func(p *model.Post) { p.AddProp(model.PostPropAssistantCard, model.NewId()) },
		"an agent's":       func(p *model.Post) { p.AddProp(PostPropAgentStreaming, "true") },
		"a username":       func(p *model.Post) { p.AddProp(model.PostPropsOverrideUsername, "maya") },
		"an icon":          func(p *model.Post) { p.AddProp(model.PostPropsOverrideIconURL, "https://example.com/i.png") },
		"an emoji":         func(p *model.Post) { p.AddProp(model.PostPropsOverrideIconEmoji, "fire") },
		"a display name":   func(p *model.Post) { p.AddProp(model.PostPropsWebhookDisplayName, "Alerts") },
		"from_bot false":   func(p *model.Post) { p.AddProp(model.PostPropsFromBot, false) },
	} {
		assert.False(t, assistantAppPostShape(shaped(change)), name)
	}
	assert.False(t, assistantAppPostShape(nil))
}

// What the engines read of an app's post: its text, the attachments' pretext, title, text and fields, a line each,
// capped.
func TestAssistantAppPostText(t *testing.T) {
	post := &model.Post{Message: "Alerts for prod"}
	model.ParseMessageAttachment(post, []*model.MessageAttachment{
		{Pretext: "From Alertmanager", Title: "Firing: InstanceDown", Text: "`www-1`\nwww-1 is down", Footer: "not read", Fields: []*model.MessageAttachmentField{
			{Title: "Severity", Value: "critical"}, {Title: "Count", Value: 3}, {Title: "Empty", Value: nil}, {Title: "", Value: "untitled"}, nil,
		}},
		{Title: "Second"},
	})
	assert.Equal(t, "Alerts for prod\nFrom Alertmanager\nFiring: InstanceDown\n`www-1`\nwww-1 is down\nSeverity: critical\nCount: 3\nuntitled\nSecond", assistantAppPostText(post))

	long := &model.Post{}
	model.ParseMessageAttachment(long, []*model.MessageAttachment{{Text: strings.Repeat("é", model.AssistantAppTextMaxRunes+100)}})
	assert.Equal(t, model.AssistantAppTextMaxRunes, len([]rune(assistantAppPostText(long))), "capped, in characters")
}
