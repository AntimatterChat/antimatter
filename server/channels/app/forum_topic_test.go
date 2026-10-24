// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Antimatter forums: a topic's accepted answer (Q&A) and status (Ideas), who may set them, the system replies they
// leave, the answer taken back when its reply goes, and voters following ideas.

func forumSession(user *model.User) model.Session {
	return model.Session{Id: model.NewId(), UserId: user.Id, Roles: model.SystemUserRoleId}
}

func requireForbidden(t *testing.T, appErr *model.AppError, id string, msgAndArgs ...any) {
	t.Helper()
	require.NotNil(t, appErr, msgAndArgs...)
	assert.Equal(t, id, appErr.Id, msgAndArgs...)
	assert.Equal(t, http.StatusForbidden, appErr.StatusCode, msgAndArgs...)
}

// moderator gives a new member of the team and the forum who is the forum's channel admin.
func (f *forumFixture) moderator(t *testing.T) *model.User {
	t.Helper()
	user := f.memberOfForum(t)
	_, appErr := f.th.App.UpdateChannelMemberSchemeRoles(f.th.Context, f.forum.Id, user.Id, false, true, true)
	require.Nil(t, appErr)
	return user
}

func (f *forumFixture) memberOfForum(t *testing.T) *model.User {
	t.Helper()
	user := f.th.CreateUser(t)
	f.th.LinkUserToTeam(t, user, f.th.BasicTeam)
	f.th.AddUserToChannel(t, user, f.forum)
	return user
}

// lastReply gives the latest reply of a topic.
func (f *forumFixture) lastReply(t *testing.T, rootID string) *model.Post {
	t.Helper()
	thread, appErr := f.th.App.GetPostThread(f.th.Context, rootID, model.GetPostsOptions{}, f.member.Id)
	require.Nil(t, appErr)
	var last *model.Post
	for _, post := range thread.Posts {
		if post.Id != rootID && (last == nil || post.CreateAt > last.CreateAt) {
			last = post
		}
	}
	require.NotNil(t, last)
	return last
}

func storedProp(t *testing.T, th *TestHelper, postID, key string) any {
	t.Helper()
	post, err := th.App.Srv().Store().Post().GetSingle(th.Context, postID, false)
	require.NoError(t, err)
	return post.GetProp(key)
}

// RBAC regression: on a Q&A topic, the asker and the forum's moderators choose the answer; nobody else.
func TestForumTopicAcceptedAnswer(t *testing.T) {
	mainHelper.Parallel(t)
	f := setupForum(t, model.ChannelPresetForumQA)
	th := f.th
	asker := f.member
	answerer := th.BasicUser2
	answer := f.post(t, answerer.Id, f.topic.Id, "")
	accept := func(id string) *model.ForumTopicPatch { return &model.ForumTopicPatch{AcceptedAnswerId: &id} }

	t.Run("a member can't choose the answer of someone else's question", func(t *testing.T) {
		_, appErr := th.App.PatchForumTopic(th.Context, forumSession(answerer), f.topic.Id, accept(answer.Id))
		requireForbidden(t, appErr, "app.topic.accept.not_allowed.app_error", "not even the answer's author")
		_, appErr = th.App.PatchForumTopic(th.Context, forumSession(f.memberOfForum(t)), f.topic.Id, accept(answer.Id))
		requireForbidden(t, appErr, "app.topic.accept.not_allowed.app_error")
		assert.Nil(t, storedProp(t, th, f.topic.Id, model.PostPropsAcceptedAnswer))
	})

	t.Run("the asker accepts a reply", func(t *testing.T) {
		before, err := th.App.Srv().Store().Post().GetSingle(th.Context, f.topic.Id, false)
		require.NoError(t, err)
		updated, appErr := th.App.PatchForumTopic(th.Context, forumSession(asker), f.topic.Id, accept(answer.Id))
		require.Nil(t, appErr)
		assert.Equal(t, answer.Id, updated.GetProp(model.PostPropsAcceptedAnswer))
		assert.Equal(t, before.EditAt, updated.EditAt, "no Edited mark")
		assert.Greater(t, updated.UpdateAt, before.UpdateAt)
		assert.Equal(t, answer.Id, storedProp(t, th, f.topic.Id, model.PostPropsAcceptedAnswer))

		note := f.lastReply(t, f.topic.Id)
		assert.Equal(t, model.PostTypeTopicAnswer, note.Type)
		assert.Equal(t, answer.Id, note.GetProp("answer_id"))
		assert.Contains(t, note.Message, asker.Username)
		assert.Contains(t, note.Message, answerer.Username)

		_, appErr = th.App.PatchForumTopic(th.Context, forumSession(asker), f.topic.Id, accept(answer.Id))
		require.Nil(t, appErr)
		assert.Equal(t, note.Id, f.lastReply(t, f.topic.Id).Id, "the same answer again changes nothing")
	})

	t.Run("a moderator takes it back and accepts another", func(t *testing.T) {
		moderator := f.moderator(t)
		_, appErr := th.App.PatchForumTopic(th.Context, forumSession(moderator), f.topic.Id, accept(""))
		require.Nil(t, appErr)
		assert.Nil(t, storedProp(t, th, f.topic.Id, model.PostPropsAcceptedAnswer))
		assert.Equal(t, model.PostTypeTopicAnswer, f.lastReply(t, f.topic.Id).Type)

		other := f.post(t, moderator.Id, f.topic.Id, "")
		_, appErr = th.App.PatchForumTopic(th.Context, forumSession(moderator), f.topic.Id, accept(other.Id))
		require.Nil(t, appErr)
		assert.Equal(t, other.Id, storedProp(t, th, f.topic.Id, model.PostPropsAcceptedAnswer))
	})

	t.Run("an asker who may no longer post there can't", func(t *testing.T) {
		topic := f.post(t, asker.Id, "", "Another question")
		reply := f.post(t, answerer.Id, topic.Id, "")
		_, appErr := th.App.SaveChannelOverride(th.Context, f.forum, &model.ChannelOverride{TargetType: model.ChannelOverrideTargetUser, TargetId: asker.Id, Deny: []string{model.PermissionCreatePost.Id}})
		require.Nil(t, appErr)
		defer func() {
			_, appErr = th.App.SaveChannelOverride(th.Context, f.forum, &model.ChannelOverride{TargetType: model.ChannelOverrideTargetUser, TargetId: asker.Id})
			require.Nil(t, appErr)
		}()
		_, appErr = th.App.PatchForumTopic(th.Context, forumSession(asker), topic.Id, accept(reply.Id))
		requireForbidden(t, appErr, "app.topic.accept.not_allowed.app_error")
	})

	t.Run("only a live reply of the topic can be the answer", func(t *testing.T) {
		elsewhere := f.post(t, asker.Id, "", "Elsewhere")
		otherReply := f.post(t, answerer.Id, elsewhere.Id, "")
		for name, id := range map[string]string{
			"another topic's reply": otherReply.Id,
			"the question itself":   f.topic.Id,
			"a system reply":        f.lastReply(t, f.topic.Id).Id,
			"no such post":          model.NewId(),
		} {
			_, appErr := th.App.PatchForumTopic(th.Context, forumSession(asker), f.topic.Id, accept(id))
			requireBadRequest(t, appErr, "app.topic.accept.not_a_reply.app_error", name)
		}
	})

	t.Run("deleting the accepted reply takes the answer back", func(t *testing.T) {
		topic := f.post(t, asker.Id, "", "Deleted answer")
		reply := f.post(t, answerer.Id, topic.Id, "")
		_, appErr := th.App.PatchForumTopic(th.Context, forumSession(asker), topic.Id, accept(reply.Id))
		require.Nil(t, appErr)
		_, appErr = th.App.DeletePost(th.Context, reply.Id, answerer.Id)
		require.Nil(t, appErr)
		assert.Nil(t, storedProp(t, th, topic.Id, model.PostPropsAcceptedAnswer))
	})

	t.Run("statuses are for Ideas forums", func(t *testing.T) {
		status := model.TopicStatusPlanned
		_, appErr := th.App.PatchForumTopic(th.Context, forumSession(f.moderator(t)), f.topic.Id, &model.ForumTopicPatch{Status: &status})
		requireBadRequest(t, appErr, "app.topic.mode.app_error")
	})

	t.Run("not a topic", func(t *testing.T) {
		_, appErr := th.App.PatchForumTopic(th.Context, forumSession(asker), answer.Id, accept(""))
		requireBadRequest(t, appErr, "app.topic.not_a_topic.app_error", "a reply")
		post, _, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: asker.Id, ChannelId: th.BasicChannel.Id, Message: "Text"}, th.BasicChannel, model.CreatePostFlags{})
		require.Nil(t, appErr)
		_, appErr = th.App.PatchForumTopic(th.Context, forumSession(asker), post.Id, accept(""))
		requireBadRequest(t, appErr, "app.topic.not_a_topic.app_error", "a text channel's message")
	})
}

// RBAC regression: only the forum's moderators set an idea's status.
func TestForumTopicStatus(t *testing.T) {
	mainHelper.Parallel(t)
	f := setupForum(t, model.ChannelPresetForumIdeas)
	th := f.th
	set := func(s string) *model.ForumTopicPatch { return &model.ForumTopicPatch{Status: &s} }

	t.Run("a member can't, not even the idea's author", func(t *testing.T) {
		_, appErr := th.App.PatchForumTopic(th.Context, forumSession(f.member), f.topic.Id, set(model.TopicStatusDone))
		requireForbidden(t, appErr, "app.topic.status.not_allowed.app_error")
		_, appErr = th.App.PatchForumTopic(th.Context, forumSession(th.BasicUser2), f.topic.Id, set(model.TopicStatusDone))
		requireForbidden(t, appErr, "app.topic.status.not_allowed.app_error")
		assert.Nil(t, storedProp(t, th, f.topic.Id, model.PostPropsTopicStatus))
	})

	t.Run("a moderator sets it", func(t *testing.T) {
		moderator := f.moderator(t)
		updated, appErr := th.App.PatchForumTopic(th.Context, forumSession(moderator), f.topic.Id, set(model.TopicStatusPlanned))
		require.Nil(t, appErr)
		assert.Equal(t, model.TopicStatusPlanned, updated.GetProp(model.PostPropsTopicStatus))
		assert.Equal(t, moderator.Id, updated.GetProp(model.PostPropsTopicStatusBy))
		assert.NotNil(t, updated.GetProp(model.PostPropsTopicStatusAt))

		note := f.lastReply(t, f.topic.Id)
		assert.Equal(t, model.PostTypeTopicStatus, note.Type)
		assert.Equal(t, model.TopicStatusPlanned, note.GetProp("status"))
		assert.Contains(t, note.Message, "Planned")

		_, appErr = th.App.PatchForumTopic(th.Context, forumSession(moderator), f.topic.Id, set("closed"))
		requireBadRequest(t, appErr, "app.topic.status.invalid.app_error")

		answer := ""
		_, appErr = th.App.PatchForumTopic(th.Context, forumSession(moderator), f.topic.Id, &model.ForumTopicPatch{AcceptedAnswerId: &answer})
		requireBadRequest(t, appErr, "app.topic.mode.app_error", "answers are for Q&A forums")
	})

	t.Run("open again: a topic without a status is open", func(t *testing.T) {
		moderator := f.moderator(t)
		topic := f.post(t, f.member.Id, "", "New idea")
		_, appErr := th.App.PatchForumTopic(th.Context, forumSession(moderator), topic.Id, set(model.TopicStatusOpen))
		require.Nil(t, appErr)
		assert.Nil(t, storedProp(t, th, topic.Id, model.PostPropsTopicStatus), "nothing changed, nothing written")
	})
}

func TestIdeasVotersFollow(t *testing.T) {
	mainHelper.Parallel(t)
	f := setupForum(t, model.ChannelPresetForumIdeas)
	th := f.th
	following := func(userID, postID string) bool { return isFollowing(th, userID, postID) }
	react := func(t *testing.T, userID, postID, emoji string) *model.Reaction {
		t.Helper()
		reaction, appErr := th.App.SaveReactionForPost(th.Context, &model.Reaction{UserId: userID, PostId: postID, EmojiName: emoji})
		require.Nil(t, appErr)
		return reaction
	}

	voter := th.BasicUser2
	vote := react(t, voter.Id, f.topic.Id, "+1")
	assert.True(t, following(voter.Id, f.topic.Id), "a vote follows the idea")

	require.Nil(t, th.App.DeleteReactionForPost(th.Context, vote))
	assert.True(t, following(voter.Id, f.topic.Id), "taking the vote back doesn't unfollow")

	other := f.memberOfForum(t)
	react(t, other.Id, f.topic.Id, "heart")
	assert.False(t, following(other.Id, f.topic.Id), "a heart isn't a vote")
	reply := f.post(t, f.member.Id, f.topic.Id, "")
	react(t, other.Id, reply.Id, "thumbsup")
	assert.False(t, following(other.Id, reply.Id), "nor is a reply an idea")

	agora := createTypedChannel(t, th, model.ChannelPresetForumAgora)
	th.AddUserToChannel(t, f.member, agora)
	th.AddUserToChannel(t, other, agora)
	topic, _, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: f.member.Id, ChannelId: agora.Id, Message: "Discussion", Props: model.StringInterface{model.PostPropsThreadTitle: "Talk"}}, agora, model.CreatePostFlags{})
	require.Nil(t, appErr)
	react(t, other.Id, topic.Id, "+1")
	assert.False(t, following(other.Id, topic.Id), "only in Ideas forums")
}

func TestGetForumTopicsModes(t *testing.T) {
	mainHelper.Parallel(t)
	f := setupForum(t, model.ChannelPresetForumAgora)
	th := f.th
	yes := true

	_, appErr := th.App.GetForumTopics(th.Context, f.forum, model.ForumTopicsOptions{UserId: f.member.Id, Sort: model.ForumTopicSortUnanswered})
	requireBadRequest(t, appErr, "app.topic.mode.app_error")
	_, appErr = th.App.GetForumTopics(th.Context, f.forum, model.ForumTopicsOptions{UserId: f.member.Id, Answered: &yes})
	requireBadRequest(t, appErr, "app.topic.mode.app_error")
	_, appErr = th.App.GetForumTopics(th.Context, f.forum, model.ForumTopicsOptions{UserId: f.member.Id, Status: model.TopicStatusDone})
	requireBadRequest(t, appErr, "app.topic.mode.app_error")
	_, appErr = th.App.GetForumTopics(th.Context, th.BasicChannel, model.ForumTopicsOptions{UserId: f.member.Id})
	requireBadRequest(t, appErr, "app.topic.not_a_forum.app_error")

	// A Q&A forum's answer stays on the post when it becomes a discussion, but isn't given.
	stored := f.topic.Clone()
	stored.AddProp(model.PostPropsAcceptedAnswer, model.NewId())
	_, err := th.App.Srv().Store().Post().Overwrite(th.Context, stored)
	require.NoError(t, err)
	list, appErr := th.App.GetForumTopics(th.Context, f.forum, model.ForumTopicsOptions{UserId: f.member.Id})
	require.Nil(t, appErr)
	require.Len(t, list.Topics, 1)
	assert.Empty(t, list.Topics[0].AcceptedAnswerId)
	assert.Empty(t, list.Topics[0].Status)
	assert.Equal(t, "First topic", list.Topics[0].Post.GetProp(model.PostPropsThreadTitle))
}

func isFollowing(th *TestHelper, userID, postID string) bool {
	membership, appErr := th.App.GetThreadMembershipForUser(userID, postID)
	return appErr == nil && membership != nil && membership.Following
}

// A moderator's status change and the author's edit of the message, one after the other from what each read
// before, both stay: the topic patch writes only its props, and an edit keeps them.
func TestForumTopicPatchAndEditBothSurvive(t *testing.T) {
	mainHelper.Parallel(t)
	f := setupForum(t, model.ChannelPresetForumIdeas)
	th := f.th
	moderator := f.moderator(t)
	planned := model.TopicStatusPlanned
	th.Context = th.Context.WithSession(&model.Session{UserId: f.member.Id, Id: model.NewId()}).(*request.Context)

	// The author's client read the topic before the status changed, and saves an edit after it.
	readByAuthor := f.topic.Clone()
	_, appErr := th.App.PatchForumTopic(th.Context, forumSession(moderator), f.topic.Id, &model.ForumTopicPatch{Status: &planned})
	require.Nil(t, appErr)
	readByAuthor.Message = "Edited after the status changed"
	_, _, appErr = th.App.UpdatePost(th.Context, readByAuthor, nil)
	require.Nil(t, appErr)

	// The moderator acts on what they read before the next edit.
	edit := readByAuthor.Clone()
	edit.Message = "Edited again"
	_, _, appErr = th.App.UpdatePost(th.Context, edit, nil)
	require.Nil(t, appErr)
	done := model.TopicStatusDone
	_, appErr = th.App.PatchForumTopic(th.Context, forumSession(moderator), f.topic.Id, &model.ForumTopicPatch{Status: &done})
	require.Nil(t, appErr)

	stored, err := th.App.Srv().Store().Post().GetSingle(th.Context, f.topic.Id, false)
	require.NoError(t, err)
	assert.Equal(t, "Edited again", stored.Message, "the edits survive the status changes")
	assert.Equal(t, model.TopicStatusDone, stored.GetProp(model.PostPropsTopicStatus), "and the status the edits")
	assert.Equal(t, "First topic", stored.GetProp(model.PostPropsThreadTitle))
}
