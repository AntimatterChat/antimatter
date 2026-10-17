// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// messageActionReceiver is a callback URL that records what it was sent, and answers with answer.
type messageActionReceiver struct {
	*httptest.Server
	mu          sync.Mutex
	calls       int
	contentType string
	body        []byte
	answer      string
}

func newMessageActionReceiver(t *testing.T, answer string) *messageActionReceiver {
	rec := &messageActionReceiver{answer: answer}
	rec.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		rec.mu.Lock()
		rec.calls++
		rec.contentType = r.Header.Get("Content-Type")
		rec.body = body
		rec.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(rec.answer))
	}))
	t.Cleanup(rec.Close)
	return rec
}

func (rec *messageActionReceiver) payload(t *testing.T) *model.MessageActionPayload {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	require.Equal(t, 1, rec.calls, "the callback URL should have been called once")
	var p model.MessageActionPayload
	require.NoError(t, json.Unmarshal(rec.body, &p))
	return &p
}

func (rec *messageActionReceiver) callCount() int {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return rec.calls
}

func setupMessageActions(t *testing.T) *TestHelper {
	th := Setup(t).InitBasic(t)
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableOutgoingWebhooks = true
		*cfg.ServiceSettings.AllowedUntrustedInternalConnections = "localhost,127.0.0.1"
		*cfg.ServiceSettings.SiteURL = "https://chat.example.com"
	})
	return th
}

func createMessageAction(t *testing.T, th *TestHelper, channelID, callbackURL string) *model.OutgoingWebhook {
	hook, appErr := th.App.CreateOutgoingWebhook(&model.OutgoingWebhook{
		TeamId:       th.BasicTeam.Id,
		ChannelId:    channelID,
		CreatorId:    th.BasicUser.Id,
		TriggerWhen:  model.OutgoingWebhookTriggerWhenMessageAction,
		CallbackURLs: []string{callbackURL},
		DisplayName:  "Archive",
		Description:  "Files the message in the archive",
		ContentType:  "application/json",
	})
	require.Nil(t, appErr)
	return hook
}

func TestMessageActionCovers(t *testing.T) {
	mainHelper.Parallel(t)

	teamID := model.NewId()
	open := &model.Channel{Id: model.NewId(), TeamId: teamID, Type: model.ChannelTypeOpen}
	otherOpen := &model.Channel{Id: model.NewId(), TeamId: teamID, Type: model.ChannelTypeOpen}
	private := &model.Channel{Id: model.NewId(), TeamId: teamID, Type: model.ChannelTypePrivate}
	direct := &model.Channel{Id: model.NewId(), Type: model.ChannelTypeDirect}
	otherTeam := &model.Channel{Id: model.NewId(), TeamId: model.NewId(), Type: model.ChannelTypeOpen}
	member := &model.User{Id: model.NewId(), Roles: model.SystemUserRoleId}
	guest := &model.User{Id: model.NewId(), Roles: model.SystemGuestRoleId}

	teamWide := &model.OutgoingWebhook{Id: model.NewId(), TeamId: teamID, TriggerWhen: model.OutgoingWebhookTriggerWhenMessageAction}
	bound := &model.OutgoingWebhook{Id: model.NewId(), TeamId: teamID, ChannelId: open.Id, TriggerWhen: model.OutgoingWebhookTriggerWhenMessageAction}
	triggerWords := &model.OutgoingWebhook{Id: model.NewId(), TeamId: teamID, ChannelId: open.Id, TriggerWords: []string{"go"}}
	deleted := &model.OutgoingWebhook{Id: model.NewId(), TeamId: teamID, TriggerWhen: model.OutgoingWebhookTriggerWhenMessageAction, DeleteAt: 1}

	for name, tc := range map[string]struct {
		hook    *model.OutgoingWebhook
		channel *model.Channel
		user    *model.User
		want    bool
	}{
		"a community-wide action in a public channel":    {teamWide, open, member, true},
		"a community-wide action in a private channel":   {teamWide, private, member, false},
		"a community-wide action in a direct message":    {teamWide, direct, member, false},
		"a community-wide action in another community":   {teamWide, otherTeam, member, false},
		"a community-wide action, for a guest":           {teamWide, open, guest, false},
		"an action bound to the channel":                 {bound, open, member, true},
		"an action bound to the channel, for a guest":    {bound, open, guest, true},
		"an action bound to another channel":             {bound, otherOpen, member, false},
		"a trigger-word hook is not in the message menu": {triggerWords, open, member, false},
		"a deleted action":                               {deleted, open, member, false},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, messageActionCovers(tc.hook, tc.channel, tc.user))
		})
	}
}

func TestGetMessageActionsForChannel(t *testing.T) {
	mainHelper.Parallel(t)
	th := setupMessageActions(t)

	bound := createMessageAction(t, th, th.BasicChannel.Id, "https://example.com/bound")
	teamWide := createMessageAction(t, th, "", "https://example.com/team")
	_, appErr := th.App.CreateOutgoingWebhook(&model.OutgoingWebhook{
		TeamId:       th.BasicTeam.Id,
		ChannelId:    th.BasicChannel.Id,
		CreatorId:    th.BasicUser.Id,
		TriggerWords: []string{"deploy"},
		CallbackURLs: []string{"https://example.com/words"},
	})
	require.Nil(t, appErr)

	t.Run("lists the message actions covering the channel, without their tokens", func(t *testing.T) {
		actions, appErr := th.App.GetMessageActionsForChannel(th.Context, th.BasicChannel, th.BasicUser)
		require.Nil(t, appErr)
		ids := []string{}
		for _, a := range actions {
			ids = append(ids, a.Id)
		}
		assert.ElementsMatch(t, []string{bound.Id, teamWide.Id}, ids)
		js := string(model.ToJSON(actions))
		assert.NotContains(t, js, bound.Token)
		assert.NotContains(t, js, teamWide.Token)
		assert.NotContains(t, js, "example.com")
	})

	t.Run("a guest only gets the actions bound to the channel", func(t *testing.T) {
		guest := &model.User{Id: model.NewId(), Roles: model.SystemGuestRoleId}
		actions, appErr := th.App.GetMessageActionsForChannel(th.Context, th.BasicChannel, guest)
		require.Nil(t, appErr)
		require.Len(t, actions, 1)
		assert.Equal(t, bound.Id, actions[0].Id)
	})

	t.Run("none when outgoing webhooks are off", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOutgoingWebhooks = false })
		defer th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOutgoingWebhooks = true })
		actions, appErr := th.App.GetMessageActionsForChannel(th.Context, th.BasicChannel, th.BasicUser)
		require.Nil(t, appErr)
		assert.Empty(t, actions)
	})
}

func TestCreateMessageActionWebhook(t *testing.T) {
	mainHelper.Parallel(t)
	th := setupMessageActions(t)

	t.Run("a community-wide message action needs no trigger words", func(t *testing.T) {
		hook := createMessageAction(t, th, "", "https://example.com/hook")
		assert.True(t, hook.IsMessageAction())
	})

	t.Run("a message action can't have trigger words", func(t *testing.T) {
		_, appErr := th.App.CreateOutgoingWebhook(&model.OutgoingWebhook{
			TeamId:       th.BasicTeam.Id,
			CreatorId:    th.BasicUser.Id,
			TriggerWhen:  model.OutgoingWebhookTriggerWhenMessageAction,
			TriggerWords: []string{"archive"},
			CallbackURLs: []string{"https://example.com/hook"},
			DisplayName:  "Archive",
		})
		require.NotNil(t, appErr)
	})

	t.Run("an update can't give a message action trigger words", func(t *testing.T) {
		hook := createMessageAction(t, th, "", "https://example.com/hook2")
		updated := *hook
		updated.TriggerWords = []string{"archive"}
		_, appErr := th.App.UpdateOutgoingWebhook(th.Context, hook, &updated)
		require.NotNil(t, appErr)
		assert.Equal(t, "model.outgoing_hook.is_valid.message_action_words.app_error", appErr.Id)
	})

	t.Run("a trigger-word hook still needs a channel or trigger words", func(t *testing.T) {
		_, appErr := th.App.CreateOutgoingWebhook(&model.OutgoingWebhook{
			TeamId:       th.BasicTeam.Id,
			CreatorId:    th.BasicUser.Id,
			CallbackURLs: []string{"https://example.com/hook"},
		})
		require.NotNil(t, appErr)
		assert.Equal(t, "api.webhook.create_outgoing.triggers.app_error", appErr.Id)
	})
}

// A message action bound to a channel has no trigger words, like a hook firing on every message of its channel: it
// must still never fire on posted messages.
func TestMessageActionNotTriggeredByPosts(t *testing.T) {
	mainHelper.Parallel(t)
	th := setupMessageActions(t)

	rec := newMessageActionReceiver(t, `{}`)
	createMessageAction(t, th, th.BasicChannel.Id, rec.URL)

	post := th.CreatePost(t, th.BasicChannel)
	appErr := th.App.handleWebhookEvents(th.Context, post, th.BasicTeam, th.BasicChannel, th.BasicUser)
	require.Nil(t, appErr)
	assert.Equal(t, 0, rec.callCount())
}

func TestRunMessageActionPayload(t *testing.T) {
	mainHelper.Parallel(t)
	th := setupMessageActions(t)

	rec := newMessageActionReceiver(t, `{}`)
	hook := createMessageAction(t, th, th.BasicChannel.Id, rec.URL)

	postID := model.NewId()
	saveFile := func(name, postID, channelID string) *model.FileInfo {
		info, err := th.App.Srv().Store().FileInfo().Save(th.Context, &model.FileInfo{
			Name: name, MimeType: "text/plain", ChannelId: channelID, CreatorId: th.BasicUser.Id, PostId: postID, Path: "/path/" + name,
		})
		require.NoError(t, err)
		return info
	}
	mine := saveFile("report.pdf", postID, th.BasicChannel.Id)
	otherPost := saveFile("someone-else.pdf", model.NewId(), th.BasicChannel.Id)
	otherChannel := saveFile("elsewhere.pdf", postID, model.NewId())

	post := &model.Post{
		Id:        postID,
		UserId:    th.BasicUser.Id,
		ChannelId: th.BasicChannel.Id,
		Message:   "Quarterly report attached",
		CreateAt:  1700000000000,
		FileIds:   model.StringArray{mine.Id, otherPost.Id, otherChannel.Id},
	}

	// BasicUser2 picks the action for BasicUser's message.
	result, appErr := th.App.RunMessageAction(th.Context, hook.Id, post, th.BasicChannel, th.BasicUser2)
	require.Nil(t, appErr)
	assert.Empty(t, result.EphemeralText)

	assert.Equal(t, "application/json", rec.contentType)
	p := rec.payload(t)
	assert.Equal(t, model.MessageActionTrigger, p.Trigger)
	assert.Equal(t, hook.Token, p.Token)
	assert.Equal(t, th.BasicTeam.Id, p.TeamId)
	assert.Equal(t, th.BasicTeam.Name, p.TeamDomain)
	assert.Equal(t, th.BasicChannel.Id, p.ChannelId)
	assert.Equal(t, th.BasicChannel.Name, p.ChannelName)
	assert.Equal(t, th.BasicUser2.Id, p.UserId, "the person who picked the action")
	assert.Equal(t, th.BasicUser2.Username, p.UserName)
	assert.Equal(t, postID, p.PostId)
	assert.Equal(t, th.BasicUser.Id, p.PostUserId, "the message's author")
	assert.Equal(t, th.BasicUser.Username, p.PostUserName)
	assert.Equal(t, int64(1700000000000), p.PostCreateAt)
	assert.Equal(t, "Quarterly report attached", p.Text)
	assert.Equal(t, "https://chat.example.com/"+th.BasicTeam.Name+"/pl/"+postID, p.Permalink)
	assert.False(t, p.BurnOnRead)
	assert.NotZero(t, p.Timestamp)

	// Only the files of this message, in this channel.
	require.Len(t, p.Files, 1)
	assert.Equal(t, model.MessageActionFile{Id: mine.Id, Name: "report.pdf", Link: "https://chat.example.com/api/v4/files/" + mine.Id}, p.Files[0])
}

func TestRunMessageActionFormEncoded(t *testing.T) {
	mainHelper.Parallel(t)
	th := setupMessageActions(t)

	rec := newMessageActionReceiver(t, ``)
	hook := createMessageAction(t, th, th.BasicChannel.Id, rec.URL)
	hook.ContentType = "application/x-www-form-urlencoded"
	_, err := th.App.Srv().Store().Webhook().UpdateOutgoing(hook)
	require.NoError(t, err)

	_, appErr := th.App.RunMessageAction(th.Context, hook.Id, th.BasicPost, th.BasicChannel, th.BasicUser)
	require.Nil(t, appErr)

	assert.Equal(t, "application/x-www-form-urlencoded", rec.contentType)
	v, parseErr := url.ParseQuery(string(rec.body))
	require.NoError(t, parseErr)
	assert.Equal(t, "message_action", v.Get("trigger"))
	assert.Equal(t, th.BasicPost.Id, v.Get("post_id"))
	assert.Equal(t, th.BasicPost.Message, v.Get("text"))
	assert.Equal(t, hook.Token, v.Get("token"))
}

func TestRunMessageActionBurnOnRead(t *testing.T) {
	mainHelper.Parallel(t)
	th := setupMessageActions(t)

	rec := newMessageActionReceiver(t, `{}`)
	hook := createMessageAction(t, th, th.BasicChannel.Id, rec.URL)

	postID := model.NewId()
	info, err := th.App.Srv().Store().FileInfo().Save(th.Context, &model.FileInfo{
		Name: "secret.png", MimeType: "image/png", ChannelId: th.BasicChannel.Id, CreatorId: th.BasicUser.Id, PostId: postID, Path: "/path/secret.png",
	})
	require.NoError(t, err)
	post := &model.Post{
		Id:        postID,
		UserId:    th.BasicUser.Id,
		ChannelId: th.BasicChannel.Id,
		Type:      model.PostTypeBurnOnRead,
		Message:   "the door code is 4242",
		FileIds:   model.StringArray{info.Id},
	}

	_, appErr := th.App.RunMessageAction(th.Context, hook.Id, post, th.BasicChannel, th.BasicUser2)
	require.Nil(t, appErr)

	p := rec.payload(t)
	assert.True(t, p.BurnOnRead)
	assert.Equal(t, postID, p.PostId)
	assert.Empty(t, p.Text, "a burn-on-read message's text never leaves the server")
	assert.Empty(t, p.Files, "nor its files")
	assert.NotContains(t, string(rec.body), "4242")
	assert.NotContains(t, string(rec.body), "secret.png")
}

func TestRunMessageActionPriority(t *testing.T) {
	mainHelper.Parallel(t)
	th := setupMessageActions(t)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.PostPriority = true })
	rec := newMessageActionReceiver(t, `{}`)
	hook := createMessageAction(t, th, th.BasicChannel.Id, rec.URL)

	post := th.CreatePost(t, th.BasicChannel, func(p *model.Post) {
		p.Metadata = &model.PostMetadata{Priority: &model.PostPriority{Priority: model.NewPointer(model.PostPriorityUrgent), RequestedAck: model.NewPointer(true)}}
	})

	_, appErr := th.App.RunMessageAction(th.Context, hook.Id, post, th.BasicChannel, th.BasicUser)
	require.Nil(t, appErr)
	p := rec.payload(t)
	assert.Equal(t, model.PostPriorityUrgent, p.Priority)
	assert.True(t, p.RequestedAck)
}

func TestRunMessageActionRefusals(t *testing.T) {
	mainHelper.Parallel(t)
	th := setupMessageActions(t)

	rec := newMessageActionReceiver(t, `{}`)
	hook := createMessageAction(t, th, th.BasicChannel.Id, rec.URL)

	t.Run("not in another channel than its own", func(t *testing.T) {
		other := th.CreateChannel(t, th.BasicTeam)
		post := th.CreatePost(t, other)
		_, appErr := th.App.RunMessageAction(th.Context, hook.Id, post, other, th.BasicUser)
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusNotFound, appErr.StatusCode)
	})

	t.Run("not with a post from another channel than the one given", func(t *testing.T) {
		other := th.CreateChannel(t, th.BasicTeam)
		post := th.CreatePost(t, other)
		_, appErr := th.App.RunMessageAction(th.Context, hook.Id, post, th.BasicChannel, th.BasicUser)
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusNotFound, appErr.StatusCode)
	})

	t.Run("not a trigger-word hook", func(t *testing.T) {
		words, appErr := th.App.CreateOutgoingWebhook(&model.OutgoingWebhook{
			TeamId: th.BasicTeam.Id, ChannelId: th.BasicChannel.Id, CreatorId: th.BasicUser.Id,
			TriggerWords: []string{"deploy"}, CallbackURLs: []string{rec.URL},
		})
		require.Nil(t, appErr)
		_, appErr = th.App.RunMessageAction(th.Context, words.Id, th.BasicPost, th.BasicChannel, th.BasicUser)
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusNotFound, appErr.StatusCode)
	})

	t.Run("not a system message", func(t *testing.T) {
		system := &model.Post{Id: model.NewId(), ChannelId: th.BasicChannel.Id, UserId: th.BasicUser.Id, Type: model.PostTypeJoinChannel}
		_, appErr := th.App.RunMessageAction(th.Context, hook.Id, system, th.BasicChannel, th.BasicUser)
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusBadRequest, appErr.StatusCode)
	})

	t.Run("not when outgoing webhooks are off", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOutgoingWebhooks = false })
		defer th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOutgoingWebhooks = true })
		_, appErr := th.App.RunMessageAction(th.Context, hook.Id, th.BasicPost, th.BasicChannel, th.BasicUser)
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusNotImplemented, appErr.StatusCode)
	})

	assert.Equal(t, 0, rec.callCount(), "nothing was sent")
}

// Message actions go through the client for untrusted URLs: an internal address is refused unless
// AllowedUntrustedInternalConnections lists it.
func TestRunMessageActionSSRF(t *testing.T) {
	mainHelper.Parallel(t)
	th := setupMessageActions(t)

	rec := newMessageActionReceiver(t, `{}`)
	hook := createMessageAction(t, th, th.BasicChannel.Id, rec.URL)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.AllowedUntrustedInternalConnections = "" })
	_, appErr := th.App.RunMessageAction(th.Context, hook.Id, th.BasicPost, th.BasicChannel, th.BasicUser)
	require.NotNil(t, appErr)
	assert.Equal(t, http.StatusBadGateway, appErr.StatusCode)
	assert.Equal(t, "app.message_action.failed.app_error", appErr.Id)
	assert.Equal(t, 0, rec.callCount(), "the internal address must not be reached")

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.AllowedUntrustedInternalConnections = "127.0.0.1" })
	_, appErr = th.App.RunMessageAction(th.Context, hook.Id, th.BasicPost, th.BasicChannel, th.BasicUser)
	require.Nil(t, appErr)
	assert.Equal(t, 1, rec.callCount())
}

func TestRunMessageActionAnswers(t *testing.T) {
	mainHelper.Parallel(t)
	th := setupMessageActions(t)

	t.Run("the ephemeral text goes back to the person alone", func(t *testing.T) {
		rec := newMessageActionReceiver(t, `{"ephemeral_text": "Filed as ticket 42"}`)
		hook := createMessageAction(t, th, th.BasicChannel.Id, rec.URL)
		before, appErr := th.App.GetPosts(th.Context, th.BasicChannel.Id, 0, 100)
		require.Nil(t, appErr)

		result, appErr := th.App.RunMessageAction(th.Context, hook.Id, th.BasicPost, th.BasicChannel, th.BasicUser)
		require.Nil(t, appErr)
		assert.Equal(t, "Filed as ticket 42", result.EphemeralText)

		after, appErr := th.App.GetPosts(th.Context, th.BasicChannel.Id, 0, 100)
		require.Nil(t, appErr)
		assert.Equal(t, len(before.Order), len(after.Order), "nothing is posted in the channel")
	})

	t.Run("a comment answer is posted in the message's thread", func(t *testing.T) {
		rec := newMessageActionReceiver(t, `{"text": "Archived", "response_type": "comment"}`)
		hook := createMessageAction(t, th, th.BasicChannel.Id, rec.URL)
		post := th.CreatePost(t, th.BasicChannel)

		_, appErr := th.App.RunMessageAction(th.Context, hook.Id, post, th.BasicChannel, th.BasicUser)
		require.Nil(t, appErr)

		thread, appErr := th.App.GetPostThread(th.Context, post.Id, model.GetPostsOptions{}, th.BasicUser.Id)
		require.Nil(t, appErr)
		found := false
		for _, p := range thread.Posts {
			if p.RootId == post.Id && p.Message == "Archived" {
				found = true
				assert.Equal(t, "true", p.GetProp(model.PostPropsFromWebhook))
			}
		}
		assert.True(t, found, "the answer should be a reply to the message")
	})

	t.Run("an archived channel's message is still sent, but the answer isn't posted", func(t *testing.T) {
		channel := th.CreateChannel(t, th.BasicTeam)
		post := th.CreatePost(t, channel)
		rec := newMessageActionReceiver(t, `{"text": "Archived"}`)
		// Archiving a channel deletes the webhooks bound to it: a community-wide action still covers it.
		hook := createMessageAction(t, th, "", rec.URL)
		require.Nil(t, th.App.DeleteChannel(th.Context, channel, th.SystemAdminUser.Id))
		archived, appErr := th.App.GetChannel(th.Context, channel.Id)
		require.Nil(t, appErr)
		require.NotZero(t, archived.DeleteAt)
		before, appErr := th.App.GetPosts(th.Context, channel.Id, 0, 100)
		require.Nil(t, appErr)

		_, appErr = th.App.RunMessageAction(th.Context, hook.Id, post, archived, th.BasicUser)
		require.Nil(t, appErr)
		assert.Equal(t, post.Id, rec.payload(t).PostId)

		after, appErr := th.App.GetPosts(th.Context, channel.Id, 0, 100)
		require.Nil(t, appErr)
		assert.Equal(t, len(before.Order), len(after.Order), "no answer is posted in an archived channel")
	})
}

func TestRunMessageActionRateLimit(t *testing.T) {
	mainHelper.Parallel(t)
	th := setupMessageActions(t)

	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()
	hook := createMessageAction(t, th, th.BasicChannel.Id, ts.URL)

	limited := false
	for range messageActionsBurst + 3 {
		_, appErr := th.App.RunMessageAction(th.Context, hook.Id, th.BasicPost, th.BasicChannel, th.BasicUser)
		if appErr != nil {
			require.Equal(t, http.StatusTooManyRequests, appErr.StatusCode)
			limited = true
			break
		}
	}
	require.True(t, limited, "running message actions in a burst should hit the limit")
	sent := calls.Load()

	// The limit is per person: someone else still may.
	_, appErr := th.App.RunMessageAction(th.Context, hook.Id, th.BasicPost, th.BasicChannel, th.BasicUser2)
	require.Nil(t, appErr)
	assert.Equal(t, sent+1, calls.Load())
}
