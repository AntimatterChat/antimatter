// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForumTopicTitle(t *testing.T) {
	post := &Post{}
	assert.Equal(t, "", post.TopicTitle())
	post.AddProp(PostPropsThreadTitle, 42)
	assert.Equal(t, "", post.TopicTitle(), "a title that isn't text is none")
	post.AddProp(PostPropsThreadTitle, "  Beam alignment \n")
	assert.Equal(t, "Beam alignment", post.TopicTitle())

	require.NotNil(t, ValidateForumTopicTitle(""))
	assert.Equal(t, "app.post.forum.title_required.app_error", ValidateForumTopicTitle("   ").Id)
	assert.Nil(t, ValidateForumTopicTitle(strings.Repeat("é", ForumTopicTitleMaxRunes)), "runes, not bytes")
	appErr := ValidateForumTopicTitle(strings.Repeat("a", ForumTopicTitleMaxRunes+1))
	require.NotNil(t, appErr)
	assert.Equal(t, "app.post.forum.title_too_long.app_error", appErr.Id)
}

func TestServerOwnedPostProps(t *testing.T) {
	forged := &Post{Props: StringInterface{
		PostPropsAcceptedAnswer: NewId(), PostPropsTopicStatus: TopicStatusDone, PostPropsTopicStatusBy: NewId(),
		PostPropsTopicStatusAt: 1, PostPropsThreadTitle: "Kept",
	}}
	forged.RemoveServerOwnedProps()
	assert.Equal(t, StringInterface{PostPropsThreadTitle: "Kept"}, forged.GetProps(), "only the server-owned ones go")

	stored := &Post{Props: StringInterface{PostPropsAcceptedAnswer: "answer", PostPropsTopicStatus: TopicStatusPlanned}}
	edit := &Post{Props: StringInterface{PostPropsAcceptedAnswer: "forged", PostPropsTopicStatusBy: "forged", "other": "x"}}
	edit.KeepServerOwnedPropsFrom(stored)
	assert.Equal(t, StringInterface{PostPropsAcceptedAnswer: "answer", PostPropsTopicStatus: TopicStatusPlanned, "other": "x"}, edit.GetProps())

	edit.KeepServerOwnedPropsFrom(&Post{})
	assert.Equal(t, StringInterface{"other": "x"}, edit.GetProps(), "a stored post without them leaves none")
}

func TestForumTopicCursor(t *testing.T) {
	c := &ForumTopicCursor{OnTop: 1, Key1: 1712345678901, Key2: -3, Id: NewId()}
	parsed, err := ParseForumTopicCursor(c.String())
	require.NoError(t, err)
	assert.Equal(t, c, parsed)

	none, err := ParseForumTopicCursor("")
	require.NoError(t, err)
	assert.Nil(t, none)
	assert.Equal(t, "", (*ForumTopicCursor)(nil).String())

	for _, bad := range []string{"%%%", "MToyOjM", (&ForumTopicCursor{Id: "short"}).String(), (&ForumTopicCursor{OnTop: 2, Id: NewId()}).String()} {
		_, err := ParseForumTopicCursor(bad)
		assert.Error(t, err, bad)
	}
}

func TestTopicStatusesAndVotes(t *testing.T) {
	for _, s := range []string{TopicStatusOpen, TopicStatusPlanned, TopicStatusInProgress, TopicStatusDone, TopicStatusDeclined} {
		assert.True(t, IsValidTopicStatus(s), s)
	}
	assert.False(t, IsValidTopicStatus("closed"))
	assert.False(t, IsValidTopicStatus(""))

	assert.True(t, IsVoteEmoji("+1"))
	assert.True(t, IsVoteEmoji("thumbsup"))
	assert.False(t, IsVoteEmoji("-1"))
	assert.False(t, IsVoteEmoji("heart"))
}

func TestTopicSystemPostsAreValid(t *testing.T) {
	for _, postType := range []string{PostTypeTopicAnswer, PostTypeTopicStatus} {
		post := &Post{Id: NewId(), UserId: NewId(), ChannelId: NewId(), RootId: NewId(), CreateAt: 1, UpdateAt: 1, Type: postType, Message: "x"}
		assert.Nil(t, post.IsValid(PostMessageMaxRunesV2), postType)
		assert.True(t, post.IsSystemMessage(), postType)
		assert.LessOrEqual(t, len(postType), 26, "Posts.Type is a varchar(26)")
	}
}
