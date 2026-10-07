// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostGetReplyToProp(t *testing.T) {
	post := &Post{}
	assert.Equal(t, "", post.GetReplyToProp())

	id := NewId()
	post.AddProp(PostPropsReplyTo, id)
	assert.Equal(t, id, post.GetReplyToProp())

	post.AddProp(PostPropsReplyTo, 42)
	assert.Equal(t, "", post.GetReplyToProp())
}

func TestPostReplyToMentionsAuthor(t *testing.T) {
	post := &Post{}
	assert.True(t, post.ReplyToMentionsAuthor())

	post.AddProp(PostPropsReplyToMention, false)
	assert.False(t, post.ReplyToMentionsAuthor())

	post.AddProp(PostPropsReplyToMention, true)
	assert.True(t, post.ReplyToMentionsAuthor())

	post.AddProp(PostPropsReplyToMention, "false")
	assert.True(t, post.ReplyToMentionsAuthor())
}

func TestNewPostReplyTo(t *testing.T) {
	channelID := NewId()
	reply := &Post{Id: NewId(), ChannelId: channelID}
	target := &Post{
		Id:        NewId(),
		ChannelId: channelID,
		UserId:    NewId(),
		Message:   "the original message",
		CreateAt:  1000,
		EditAt:    2000,
		FileIds:   StringArray{NewId(), NewId()},
	}

	t.Run("describes the quoted message", func(t *testing.T) {
		replyTo := NewPostReplyTo(reply, target.Id, target)
		assert.Equal(t, &PostReplyTo{
			PostId:    target.Id,
			UserId:    target.UserId,
			Message:   "the original message",
			CreateAt:  1000,
			EditAt:    2000,
			FileCount: 2,
		}, replyTo)
	})

	t.Run("shortens long messages", func(t *testing.T) {
		long := target.Clone()
		long.Message = strings.Repeat("é", PostReplyToMessageMaxRunes+50)
		replyTo := NewPostReplyTo(reply, long.Id, long)
		assert.Equal(t, PostReplyToMessageMaxRunes, utf8.RuneCountInString(replyTo.Message))
		assert.True(t, strings.HasSuffix(replyTo.Message, "…"))
	})

	t.Run("names the webhook a message was posted under", func(t *testing.T) {
		hook := target.Clone()
		hook.AddProp(PostPropsOverrideUsername, "ci")
		assert.Empty(t, NewPostReplyTo(reply, hook.Id, hook).OverrideUsername, "override without from_webhook")

		hook.AddProp(PostPropsFromWebhook, "true")
		assert.Equal(t, "ci", NewPostReplyTo(reply, hook.Id, hook).OverrideUsername)
	})

	deleted := &PostReplyTo{PostId: target.Id, Deleted: true}
	t.Run("hides a missing message", func(t *testing.T) {
		assert.Equal(t, deleted, NewPostReplyTo(reply, target.Id, nil))
	})

	t.Run("hides a deleted message", func(t *testing.T) {
		gone := target.Clone()
		gone.DeleteAt = 3000
		assert.Equal(t, deleted, NewPostReplyTo(reply, gone.Id, gone))
	})

	t.Run("hides a message from another channel", func(t *testing.T) {
		moved := target.Clone()
		moved.ChannelId = NewId()
		assert.Equal(t, deleted, NewPostReplyTo(reply, moved.Id, moved))
	})

	t.Run("hides a burn-on-read message", func(t *testing.T) {
		burn := target.Clone()
		burn.Type = PostTypeBurnOnRead
		assert.Equal(t, deleted, NewPostReplyTo(reply, burn.Id, burn))
	})

	t.Run("leaves out a spoiler's text", func(t *testing.T) {
		spoiler := target.Clone()
		spoiler.AddProp(PostPropsSpoiler, true)
		replyTo := NewPostReplyTo(reply, spoiler.Id, spoiler)
		assert.True(t, replyTo.Spoiler)
		assert.Empty(t, replyTo.Message)
		assert.Equal(t, 2, replyTo.FileCount)
	})
}

func TestPostIsSpoiler(t *testing.T) {
	for name, tc := range map[string]struct {
		value any
		want  bool
	}{
		"unset":         {nil, false},
		"true":          {true, true},
		"false":         {false, false},
		"string true":   {"true", true},
		"another value": {"yes", false},
	} {
		t.Run(name, func(t *testing.T) {
			post := &Post{}
			if tc.value != nil {
				post.AddProp(PostPropsSpoiler, tc.value)
			}
			assert.Equal(t, tc.want, post.IsSpoiler())
		})
	}
}

func TestPostMetadataCopy_ReplyTo(t *testing.T) {
	original := &PostMetadata{ReplyTo: &PostReplyTo{PostId: NewId(), Message: "quoted"}}

	copied := original.Copy()
	require.Equal(t, original.ReplyTo, copied.ReplyTo)

	copied.ReplyTo.Message = "changed"
	require.Equal(t, "quoted", original.ReplyTo.Message, "the copy shares nothing with the original")
}
