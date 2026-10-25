// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app/aimcp"
)

// assistantTest is a community channel with three people (maya, sam, olivia), each signed in with their own token to
// an MCP server whose tools say whose token ran them: host_metrics reads, deploy changes something.
type assistantTest struct {
	t       *testing.T
	th      *TestHelper
	mcp     *fakeMCPServer
	server  *model.AIMCPServer
	channel *model.Channel
	maya    *model.User
	sam     *model.User
	olivia  *model.User

	mu     sync.Mutex
	events []*model.WebSocketEvent
	clock  int64
}

func setupAssistantTest(t *testing.T) *assistantTest {
	t.Helper()
	th := setupAITest(t)
	th.App.UpdateConfig(func(cfg *model.Config) {
		s := &cfg.AssistantSettings
		s.Enable = model.NewPointer(true)
		s.Engine = model.NewPointer(model.AssistantEngineRules)
		// The tests run the windows themselves.
		s.DebounceSeconds = model.NewPointer(60)
		s.SeedEntities = json.RawMessage(`[{"type":"host","value":"www-1"},{"type":"service","value":"web-app"}]`)
	})
	at := &assistantTest{t: t, th: th, maya: th.BasicUser, sam: th.BasicUser2, clock: model.GetMillis()}
	f := newFakeMCPServer(t)
	yes, no := true, false
	f.tools = []aimcp.Tool{
		{Name: "host_metrics", Title: "Host metrics", Description: "CPU and memory of a host", InputSchema: json.RawMessage(`{"type":"object","properties":{"host":{"type":"string"},"metric":{"type":"string"}},"required":["host"]}`), Annotations: &aimcp.ToolAnnotations{ReadOnlyHint: &yes}},
		{Name: "deploy", Title: "Deploy", Description: "Deploy a service", InputSchema: json.RawMessage(`{"type":"object","properties":{"service":{"type":"string"}},"required":["service"]}`), Annotations: &aimcp.ToolAnnotations{ReadOnlyHint: &no}},
	}
	f.answer = func(name string, arguments json.RawMessage, who string) string {
		return name + " " + string(arguments) + " as " + who
	}
	at.mcp = f
	at.server = createTestMCPServer(t, th, f, nil)
	at.olivia = th.CreateUser(t)
	th.LinkUserToTeam(t, at.olivia, th.BasicTeam)
	at.channel = th.CreateChannel(t, th.BasicTeam)
	th.AddUserToChannel(t, at.sam, at.channel)
	th.AddUserToChannel(t, at.olivia, at.channel)
	for _, u := range []*model.User{at.maya, at.sam, at.olivia} {
		at.signIn(u)
	}
	st := th.App.assistant()
	st.mu.Lock()
	st.publishHook = func(event *model.WebSocketEvent) {
		at.mu.Lock()
		defer at.mu.Unlock()
		at.events = append(at.events, event)
	}
	st.mu.Unlock()
	return at
}

func (at *assistantTest) signIn(u *model.User) {
	token := "token-of-" + u.Username
	at.mcp.takeToken(token, u.Username)
	signInToTestMCPServer(at.t, at.th, at.server, u.Id, token)
}

func (at *assistantTest) tool(name string) string {
	return at.server.Namespace() + "." + name
}

// open opens the assistant on a channel for someone; each opens after the one before.
func (at *assistantTest) open(u *model.User, channel *model.Channel) {
	at.t.Helper()
	presence, appErr := at.th.App.SetAssistantPresence(at.th.Context, u.Id, channel.Id, true)
	require.Nil(at.t, appErr)
	require.True(at.t, presence.Following)
	time.Sleep(3 * time.Millisecond)
}

// post saves a message without the hooks (the tests run the windows themselves), each after the one before.
func (at *assistantTest) post(u *model.User, channel *model.Channel, message string) *model.Post {
	at.t.Helper()
	at.mu.Lock()
	at.clock += 10
	createAt := at.clock
	at.mu.Unlock()
	post, err := at.th.App.Srv().Store().Post().Save(at.th.Context, &model.Post{UserId: u.Id, ChannelId: channel.Id, Message: message, CreateAt: createAt})
	require.NoError(at.t, err)
	return post
}

func (at *assistantTest) run(channel *model.Channel, posts ...*model.Post) {
	ids := make([]string, 0, len(posts))
	for _, p := range posts {
		ids = append(ids, p.Id)
	}
	at.th.App.runAssistantWindow(at.th.Context, channel.Id, ids)
}

func (at *assistantTest) feed(u *model.User, channel *model.Channel) []*model.AssistantCard {
	at.t.Helper()
	feed, appErr := at.th.App.GetAssistantFeed(at.th.Context, u.Id, channel.Id)
	require.Nil(at.t, appErr)
	return feed.Cards
}

func (at *assistantTest) sentEvents() []*model.WebSocketEvent {
	at.mu.Lock()
	defer at.mu.Unlock()
	return append([]*model.WebSocketEvent(nil), at.events...)
}

func (at *assistantTest) postCount(channel *model.Channel) int {
	list, appErr := at.th.App.GetPostsPage(at.th.Context, model.GetPostsOptions{ChannelId: channel.Id, PerPage: 200})
	require.Nil(at.t, appErr)
	return len(list.Order)
}

// Insights run as each person viewing, with their own sign-in and permissions, and land in their own feed only: the
// same card for each, never one person's result in another's feed, never as the author.
func TestAssistantInsightsRunAsEachViewer(t *testing.T) {
	at := setupAssistantTest(t)
	nobody := at.th.CreateUser(t)
	at.th.LinkUserToTeam(t, nobody, at.th.BasicTeam)
	at.th.AddUserToChannel(t, nobody, at.channel)
	at.open(at.maya, at.channel)
	at.open(at.sam, at.channel)
	at.open(nobody, at.channel)
	before := at.postCount(at.channel)

	post := at.post(at.maya, at.channel, "have you seen the cpu on www-1?")
	at.run(at.channel, post)

	maya, sam := at.feed(at.maya, at.channel), at.feed(at.sam, at.channel)
	require.Len(t, maya, 1)
	require.Len(t, sam, 1)
	assert.Equal(t, model.AssistantCardKindInsight, maya[0].Kind)
	assert.Equal(t, at.tool("host_metrics"), maya[0].Tool)
	assert.Equal(t, `host_metrics {"host":"www-1","metric":"cpu"} as `+at.maya.Username, maya[0].Result)
	assert.Equal(t, `host_metrics {"host":"www-1","metric":"cpu"} as `+at.sam.Username, sam[0].Result, "sam's card is built with sam's own sign-in, not the author's")
	assert.Equal(t, maya[0].DedupeKey, sam[0].DedupeKey)
	assert.NotEqual(t, maya[0].Id, sam[0].Id)
	assert.Equal(t, []string{"cpu on www-1"}, maya[0].Why.Words, "the card says which words matched")
	assert.Equal(t, []string{post.Id}, maya[0].PostIds)
	assert.Empty(t, at.feed(nobody, at.channel), "a tool the person can't use (no sign-in) is skipped for them")

	t.Run("the same card isn't fetched twice in ten minutes", func(t *testing.T) {
		at.run(at.channel, at.post(at.sam, at.channel, "CPU on WWW-1 again"))
		assert.Len(t, at.feed(at.maya, at.channel), 1)
		assert.Len(t, at.feed(at.sam, at.channel), 1)
	})

	t.Run("cards reach their owner only, and nothing is posted", func(t *testing.T) {
		added := 0
		for _, e := range at.sentEvents() {
			assert.Empty(t, e.GetBroadcast().ChannelId, "no event goes to a channel")
			assert.Empty(t, e.GetBroadcast().TeamId, "no event goes to a community")
			if e.EventType() != model.WebsocketEventAssistantCardAdded {
				continue
			}
			added++
			card, ok := e.GetData()["card"].(*model.AssistantCard)
			require.True(t, ok)
			assert.Equal(t, card.UserId, e.GetBroadcast().UserId, "a card goes to its owner's clients only")
		}
		assert.Equal(t, 2, added)
		assert.Equal(t, before+2, at.postCount(at.channel), "only the two messages: the feed never posts (nor notifies)")
	})

	t.Run("tool calls are audited as the viewer's, run by the assistant", func(t *testing.T) {
		for _, auth := range at.mcp.authsSeen() {
			assert.Contains(t, []string{"Bearer token-of-" + at.maya.Username, "Bearer token-of-" + at.sam.Username, ""}, auth, "only maya's and sam's own tokens")
		}
	})
}

// The assistant's namespace of the key-value store can't be an app's: no installed app reads people's cards.
func TestAssistantNamespaceIsNoApps(t *testing.T) {
	assert.False(t, model.IsValidPluginId(assistantKVNamespace))
}

// Nobody reads another person's feed, and a card is someone's own to dismiss, pin, mute or post.
func TestAssistantFeedsArePersonal(t *testing.T) {
	at := setupAssistantTest(t)
	at.open(at.maya, at.channel)
	at.run(at.channel, at.post(at.maya, at.channel, "cpu on www-1?"))
	card := at.feed(at.maya, at.channel)[0]

	assert.Empty(t, at.feed(at.sam, at.channel), "sam has no feed of maya's")
	for name, op := range map[string]func() *model.AppError{
		"dismiss": func() *model.AppError {
			_, appErr := at.th.App.DismissAssistantCard(at.th.Context, at.sam.Id, at.channel.Id, card.Id)
			return appErr
		},
		"pin": func() *model.AppError {
			_, appErr := at.th.App.PinAssistantCard(at.th.Context, at.sam.Id, at.channel.Id, card.Id, true)
			return appErr
		},
		"mute": func() *model.AppError {
			_, appErr := at.th.App.MuteAssistantCardTool(at.th.Context, at.sam.Id, at.channel.Id, card.Id)
			return appErr
		},
		"post": func() *model.AppError {
			_, appErr := at.th.App.PostAssistantCard(at.th.Context, at.sam.Id, at.channel.Id, card.Id)
			return appErr
		},
	} {
		appErr := op()
		require.NotNil(t, appErr, name)
		assert.Equal(t, http.StatusNotFound, appErr.StatusCode, "%s: someone else's card isn't found", name)
	}

	t.Run("pin, post to the channel, dismiss", func(t *testing.T) {
		pinned, appErr := at.th.App.PinAssistantCard(at.th.Context, at.maya.Id, at.channel.Id, card.Id, true)
		require.Nil(t, appErr)
		assert.True(t, pinned.Pinned)
		post, appErr := at.th.App.PostAssistantCard(at.th.Context, at.maya.Id, at.channel.Id, card.Id)
		require.Nil(t, appErr)
		assert.Equal(t, at.maya.Id, post.UserId, "posted as the person")
		assert.Contains(t, post.Message, "as "+at.maya.Username)
		assert.Equal(t, card.Id, post.GetProp(model.PostPropAssistantCard))
		assert.False(t, assistantPostCounts(post), "a message posted from a card doesn't start the assistant again")
		assert.Equal(t, model.AssistantCardStatusPosted, at.feed(at.maya, at.channel)[0].Status)

		_, appErr = at.th.App.DismissAssistantCard(at.th.Context, at.maya.Id, at.channel.Id, card.Id)
		require.Nil(t, appErr)
		assert.Empty(t, at.feed(at.maya, at.channel))
	})

	t.Run("posting needs the permission to post", func(t *testing.T) {
		at.run(at.channel, at.post(at.maya, at.channel, "memory on www-1?"))
		card := at.feed(at.maya, at.channel)[0]
		at.th.RemovePermissionFromRole(t, model.PermissionCreatePost.Id, model.ChannelUserRoleId)
		defer at.th.AddPermissionToRole(t, model.PermissionCreatePost.Id, model.ChannelUserRoleId)
		_, appErr := at.th.App.PostAssistantCard(at.th.Context, at.maya.Id, at.channel.Id, card.Id)
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusForbidden, appErr.StatusCode)
	})

	t.Run("don't suggest this kind again", func(t *testing.T) {
		at.run(at.channel, at.post(at.maya, at.channel, "load on www-1?"))
		card := at.feed(at.maya, at.channel)[0]
		_, appErr := at.th.App.MuteAssistantCardTool(at.th.Context, at.maya.Id, at.channel.Id, card.Id)
		require.Nil(t, appErr)
		assert.True(t, at.th.App.GetAssistantMutedTools(at.maya.Id).Mutes(at.tool("host_metrics")))
		left := len(at.feed(at.maya, at.channel))
		at.run(at.channel, at.post(at.maya, at.channel, "disk on www-1?"))
		assert.Len(t, at.feed(at.maya, at.channel), left, "a muted tool isn't suggested any more")
	})
}

// setAssistantCardResult overwrites a card's result for the next PostAssistantCard call, as if a tool had
// returned it (ASSISTANT_DISPLAY.md #1), and puts it back in the "new" status so it posts again.
func (at *assistantTest) setAssistantCardResult(u *model.User, channel *model.Channel, card *model.AssistantCard, result assistantCardResult) {
	at.t.Helper()
	raw, err := json.Marshal(result)
	require.NoError(at.t, err)
	_, appErr := at.th.App.changeAssistantCard(at.th.Context, u.Id, channel.Id, card.Id, func(c *model.AssistantCard) *model.AppError {
		c.Status = model.AssistantCardStatusNew
		c.Result = string(raw)
		return nil
	})
	require.Nil(at.t, appErr)
}

// PostAssistantCard draws a formatted message from a result in the attachments format (ASSISTANT_DISPLAY.md #1/#4):
// the summary as the post's message, and the attachments sanitised into props.attachments. A result without
// attachments still posts as it always has (the title and the raw text).
func TestAssistantPostCardAttachments(t *testing.T) {
	at := setupAssistantTest(t)
	at.open(at.maya, at.channel)
	at.run(at.channel, at.post(at.maya, at.channel, "cpu on www-1?"))
	card := at.feed(at.maya, at.channel)[0]

	t.Run("an attachment result posts the summary and the attachments", func(t *testing.T) {
		threshold := 80.0
		at.setAssistantCardResult(at.maya, at.channel, card, assistantCardResult{
			Summary: "CPU on www-1: 4.1% now",
			Attachments: []*model.MessageAttachment{{
				Title:      "CPU on www-1",
				TitleLink:  "https://grafana.example/d/www-1",
				AuthorName: "Grafana",
				Text:       "4.1% now",
				Fields:     []*model.MessageAttachmentField{{Title: "Now", Value: "4.1%", Short: true}},
				Chart:      &model.MessageAttachmentChart{Values: []float64{3.1, 3.4, 4.1}, Threshold: &threshold, Label: "CPU %"},
			}},
		})

		post, appErr := at.th.App.PostAssistantCard(at.th.Context, at.maya.Id, at.channel.Id, card.Id)
		require.Nil(t, appErr)
		assert.Equal(t, "CPU on www-1: 4.1% now", post.Message)
		assert.Equal(t, model.PostTypeMessageAttachment, post.Type)

		attachments := post.Attachments()
		require.Len(t, attachments, 1)
		assert.Equal(t, "CPU on www-1", attachments[0].Title)
		assert.Equal(t, "https://grafana.example/d/www-1", attachments[0].TitleLink)
		require.Len(t, attachments[0].Fields, 1)
		assert.Equal(t, "4.1%", attachments[0].Fields[0].Value)
		require.NotNil(t, attachments[0].Chart)
		assert.Equal(t, []float64{3.1, 3.4, 4.1}, attachments[0].Chart.Values)
		assert.Equal(t, 80.0, *attachments[0].Chart.Threshold)
	})

	t.Run("actions are stripped", func(t *testing.T) {
		at.setAssistantCardResult(at.maya, at.channel, card, assistantCardResult{
			Summary: "ok",
			Attachments: []*model.MessageAttachment{{
				Title:   "t",
				Actions: []*model.PostAction{{Id: "ack", Name: "Acknowledge", Type: "button"}},
			}},
		})

		post, appErr := at.th.App.PostAssistantCard(at.th.Context, at.maya.Id, at.channel.Id, card.Id)
		require.Nil(t, appErr)
		attachments := post.Attachments()
		require.Len(t, attachments, 1)
		assert.Empty(t, attachments[0].Actions)
	})

	t.Run("unsafe links are dropped", func(t *testing.T) {
		at.setAssistantCardResult(at.maya, at.channel, card, assistantCardResult{
			Summary: "ok",
			Attachments: []*model.MessageAttachment{{
				Title:      "t",
				TitleLink:  "javascript:alert(1)",
				AuthorName: "a",
				AuthorLink: "ftp://example.com/a",
				AuthorIcon: "data:image/png;base64,xx",
				ImageURL:   "ftp://example.com/i.png",
				FooterIcon: "http://example.com/icon.png",
			}},
		})

		post, appErr := at.th.App.PostAssistantCard(at.th.Context, at.maya.Id, at.channel.Id, card.Id)
		require.Nil(t, appErr)
		attachments := post.Attachments()
		require.Len(t, attachments, 1)
		assert.Empty(t, attachments[0].TitleLink)
		assert.Empty(t, attachments[0].AuthorLink)
		assert.Empty(t, attachments[0].AuthorIcon)
		assert.Empty(t, attachments[0].ImageURL)
		assert.Equal(t, "http://example.com/icon.png", attachments[0].FooterIcon, "an http(s) icon is kept")
	})

	t.Run("caps are applied", func(t *testing.T) {
		many := make([]*model.MessageAttachment, 0, assistantPostAttachmentsMax+3)
		for i := 0; i < assistantPostAttachmentsMax+3; i++ {
			many = append(many, &model.MessageAttachment{Title: fmt.Sprintf("a%d", i)})
		}
		longText := strings.Repeat("x", assistantPostAttachmentTextMax+500)
		longValue := strings.Repeat("y", assistantPostFieldValueMax+100)
		many[0].Text = longText
		fields := make([]*model.MessageAttachmentField, 0, assistantPostFieldsMax+5)
		for i := 0; i < assistantPostFieldsMax+5; i++ {
			fields = append(fields, &model.MessageAttachmentField{Title: fmt.Sprintf("f%d", i), Value: longValue})
		}
		many[0].Fields = fields
		values := make([]float64, 0, assistantPostChartValuesMax+10)
		for i := 0; i < assistantPostChartValuesMax+10; i++ {
			values = append(values, float64(i))
		}
		many[0].Chart = &model.MessageAttachmentChart{Values: values}

		at.setAssistantCardResult(at.maya, at.channel, card, assistantCardResult{Summary: "ok", Attachments: many})

		post, appErr := at.th.App.PostAssistantCard(at.th.Context, at.maya.Id, at.channel.Id, card.Id)
		require.Nil(t, appErr)
		attachments := post.Attachments()
		assert.Len(t, attachments, assistantPostAttachmentsMax, "at most 10 attachments")
		assert.Len(t, attachments[0].Text, assistantPostAttachmentTextMax, "text capped")
		assert.Len(t, attachments[0].Fields, assistantPostFieldsMax, "fields capped")
		assert.Len(t, attachments[0].Fields[0].Value.(string), assistantPostFieldValueMax, "a field's value capped")
		require.NotNil(t, attachments[0].Chart)
		assert.Len(t, attachments[0].Chart.Values, assistantPostChartValuesMax, "chart values capped at 60 finite numbers")
		assert.Equal(t, float64(assistantPostChartValuesMax+10-1), attachments[0].Chart.Values[len(attachments[0].Chart.Values)-1], "the most recent values are kept")
	})

	t.Run("a text result is unchanged", func(t *testing.T) {
		_, appErr := at.th.App.changeAssistantCard(at.th.Context, at.maya.Id, at.channel.Id, card.Id, func(c *model.AssistantCard) *model.AppError {
			c.Status = model.AssistantCardStatusNew
			c.Result = "plain text, not JSON"
			return nil
		})
		require.Nil(t, appErr)

		post, appErr := at.th.App.PostAssistantCard(at.th.Context, at.maya.Id, at.channel.Id, card.Id)
		require.Nil(t, appErr)
		assert.Contains(t, post.Message, "plain text, not JSON")
		assert.Empty(t, post.Attachments())
	})

	t.Run("posting is refused for a non-member of a private channel", func(t *testing.T) {
		th := at.th
		private := th.CreatePrivateChannel(t, th.BasicTeam)
		th.AddUserToChannel(t, at.maya, private)
		at.open(at.maya, private)
		at.run(private, at.post(at.maya, private, "cpu on www-1?"))
		privateCard := at.feed(at.maya, private)[0]

		_, appErr := at.th.App.PostAssistantCard(at.th.Context, at.olivia.Id, private.Id, privateCard.Id)
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusForbidden, appErr.StatusCode)
	})
}

// sanitizeAssistantChart drops a non-finite value or threshold: valid JSON can't spell NaN or Infinity, but a
// tool's result is a JSON payload an app made up, so this stays defensive rather than trusting the parse.
func TestAssistantSanitizeChartDropsNonFinite(t *testing.T) {
	values := make([]float64, 0, assistantPostChartValuesMax+5)
	for i := 0; i < assistantPostChartValuesMax+5; i++ {
		values = append(values, float64(i))
	}
	threshold := math.NaN()
	clean := sanitizeAssistantChart(&model.MessageAttachmentChart{
		Values:    append(append([]float64{}, values...), math.NaN(), math.Inf(1), math.Inf(-1)),
		Threshold: &threshold,
		Label:     "CPU %",
	})

	require.NotNil(t, clean)
	assert.Len(t, clean.Values, assistantPostChartValuesMax, "at most 60 finite values")
	assert.Equal(t, values[len(values)-assistantPostChartValuesMax:], clean.Values, "the most recent values, oldest first")
	assert.Nil(t, clean.Threshold, "a non-finite threshold is dropped")
	assert.Equal(t, "CPU %", clean.Label)

	ok := 42.0
	cleanOK := sanitizeAssistantChart(&model.MessageAttachmentChart{Values: []float64{1, 2}, Threshold: &ok})
	require.NotNil(t, cleanOK.Threshold)
	assert.Equal(t, ok, *cleanOK.Threshold, "a finite threshold is kept")

	assert.Nil(t, sanitizeAssistantChart(nil))
}

// Direct and group messages and private channels count only for their members; archived channels, and channels
// where the person or the community turned the assistant off, give nothing.
func TestAssistantOnlyWhereThePersonMayRead(t *testing.T) {
	at := setupAssistantTest(t)
	th := at.th

	expectNothingFor := func(t *testing.T, u *model.User, channel *model.Channel) {
		t.Helper()
		feed := th.App.assistantFeed(u.Id, channel.Id)
		assert.Empty(t, feed.Cards)
	}

	t.Run("direct messages", func(t *testing.T) {
		dm := th.CreateDmChannel(t, at.sam)
		at.open(at.maya, dm)
		_, appErr := th.App.SetAssistantPresence(th.Context, at.olivia.Id, dm.Id, true)
		require.NotNil(t, appErr, "someone outside a direct message can't open the assistant on it")
		assert.Equal(t, http.StatusForbidden, appErr.StatusCode)
		_, appErr = th.App.GetAssistantFeed(th.Context, at.olivia.Id, dm.Id)
		require.NotNil(t, appErr)
		// Even with a presence slipped in, an outsider gets nothing.
		require.Nil(t, th.App.setAssistantPresenceEntry(th.Context, at.olivia.Id, dm.Id, true))
		at.run(dm, at.post(at.maya, dm, "cpu on www-1?"))
		assert.Len(t, at.feed(at.maya, dm), 1)
		expectNothingFor(t, at.olivia, dm)
	})

	t.Run("group messages", func(t *testing.T) {
		other := th.CreateUser(t)
		gm := th.CreateGroupChannel(t, at.sam, other)
		at.open(at.sam, gm)
		require.Nil(t, th.App.setAssistantPresenceEntry(th.Context, at.olivia.Id, gm.Id, true))
		at.run(gm, at.post(at.maya, gm, "cpu on www-1?"))
		assert.Len(t, at.feed(at.sam, gm), 1)
		expectNothingFor(t, at.olivia, gm)
	})

	t.Run("private channels", func(t *testing.T) {
		private := th.CreatePrivateChannel(t, th.BasicTeam)
		at.open(at.maya, private)
		_, appErr := th.App.SetAssistantPresence(th.Context, at.sam.Id, private.Id, true)
		require.NotNil(t, appErr)
		require.Nil(t, th.App.setAssistantPresenceEntry(th.Context, at.sam.Id, private.Id, true))
		at.run(private, at.post(at.maya, private, "cpu on www-1?"))
		assert.Len(t, at.feed(at.maya, private), 1)
		expectNothingFor(t, at.sam, private)
	})

	t.Run("the person's switch", func(t *testing.T) {
		channel := th.CreateChannel(t, th.BasicTeam)
		th.AddUserToChannel(t, at.sam, channel)
		at.open(at.maya, channel)
		at.open(at.sam, channel)
		at.run(channel, at.post(at.maya, channel, "cpu on www-1?"))
		require.Len(t, at.feed(at.sam, channel), 1)

		require.Nil(t, th.App.SetAssistantChannelOn(th.Context, at.sam.Id, channel.Id, false))
		assert.Empty(t, at.feed(at.sam, channel), "turning it off forgets the feed")
		presence, appErr := th.App.SetAssistantPresence(th.Context, at.sam.Id, channel.Id, true)
		require.Nil(t, appErr)
		assert.False(t, presence.Following)
		require.Nil(t, th.App.setAssistantPresenceEntry(th.Context, at.sam.Id, channel.Id, true))
		at.run(channel, at.post(at.maya, channel, "memory on www-1?"))
		assert.Len(t, at.feed(at.maya, channel), 2)
		expectNothingFor(t, at.sam, channel)
	})

	t.Run("the community's settings", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.AssistantSettings.AllowCommunityOverride = model.NewPointer(true) })
		defer th.App.UpdateConfig(func(cfg *model.Config) { cfg.AssistantSettings.AllowCommunityOverride = model.NewPointer(false) })
		channel := th.CreateChannel(t, th.BasicTeam)
		at.open(at.maya, channel)
		_, appErr := th.App.SetAssistantCommunitySettings(th.Context, th.BasicTeam.Id, &model.AssistantCommunitySettings{Enabled: model.NewPointer(false)})
		require.Nil(t, appErr)
		defer func() {
			_, _ = th.App.SetAssistantCommunitySettings(th.Context, th.BasicTeam.Id, &model.AssistantCommunitySettings{})
		}()
		at.run(channel, at.post(at.maya, channel, "cpu on www-1?"))
		expectNothingFor(t, at.maya, channel)
		feed, appErr := th.App.GetAssistantFeed(th.Context, at.maya.Id, channel.Id)
		require.Nil(t, appErr)
		assert.False(t, feed.Enabled)
	})

	t.Run("a community can't change anything unless the server lets it", func(t *testing.T) {
		_, appErr := th.App.SetAssistantCommunitySettings(th.Context, th.BasicTeam.Id, &model.AssistantCommunitySettings{Enabled: model.NewPointer(false)})
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusForbidden, appErr.StatusCode)
	})

	t.Run("archived channels", func(t *testing.T) {
		channel := th.CreateChannel(t, th.BasicTeam)
		at.open(at.maya, channel)
		post := at.post(at.maya, channel, "cpu on www-1?")
		require.Nil(t, th.App.DeleteChannel(th.Context, channel, at.maya.Id))
		at.run(channel, post)
		expectNothingFor(t, at.maya, channel)
		if presence, appErr := th.App.SetAssistantPresence(th.Context, at.maya.Id, channel.Id, true); appErr == nil {
			assert.False(t, presence.Following, "an archived channel isn't followed")
		}
	})

	t.Run("the server's switch", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.AssistantSettings.Enable = model.NewPointer(false) })
		defer th.App.UpdateConfig(func(cfg *model.Config) { cfg.AssistantSettings.Enable = model.NewPointer(true) })
		channel := th.CreateChannel(t, th.BasicTeam)
		require.Nil(t, th.App.setAssistantPresenceEntry(th.Context, at.maya.Id, channel.Id, true))
		at.run(channel, at.post(at.maya, channel, "cpu on www-1?"))
		expectNothingFor(t, at.maya, channel)
	})

	t.Run("bots don't start it", func(t *testing.T) {
		bot := th.CreateBot(t)
		botUser, appErr := th.App.GetUser(th.Context, bot.UserId)
		require.Nil(t, appErr)
		channel := th.CreateChannel(t, th.BasicTeam)
		at.open(at.maya, channel)
		at.run(channel, at.post(botUser, channel, "cpu on www-1?"))
		expectNothingFor(t, at.maya, channel)
	})
}

// The hourly budget of cards per person.
func TestAssistantCardBudget(t *testing.T) {
	at := setupAssistantTest(t)
	at.th.App.UpdateConfig(func(cfg *model.Config) { cfg.AssistantSettings.CardsPerUserHour = model.NewPointer(1) })
	at.open(at.maya, at.channel)
	at.run(at.channel, at.post(at.maya, at.channel, "cpu on www-1?"))
	at.run(at.channel, at.post(at.maya, at.channel, "memory on www-1?"))
	assert.Len(t, at.feed(at.maya, at.channel), 1, "the second card waits for the next hour")
}

// assistantOfferOf gives the offer of a person's only action card.
func (at *assistantTest) offerCard(u *model.User, channel *model.Channel) *model.AssistantCard {
	at.t.Helper()
	for _, c := range at.feed(u, channel) {
		if c.Kind == model.AssistantCardKindAction {
			return c
		}
	}
	require.Fail(at.t, "no action card for "+u.Username)
	return nil
}

func (at *assistantTest) holder(u *model.User, channel *model.Channel) string {
	return at.offerCard(u, channel).Offer.HolderId
}

// An action is offered to the author first; the others see whom it's offered to, without Run. Only the holder
// runs it, once: it's then done for everyone.
func TestAssistantActionGoesToTheAuthorFirst(t *testing.T) {
	at := setupAssistantTest(t)
	th := at.th
	at.open(at.olivia, at.channel)
	at.open(at.sam, at.channel)
	at.open(at.maya, at.channel)
	at.run(at.channel, at.post(at.maya, at.channel, "could we deploy web-app?"))

	mayaCard, samCard := at.offerCard(at.maya, at.channel), at.offerCard(at.sam, at.channel)
	assert.Equal(t, at.tool("deploy"), mayaCard.Tool)
	assert.JSONEq(t, `{"service":"web-app"}`, string(mayaCard.Arguments))
	assert.Empty(t, mayaCard.Result, "an action never runs on its own")
	assert.Equal(t, mayaCard.OfferId, samCard.OfferId, "one offer for everyone")
	assert.Equal(t, at.maya.Id, mayaCard.Offer.HolderId)
	assert.True(t, mayaCard.Offer.CanRun)
	assert.Equal(t, at.maya.Id, samCard.Offer.HolderId, "sam sees it's offered to maya")
	assert.False(t, samCard.Offer.CanRun)
	assert.False(t, samCard.Offer.OfferedToMe)
	assert.NotContains(t, at.mcp.called, "deploy")

	_, appErr := th.App.RunAssistantCard(th.Context, at.sam.Id, at.channel.Id, samCard.Id)
	require.NotNil(t, appErr, "only the holder runs it")
	assert.Equal(t, http.StatusForbidden, appErr.StatusCode)
	_, appErr = th.App.ClaimAssistantCard(th.Context, at.sam.Id, at.channel.Id, samCard.Id)
	require.NotNil(t, appErr, "nor takes it from them")
	assert.Equal(t, http.StatusForbidden, appErr.StatusCode)

	result, appErr := th.App.RunAssistantCard(th.Context, at.maya.Id, at.channel.Id, mayaCard.Id)
	require.Nil(t, appErr)
	assert.False(t, result.Executed, "stage 1 claims and closes the action; it runs from stage 3")
	assert.Equal(t, model.AssistantOfferStatusDone, result.Card.Offer.Status)
	for _, u := range []*model.User{at.maya, at.sam, at.olivia} {
		offer := at.offerCard(u, at.channel).Offer
		assert.Equal(t, model.AssistantOfferStatusDone, offer.Status, "done for %s too", u.Username)
		assert.False(t, offer.CanRun)
		assert.Equal(t, at.maya.Id, offer.DoneBy)
	}
	_, appErr = th.App.RunAssistantCard(th.Context, at.maya.Id, at.channel.Id, mayaCard.Id)
	require.NotNil(t, appErr, "a second run")
	assert.Equal(t, http.StatusConflict, appErr.StatusCode)

	t.Run("everyone with the card is told, each with their own view", func(t *testing.T) {
		seen := map[string]bool{}
		for _, e := range at.sentEvents() {
			if e.EventType() != model.WebsocketEventAssistantOfferUpdated {
				continue
			}
			offer, ok := e.GetData()["offer"].(*model.AssistantOfferView)
			require.True(t, ok)
			if offer.Status == model.AssistantOfferStatusDone {
				seen[e.GetBroadcast().UserId] = true
				assert.False(t, offer.CanRun)
			}
		}
		assert.Equal(t, map[string]bool{at.maya.Id: true, at.sam.Id: true, at.olivia.Id: true}, seen)
	})
}

// When the author passes (dismisses it) it goes to whoever spoke next in the window, then by who opened the
// assistant first; ignoring it for the timeout passes it too; someone who passed never gets it back.
func TestAssistantActionOfferOrder(t *testing.T) {
	at := setupAssistantTest(t)
	th := at.th
	at.open(at.olivia, at.channel)
	at.open(at.maya, at.channel)
	at.open(at.sam, at.channel)
	ask := at.post(at.maya, at.channel, "deploy web-app now?")
	reply := at.post(at.sam, at.channel, "yes please")
	at.run(at.channel, ask, reply)
	require.Equal(t, at.maya.Id, at.holder(at.maya, at.channel), "the author first")

	mayaCard := at.offerCard(at.maya, at.channel)
	_, appErr := th.App.DismissAssistantCard(th.Context, at.maya.Id, at.channel.Id, mayaCard.Id)
	require.Nil(t, appErr)
	assert.Equal(t, at.sam.Id, at.holder(at.sam, at.channel), "then who spoke next, before who opened first")

	t.Run("ignored for the timeout, it moves on", func(t *testing.T) {
		offerID := mayaCard.OfferId
		_, _, appErr := th.App.updateAssistantOffer(th.Context, offerID, func(o *assistantOffer) bool {
			o.HeldSince -= o.TimeoutMs + 1
			return true
		})
		require.Nil(t, appErr)
		assert.Equal(t, at.olivia.Id, at.holder(at.olivia, at.channel), "then by who opened first")
		_, appErr = th.App.RunAssistantCard(th.Context, at.sam.Id, at.channel.Id, at.offerCard(at.sam, at.channel).Id)
		require.NotNil(t, appErr, "sam's turn is over")
		assert.Equal(t, http.StatusForbidden, appErr.StatusCode)
	})

	t.Run("nobody left: it stays with the last holder", func(t *testing.T) {
		offerID := mayaCard.OfferId
		_, _, appErr := th.App.updateAssistantOffer(th.Context, offerID, func(o *assistantOffer) bool {
			o.HeldSince -= o.TimeoutMs + 1
			return true
		})
		require.Nil(t, appErr)
		assert.Equal(t, at.olivia.Id, at.holder(at.olivia, at.channel))
	})

	t.Run("the move is sent to everyone with the card", func(t *testing.T) {
		toSam := false
		for _, e := range at.sentEvents() {
			if e.EventType() != model.WebsocketEventAssistantOfferUpdated || e.GetBroadcast().UserId != at.sam.Id {
				continue
			}
			if offer := e.GetData()["offer"].(*model.AssistantOfferView); offer.HolderId == at.sam.Id {
				toSam = toSam || offer.OfferedToMe
			}
		}
		assert.True(t, toSam, "sam was told it's offered to him")
	})
}

// The author not viewing: it's offered to who opened first. Nobody viewing: to nobody, until someone who may comes
// back.
func TestAssistantActionWithoutTheAuthor(t *testing.T) {
	at := setupAssistantTest(t)
	th := at.th
	at.open(at.olivia, at.channel)
	at.open(at.sam, at.channel)
	at.run(at.channel, at.post(at.maya, at.channel, "deploy web-app"))
	assert.Equal(t, at.olivia.Id, at.holder(at.sam, at.channel))
	assert.Empty(t, th.App.assistantFeed(at.maya.Id, at.channel.Id).Cards, "the author wasn't viewing: no card for them")

	t.Run("nobody viewing", func(t *testing.T) {
		channel := th.CreateChannel(t, th.BasicTeam)
		th.AddUserToChannel(t, at.sam, channel)
		at.open(at.sam, channel)
		at.run(channel, at.post(at.maya, channel, "deploy web-app"))
		card := at.offerCard(at.sam, channel)
		require.Nil(t, th.App.setAssistantPresenceEntry(th.Context, at.sam.Id, channel.Id, false))
		th.App.assignAssistantOffer(th.Context, card.OfferId)
		assert.Empty(t, th.App.assistantOffer(card.OfferId).HolderId, "offered to nobody")
		at.open(at.sam, channel)
		assert.Equal(t, at.sam.Id, at.holder(at.sam, channel), "back with the one who may have it")
	})
}

// Two nodes running the same window make one offer; of many runs at once only one succeeds; of claims at once only
// one takes it.
func TestAssistantActionClaimsAreAtomic(t *testing.T) {
	at := setupAssistantTest(t)
	th := at.th
	at.open(at.sam, at.channel)
	at.open(at.olivia, at.channel)
	post := at.post(at.maya, at.channel, "deploy web-app")
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			at.run(at.channel, post)
		})
	}
	wg.Wait()
	samCards, oliviaCards := at.feed(at.sam, at.channel), at.feed(at.olivia, at.channel)
	require.Len(t, samCards, 1, "one card each")
	require.Len(t, oliviaCards, 1)
	assert.Equal(t, samCards[0].OfferId, oliviaCards[0].OfferId, "one offer")

	t.Run("concurrent claims", func(t *testing.T) {
		require.Nil(t, th.App.setAssistantPresenceEntry(th.Context, at.sam.Id, at.channel.Id, false))
		require.Nil(t, th.App.setAssistantPresenceEntry(th.Context, at.olivia.Id, at.channel.Id, false))
		th.App.assignAssistantOffer(th.Context, samCards[0].OfferId)
		require.Empty(t, th.App.assistantOffer(samCards[0].OfferId).HolderId)
		require.Nil(t, th.App.setAssistantPresenceEntry(th.Context, at.sam.Id, at.channel.Id, true))
		require.Nil(t, th.App.setAssistantPresenceEntry(th.Context, at.olivia.Id, at.channel.Id, true))

		var mu sync.Mutex
		var won []string
		var wg sync.WaitGroup
		for _, c := range []struct {
			user *model.User
			card *model.AssistantCard
		}{{at.sam, samCards[0]}, {at.olivia, oliviaCards[0]}, {at.sam, samCards[0]}, {at.olivia, oliviaCards[0]}} {
			wg.Go(func() {
				if card, appErr := th.App.ClaimAssistantCard(th.Context, c.user.Id, at.channel.Id, c.card.Id); appErr == nil && card.Offer.HolderId == c.user.Id {
					mu.Lock()
					won = append(won, c.user.Id)
					mu.Unlock()
				}
			})
		}
		wg.Wait()
		require.NotEmpty(t, won)
		for _, id := range won {
			assert.Equal(t, won[0], id, "one person holds it")
		}
		assert.Equal(t, won[0], th.App.assistantOffer(samCards[0].OfferId).HolderId)
	})

	t.Run("concurrent runs", func(t *testing.T) {
		holderID := th.App.assistantOffer(samCards[0].OfferId).HolderId
		card := samCards[0]
		if holderID == at.olivia.Id {
			card = oliviaCards[0]
		}
		var mu sync.Mutex
		ok, conflicts := 0, 0
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				_, appErr := th.App.RunAssistantCard(th.Context, holderID, at.channel.Id, card.Id)
				mu.Lock()
				defer mu.Unlock()
				switch {
				case appErr == nil:
					ok++
				case appErr.StatusCode == http.StatusConflict:
					conflicts++
				}
			})
		}
		wg.Wait()
		assert.Equal(t, 1, ok, "it runs once")
		assert.Equal(t, 7, conflicts, "the others are refused with 409")
	})
}

// decisionsAnswer is a fake decision model: it answers the first question set (card, tool, message) and the
// arguments' questions.
type decisionsAnswer struct {
	mu        sync.Mutex
	card      float64
	tool      string
	toolConf  float64
	argChoice string
	argConf   float64
	asked     []map[string]any
}

func (d *decisionsAnswer) answer(body map[string]any) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.asked = append(d.asked, body)
	questions, _ := body["questions"].([]any)
	var answers []string
	for _, q := range questions {
		question := q.(map[string]any)
		name := question["name"].(string)
		switch {
		case name == "card":
			answers = append(answers, fmt.Sprintf(`{"type":"predicate","name":"card","probability":%v}`, d.card))
		case name == "tool":
			answers = append(answers, fmt.Sprintf(`{"type":"choice","name":"tool","choice":%q,"confidence":%v}`, d.tool, d.toolConf))
		case name == "message":
			answers = append(answers, `{"type":"choice","name":"message","choice":"m1","confidence":0.9}`)
		case strings.HasPrefix(name, "arg_"):
			// Only the host is given; any other argument is none.
			choice, confidence := "none", 0.9
			if instructions, _ := question["instructions"].(string); strings.Contains(instructions, "give host") {
				choice, confidence = d.argChoice, d.argConf
			}
			answers = append(answers, fmt.Sprintf(`{"type":"choice","name":%q,"choice":%q,"confidence":%v}`, name, choice, confidence))
		}
	}
	return `{"model":"decider","answers":[` + strings.Join(answers, ",") + `],"usage":{"input_tokens":10,"output_tokens":0}}`
}

func (d *decisionsAnswer) questions(i int) map[string]map[string]any {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := map[string]map[string]any{}
	if i >= len(d.asked) {
		return out
	}
	for _, q := range d.asked[i]["questions"].([]any) {
		question := q.(map[string]any)
		out[question["name"].(string)] = question
	}
	return out
}

func choiceValues(question map[string]any) []string {
	var values []string
	for _, c := range question["choices"].([]any) {
		values = append(values, c.(map[string]any)["value"].(string))
	}
	return values
}

// The model engine: the decision model of the assistant's connection (never its chat model) picks one of the
// person's tools and arguments among the conversation's values, above a threshold, within the hourly budgets.
func TestAssistantModelEngine(t *testing.T) {
	at := setupAssistantTest(t)
	th := at.th
	llm := newFakeLLM(t, "chat answer")
	d := &decisionsAnswer{card: 0.9, tool: at.tool("host_metrics"), toolConf: 0.9, argChoice: "www-1", argConf: 0.95}
	llm.decide = d.answer
	c := createTestAIConnection(t, th, llm, model.AIConnectionScopeServer, func(req *model.AIConnectionRequest) {
		req.Models.Decisions = "decider"
	})
	th.App.UpdateConfig(func(cfg *model.Config) {
		s := &cfg.AssistantSettings
		s.Engine = model.NewPointer(model.AssistantEngineModel)
		s.ConnectionId = model.NewPointer(c.Id)
		s.ThresholdPercent = model.NewPointer(70)
	})
	at.open(at.maya, at.channel)
	decisions := func() int {
		n := 0
		for _, r := range llm.calls() {
			assert.NotContains(t, r.Path, "/chat/completions", "the chat model never decides")
			if strings.HasSuffix(r.Path, "/decisions") {
				n++
			}
		}
		return n
	}

	at.run(at.channel, at.post(at.maya, at.channel, "can someone look at www-1, it feels slow"), at.post(at.sam, at.channel, "agreed"))
	cards := at.feed(at.maya, at.channel)
	require.Len(t, cards, 1)
	assert.Equal(t, at.tool("host_metrics"), cards[0].Tool)
	assert.JSONEq(t, `{"host":"www-1"}`, string(cards[0].Arguments))
	assert.Equal(t, model.AssistantEngineModel, cards[0].Why.Engine)
	assert.InDelta(t, 0.9, cards[0].Why.Probabilities["card"], 1e-9, "the card shows the model's probabilities")
	assert.InDelta(t, 0.95, cards[0].Why.Probabilities["host"], 1e-9)
	assert.Contains(t, cards[0].Result, "as "+at.maya.Username)
	assert.Equal(t, 2, decisions())

	first := d.questions(0)
	tools := choiceValues(first["tool"])
	assert.Contains(t, tools, at.tool("host_metrics"))
	assert.Contains(t, tools, "none")
	for _, name := range tools {
		assert.True(t, name == "none" || strings.Contains(name, "."), "only tools' names: %s", name)
	}
	assert.Contains(t, choiceValues(first["message"]), "m2", "the model says which message asks")
	args := d.questions(1)
	require.Contains(t, args, "arg_0")
	assert.ElementsMatch(t, []string{"www-1", "none"}, choiceValues(args["arg_0"]), "an argument is chosen among the conversation's values only")

	cases := []struct {
		name   string
		change func()
	}{
		{"under the threshold", func() { d.card = 0.5 }},
		{"a tool the person doesn't have", func() { d.card, d.tool = 0.9, "mcp.elsewhere.drop_tables" }},
		{"no tool", func() { d.tool = "none" }},
		{"an unsure tool", func() { d.tool, d.toolConf = at.tool("host_metrics"), 0.4 }},
		{"an argument that isn't one of the choices", func() { d.toolConf, d.argChoice = 0.9, "evil-host" }},
		{"an unsure argument", func() { d.argChoice, d.argConf = "www-1", 0.3 }},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d.mu.Lock()
			tc.change()
			d.mu.Unlock()
			channel := th.CreateChannel(t, th.BasicTeam)
			at.open(at.maya, channel)
			at.run(channel, at.post(at.maya, channel, fmt.Sprintf("can someone look at www-1 (%d)", i)))
			assert.Empty(t, at.feed(at.maya, channel))
		})
	}

	t.Run("the hourly budgets", func(t *testing.T) {
		d.mu.Lock()
		d.card, d.tool, d.toolConf, d.argChoice, d.argConf = 0.9, at.tool("host_metrics"), 0.9, "www-1", 0.9
		d.mu.Unlock()
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.AssistantSettings.DecisionsPerChannelHour = model.NewPointer(1) })
		channel := th.CreateChannel(t, th.BasicTeam)
		at.open(at.maya, channel)
		before := decisions()
		at.run(channel, at.post(at.maya, channel, "can someone look at www-1"))
		assert.Equal(t, before+1, decisions(), "one decision, then the channel's budget is spent")
		assert.Empty(t, at.feed(at.maya, channel))
		at.run(channel, at.post(at.maya, channel, "please look at www-1"))
		assert.Equal(t, before+1, decisions(), "no more decisions this hour")

		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.AssistantSettings.DecisionsPerChannelHour = model.NewPointer(60)
			cfg.AssistantSettings.DecisionsPerUserHour = model.NewPointer(0)
		})
		other := th.CreateChannel(t, th.BasicTeam)
		at.open(at.maya, other)
		at.run(other, at.post(at.maya, other, "can someone look at www-1"))
		assert.Equal(t, before+1, decisions(), "the person's budget too")
	})

	t.Run("no decision model, no decisions", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.AssistantSettings.DecisionsPerUserHour = model.NewPointer(30) })
		chatOnly := createTestAIConnection(t, th, llm, model.AIConnectionScopeServer, nil)
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.AssistantSettings.ConnectionId = model.NewPointer(chatOnly.Id) })
		channel := th.CreateChannel(t, th.BasicTeam)
		at.open(at.maya, channel)
		before := decisions()
		at.run(channel, at.post(at.maya, channel, "can someone look at www-1"))
		assert.Equal(t, before, decisions())
		assert.Empty(t, at.feed(at.maya, channel))
	})
}

// fakeAssistantAppTool is an app's tool, named as RegisterAIAppTools names it, whose text result the test gives. The
// app's hook is faked as a built-in runner: what's under test is what the assistant does with the text.
func fakeAssistantAppTool(appID, name, schema string, run func(arguments json.RawMessage) (string, error)) *aiResolvedTool {
	tool := &model.AITool{
		Name: model.AIToolFullName(appID, name), Namespace: appID, LocalName: name, Source: model.AIToolSourceApp, SourceId: appID,
		Title: name, Kind: model.AIToolKindRead, InputSchema: json.RawMessage(schema),
	}
	return &aiResolvedTool{tool: tool, builtin: &aiBuiltinTool{tool: tool, run: func(_ *App, _ request.CTX, call *aiToolCall) (string, error) {
		return run(call.arguments)
	}}}
}

// ageAssistantDictionaries makes the cached listings older, as if by.
func ageAssistantDictionaries(th *TestHelper, by time.Duration) {
	st := th.App.assistant()
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, cached := range st.dicts {
		cached.expires = cached.expires.Add(-by)
	}
}

// The dictionary of the settings' sources, filled from an app's listing (its JSON as the tool's text): the hosts it
// lists are entities, so "and the disk on www-1?" picks the app's host_metrics with the host and the metric. A
// listing that failed is tried again after a minute; one that worked is kept ten minutes.
func TestAssistantDictionaryFromAnAppListing(t *testing.T) {
	th := setupAITest(t)
	th.App.UpdateConfig(func(cfg *model.Config) {
		s := &cfg.AssistantSettings
		s.Enable = model.NewPointer(true)
		s.Engine = model.NewPointer(model.AssistantEngineRules)
		s.Patterns = nil
		s.SeedEntities = nil
		s.DictionarySources = json.RawMessage(`[{"tool":"*search_dashboards","type":"dashboard","field":"title"},{"tool":"*list_datasources","type":"service","field":"name"},{"tool":"*list_hosts","type":"host","field":"host"}]`)
	})
	const grafana = "com.antimatterchat.grafana"
	var (
		mu       sync.Mutex
		listErr  error
		listings int
	)
	listHosts := fakeAssistantAppTool(grafana, "list_hosts", `{"type":"object","properties":{}}`, func(json.RawMessage) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		listings++
		if listErr != nil {
			return "", listErr
		}
		return `[{"host":"localhost:9090"},{"host":"www-1"}]`, nil
	})
	hostMetrics := fakeAssistantAppTool(grafana, "host_metrics", `{"type":"object","properties":{"host":{"type":"string"},"metric":{"type":"string"},"from":{"type":"string"},"to":{"type":"string"}},"required":["host","metric"]}`, func(arguments json.RawMessage) (string, error) {
		return "metrics " + string(arguments), nil
	})
	searchDashboards := fakeAssistantAppTool(grafana, "search_dashboards", `{"type":"object","properties":{"query":{"type":"string"}}}`, func(json.RawMessage) (string, error) {
		return `[{"uid":"n1","title":"Node Exporter Full"}]`, nil
	})
	listDatasources := fakeAssistantAppTool(grafana, "list_datasources", `{"type":"object","properties":{}}`, func(json.RawMessage) (string, error) {
		return `[{"uid":"p1","name":"Prometheus","type":"prometheus"}]`, nil
	})
	tools := []*aiResolvedTool{hostMetrics, listDatasources, listHosts, searchDashboards}
	channel := th.BasicChannel
	cfg := th.App.assistantConfigFor(channel.TeamId)
	post := &model.Post{Id: model.NewId(), ChannelId: channel.Id, UserId: th.BasicUser2.Id, Message: "and the disk on www-1?"}
	candidates := func(u *model.User) []*assistantCandidate {
		dict := th.App.assistantDictionaryFor(th.Context, u, channel, tools, cfg)
		got, _ := assistantRuleCandidates(post, dict, cfg.patterns, tools)
		return got
	}
	listed := func() int {
		mu.Lock()
		defer mu.Unlock()
		return listings
	}

	t.Run("the hosts the app lists are entities", func(t *testing.T) {
		got := candidates(th.BasicUser)
		require.Len(t, got, 1)
		assert.Equal(t, grafana+".host_metrics", got[0].Tool)
		assert.JSONEq(t, `{"host":"www-1","metric":"disk"}`, string(got[0].Arguments))
		assert.Equal(t, []string{"disk on www-1"}, got[0].Why.Words)
		assert.Equal(t, 1, listed())
	})

	t.Run("a listing that failed is tried again after a minute", func(t *testing.T) {
		mu.Lock()
		listErr = errors.New("The Grafana app isn't configured on this server yet.")
		mu.Unlock()
		assert.Empty(t, candidates(th.BasicUser2), "no listing, no host")
		assert.Equal(t, 2, listed())
		dict := th.App.assistantDictionaryFor(th.Context, th.BasicUser2, channel, tools, cfg)
		_, why := assistantRuleCandidates(post, dict, cfg.patterns, tools)
		assert.Equal(t, "no pattern matched (entities: number)", why, "without the host, www-1's 1 is a number")
		assert.Equal(t, 2, listed(), "the failure is kept a moment")

		mu.Lock()
		listErr = nil
		mu.Unlock()
		st := th.App.assistant()
		st.mu.Lock()
		for _, cached := range st.dicts {
			if cached.err != nil {
				assert.WithinDuration(t, time.Now().Add(assistantDictionaryRetry), cached.expires, 5*time.Second, "a failed listing is kept a minute")
			}
		}
		st.mu.Unlock()
		ageAssistantDictionaries(th, assistantDictionaryRetry+time.Second)
		got := candidates(th.BasicUser2)
		assert.Equal(t, 3, listed(), "listed again a minute later")
		require.Len(t, got, 1)
		assert.JSONEq(t, `{"host":"www-1","metric":"disk"}`, string(got[0].Arguments))
	})

	t.Run("a listing that worked is kept ten minutes", func(t *testing.T) {
		before := listed()
		require.Len(t, candidates(th.BasicUser), 1)
		assert.Equal(t, before, listed())
		ageAssistantDictionaries(th, 10*time.Minute)
		require.Len(t, candidates(th.BasicUser), 1)
		assert.Equal(t, before+1, listed(), "listed again ten minutes later")
	})
}
