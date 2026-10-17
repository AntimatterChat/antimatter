// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validMessageActionHook() *OutgoingWebhook {
	return &OutgoingWebhook{
		Id:           NewId(),
		Token:        NewId(),
		CreateAt:     GetMillis(),
		UpdateAt:     GetMillis(),
		CreatorId:    NewId(),
		TeamId:       NewId(),
		TriggerWhen:  OutgoingWebhookTriggerWhenMessageAction,
		CallbackURLs: []string{"https://example.com/hook"},
		DisplayName:  "Send to the archive",
	}
}

func TestOutgoingWebhookMessageActionIsValid(t *testing.T) {
	t.Run("a team-wide message action without trigger words is valid", func(t *testing.T) {
		require.Nil(t, validMessageActionHook().IsValid())
	})

	t.Run("a message action never has trigger words", func(t *testing.T) {
		o := validMessageActionHook()
		o.TriggerWords = []string{"archive"}
		appErr := o.IsValid()
		require.NotNil(t, appErr)
		assert.Equal(t, "model.outgoing_hook.is_valid.message_action_words.app_error", appErr.Id)
	})

	t.Run("a message action needs a name, the label of its menu entry", func(t *testing.T) {
		o := validMessageActionHook()
		o.DisplayName = "   "
		appErr := o.IsValid()
		require.NotNil(t, appErr)
		assert.Equal(t, "model.outgoing_hook.is_valid.message_action_name.app_error", appErr.Id)
	})

	t.Run("trigger kinds past the message action are refused", func(t *testing.T) {
		o := validMessageActionHook()
		o.TriggerWhen = OutgoingWebhookTriggerWhenMessageAction + 1
		require.NotNil(t, o.IsValid())
	})

	t.Run("trigger-word hooks keep their rules", func(t *testing.T) {
		o := validMessageActionHook()
		o.TriggerWhen = OutgoingWebhookTriggerWhenStartsWith
		o.DisplayName = ""
		o.TriggerWords = []string{"archive"}
		require.Nil(t, o.IsValid())
		assert.False(t, o.IsMessageAction())
	})
}

func TestOutgoingWebhookToMessageAction(t *testing.T) {
	t.Run("never carries the token nor the callback URLs", func(t *testing.T) {
		o := validMessageActionHook()
		o.ChannelId = NewId()
		o.Description = "Files the message"
		o.IconURL = "https://example.com/icon.png"
		a := o.ToMessageAction()
		assert.Equal(t, &MessageAction{
			Id:          o.Id,
			TeamId:      o.TeamId,
			ChannelId:   o.ChannelId,
			DisplayName: "Send to the archive",
			Description: "Files the message",
			IconURL:     "https://example.com/icon.png",
		}, a)
		js := string(ToJSON(a))
		assert.NotContains(t, js, o.Token)
		assert.NotContains(t, js, "example.com/hook")
	})

	t.Run("keeps an uploaded icon", func(t *testing.T) {
		o := validMessageActionHook()
		o.IconURL = OutgoingWebhookIconPath(o.Id) + "?_=123"
		assert.True(t, o.HasUploadedIcon())
		assert.Equal(t, o.IconURL, o.ToMessageAction().IconURL)
	})

	for name, icon := range map[string]string{
		"a data URI":                   "data:image/svg+xml;base64,PHN2Zz48L3N2Zz4=",
		"another path of the server":   "/api/v4/users/me",
		"another hook's uploaded icon": OutgoingWebhookIconPath(NewId()),
		"a javascript URL":             "javascript:alert(1)",
	} {
		t.Run("drops "+name, func(t *testing.T) {
			o := validMessageActionHook()
			o.IconURL = icon
			assert.Empty(t, o.ToMessageAction().IconURL)
		})
	}
}

func TestMessageActionPayloadToFormValues(t *testing.T) {
	p := &MessageActionPayload{
		Token:        "tok",
		Trigger:      MessageActionTrigger,
		TeamId:       "team",
		TeamDomain:   "lumen",
		ChannelId:    "chan",
		ChannelName:  "general",
		Timestamp:    1700000000123,
		UserId:       "clicker",
		UserName:     "maya",
		PostId:       "post",
		PostRootId:   "root",
		PostCreateAt: 1700000000000,
		PostUserId:   "author",
		PostUserName: "sam",
		Permalink:    "https://chat.example.com/lumen/pl/post",
		Text:         "hello, world",
		Files: []MessageActionFile{
			{Id: "f1", Name: "a, b.png", Link: "https://chat.example.com/api/v4/files/f1"},
			{Id: "f2", Name: "c.pdf", Link: "https://chat.example.com/api/v4/files/f2"},
		},
		Priority:     PostPriorityUrgent,
		RequestedAck: true,
	}

	v, err := url.ParseQuery(p.ToFormValues())
	require.NoError(t, err)
	assert.Equal(t, "message_action", v.Get("trigger"))
	assert.Equal(t, "tok", v.Get("token"))
	assert.Equal(t, "maya", v.Get("user_name"))
	assert.Equal(t, "sam", v.Get("post_user_name"))
	assert.Equal(t, "hello, world", v.Get("text"))
	assert.Equal(t, "1700000000123", v.Get("timestamp"))
	assert.Equal(t, []string{"a, b.png", "c.pdf"}, v["file_name"])
	assert.Equal(t, []string{"https://chat.example.com/api/v4/files/f1", "https://chat.example.com/api/v4/files/f2"}, v["file_link"])
	assert.Equal(t, "urgent", v.Get("priority"))
	assert.Equal(t, "true", v.Get("requested_ack"))
	assert.Equal(t, "false", v.Get("burn_on_read"))
}
