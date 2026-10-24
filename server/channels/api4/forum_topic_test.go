// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// Antimatter forums over REST: GET /channels/{id}/topics (who may list, and that q stays in the channel) and
// PATCH /posts/{id}/topic (who may accept an answer or set a status).

func createForum(t *testing.T, th *TestHelper, preset, privacy string) *model.Channel {
	t.Helper()
	channel, resp, err := th.SystemAdminClient.CreateChannel(context.Background(), &model.Channel{
		DisplayName: "Forum", Name: "forum-" + model.NewId(), Type: model.ChannelType(privacy), TeamId: th.BasicTeam.Id,
		Props: map[string]any{model.ChannelPropPreset: preset},
	})
	require.NoError(t, err)
	CheckCreatedStatus(t, resp)
	return channel
}

func topicIDsOf(list *model.ForumTopicList) []string {
	ids := []string{}
	for _, topic := range list.Topics {
		ids = append(ids, topic.Post.Id)
	}
	return ids
}

func TestGetForumTopicsAPI(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()

	forum := createForum(t, th, model.ChannelPresetForumQA, string(model.ChannelTypeOpen))
	th.AddUserToChannel(t, th.BasicUser, forum)
	th.AddUserToChannel(t, th.BasicUser2, forum)
	question, _, err := th.Client.CreatePost(ctx, titled(forum.Id, "", "Beam alignment", "How do I align the beam?"))
	require.NoError(t, err)
	older, _, err := th.Client.CreatePost(ctx, titled(forum.Id, "", "Cryostat noise", "It hums"))
	require.NoError(t, err)
	client2 := th.CreateClient()
	th.LoginBasic2WithClient(t, client2)
	_, _, err = client2.CreatePost(ctx, titled(forum.Id, question.Id, "", "Use the laser"))
	require.NoError(t, err)
	_, _, err = th.Client.SaveReaction(ctx, &model.Reaction{UserId: th.BasicUser.Id, PostId: older.Id, EmojiName: "+1"})
	require.NoError(t, err)
	_, _, err = client2.SaveReaction(ctx, &model.Reaction{UserId: th.BasicUser2.Id, PostId: older.Id, EmojiName: "+1"})
	require.NoError(t, err)

	t.Run("a member lists them", func(t *testing.T) {
		list, resp, err := th.Client.GetForumTopics(ctx, forum.Id, url.Values{"replies": {"3"}})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		assert.Equal(t, []string{question.Id, older.Id}, topicIDsOf(list), "the latest activity first")
		require.Len(t, list.Topics[0].Replies, 1)
		assert.Equal(t, "Use the laser", list.Topics[0].Replies[0].Message)
		assert.Equal(t, "Beam alignment", list.Topics[0].Post.GetProp(model.PostPropsThreadTitle))

		list, _, err = th.Client.GetForumTopics(ctx, forum.Id, url.Values{"sort": {"votes"}})
		require.NoError(t, err)
		assert.Equal(t, []string{older.Id, question.Id}, topicIDsOf(list))
		assert.Equal(t, int64(1), list.Topics[0].Votes, "the author's own 👍 doesn't count")
		require.NotNil(t, list.Topics[0].Post.Metadata, "the posts come with their metadata")
		assert.Len(t, list.Topics[0].Post.Metadata.Reactions, 2)

		list, _, err = th.Client.GetForumTopics(ctx, forum.Id, url.Values{"sort": {"unanswered"}, "answered": {"false"}, "per_page": {"1"}})
		require.NoError(t, err)
		require.Len(t, list.Topics, 1)
		assert.NotEmpty(t, list.NextCursor)
		next, _, err := th.Client.GetForumTopics(ctx, forum.Id, url.Values{"sort": {"unanswered"}, "answered": {"false"}, "per_page": {"1"}, "before": {list.NextCursor}})
		require.NoError(t, err)
		require.Len(t, next.Topics, 1)
		assert.NotEqual(t, list.Topics[0].Post.Id, next.Topics[0].Post.Id)
	})

	t.Run("bad parameters", func(t *testing.T) {
		for name, query := range map[string]url.Values{
			"sort":     {"sort": {"hot"}},
			"replies":  {"replies": {"9"}},
			"before":   {"before": {"not-a-cursor"}},
			"per_page": {"per_page": {"0"}},
			"answered": {"answered": {"maybe"}},
			"status":   {"status": {"closed"}},
			"mode":     {"status": {model.TopicStatusDone}},
		} {
			_, resp, err := th.Client.GetForumTopics(ctx, forum.Id, query)
			require.Error(t, err, name)
			CheckBadRequestStatus(t, resp)
		}
		_, resp, err := th.Client.GetForumTopics(ctx, th.BasicChannel.Id, nil)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		CheckErrorID(t, err, "app.topic.not_a_forum.app_error")
	})

	// Privacy regression: a private forum's topics are for its members only.
	t.Run("a private forum's topics aren't listed for a non-member", func(t *testing.T) {
		private := createForum(t, th, model.ChannelPresetForumAgora, string(model.ChannelTypePrivate))
		_, _, err := th.SystemAdminClient.CreatePost(ctx, titled(private.Id, "", "Secret plans", "Beam alignment, privately"))
		require.NoError(t, err)

		_, resp, err := th.Client.GetForumTopics(ctx, private.Id, nil)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		_, resp, err = th.Client.GetForumTopics(ctx, private.Id, url.Values{"q": {"beam"}})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		th.AddUserToChannel(t, th.BasicUser, private)
		list, _, err := th.Client.GetForumTopics(ctx, private.Id, nil)
		require.NoError(t, err)
		assert.Len(t, list.Topics, 1, "its members see them")
	})

	// Privacy regression: q searches the channel asked about, never another one.
	t.Run("q doesn't leak another channel's posts", func(t *testing.T) {
		other := createForum(t, th, model.ChannelPresetForumAgora, string(model.ChannelTypePrivate))
		hidden, _, err := th.SystemAdminClient.CreatePost(ctx, titled(other.Id, "", "Beam alignment", "Beam alignment, elsewhere"))
		require.NoError(t, err)

		list, _, err := th.Client.GetForumTopics(ctx, forum.Id, url.Values{"q": {"beam alignment"}})
		require.NoError(t, err)
		assert.Equal(t, []string{question.Id}, topicIDsOf(list))
		assert.NotContains(t, topicIDsOf(list), hidden.Id)

		list, _, err = th.Client.GetForumTopics(ctx, forum.Id, url.Values{"q": {"%"}})
		require.NoError(t, err)
		assert.Empty(t, list.Topics, "wildcards are taken literally")
	})
}

// RBAC regression: who may accept an answer (the asker, a moderator) or set a status (a moderator).
func TestPatchForumTopicAPI(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()
	client2 := th.CreateClient()
	th.LoginBasic2WithClient(t, client2)
	accept := func(id string) *model.ForumTopicPatch { return &model.ForumTopicPatch{AcceptedAnswerId: &id} }
	set := func(s string) *model.ForumTopicPatch { return &model.ForumTopicPatch{Status: &s} }

	t.Run("Q&A: a member can't accept on someone else's question; the asker and a moderator can", func(t *testing.T) {
		forum := createForum(t, th, model.ChannelPresetForumQA, string(model.ChannelTypeOpen))
		th.AddUserToChannel(t, th.BasicUser, forum)
		th.AddUserToChannel(t, th.BasicUser2, forum)
		question, _, err := th.Client.CreatePost(ctx, titled(forum.Id, "", "Question", "Why?"))
		require.NoError(t, err)
		answer, _, err := client2.CreatePost(ctx, titled(forum.Id, question.Id, "", "Because"))
		require.NoError(t, err)

		_, resp, err := client2.PatchForumTopic(ctx, question.Id, accept(answer.Id))
		require.Error(t, err, "the answer's author isn't the asker")
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "app.topic.accept.not_allowed.app_error")

		updated, resp, err := th.Client.PatchForumTopic(ctx, question.Id, accept(answer.Id))
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		assert.Equal(t, answer.Id, updated.GetProp(model.PostPropsAcceptedAnswer))

		updated, _, err = th.SystemAdminClient.PatchForumTopic(ctx, question.Id, accept(""))
		require.NoError(t, err, "a moderator")
		assert.Nil(t, updated.GetProp(model.PostPropsAcceptedAnswer))

		teamAdmin := th.CreateUser(t)
		th.LinkUserToTeam(t, teamAdmin, th.BasicTeam)
		th.UpdateUserToTeamAdmin(t, teamAdmin, th.BasicTeam)
		th.AddUserToChannel(t, teamAdmin, forum)
		adminClient := th.CreateClient()
		_, _, err = adminClient.Login(ctx, teamAdmin.Email, teamAdmin.Password)
		require.NoError(t, err)
		_, _, err = adminClient.PatchForumTopic(ctx, question.Id, accept(answer.Id))
		require.NoError(t, err, "the community's admin moderates its forums")
	})

	t.Run("Ideas: a non-moderator can't set a status", func(t *testing.T) {
		forum := createForum(t, th, model.ChannelPresetForumIdeas, string(model.ChannelTypeOpen))
		th.AddUserToChannel(t, th.BasicUser, forum)
		th.AddUserToChannel(t, th.BasicUser2, forum)
		idea, _, err := th.Client.CreatePost(ctx, titled(forum.Id, "", "Dark mode", "Please"))
		require.NoError(t, err)

		for _, client := range []*model.Client4{th.Client, client2} {
			_, resp, err := client.PatchForumTopic(ctx, idea.Id, set(model.TopicStatusPlanned))
			require.Error(t, err, "not the idea's author, nor anyone else")
			CheckForbiddenStatus(t, resp)
			CheckErrorID(t, err, "app.topic.status.not_allowed.app_error")
		}

		// Through the app, which drops the cached memberships: client2 was refused above, so its old roles are cached.
		_, appErr := th.App.UpdateChannelMemberSchemeRoles(th.Context, forum.Id, th.BasicUser2.Id, false, true, true)
		require.Nil(t, appErr)
		updated, _, err := client2.PatchForumTopic(ctx, idea.Id, set(model.TopicStatusPlanned))
		require.NoError(t, err, "the forum's admin")
		assert.Equal(t, model.TopicStatusPlanned, updated.GetProp(model.PostPropsTopicStatus))

		list, _, err := th.Client.GetForumTopics(ctx, forum.Id, url.Values{"status": {model.TopicStatusPlanned}})
		require.NoError(t, err)
		assert.Equal(t, []string{idea.Id}, topicIDsOf(list))
		assert.Equal(t, model.TopicStatusPlanned, list.Topics[0].Status)
	})

	t.Run("who can't read the forum gets 403", func(t *testing.T) {
		private := createForum(t, th, model.ChannelPresetForumQA, string(model.ChannelTypePrivate))
		question, _, err := th.SystemAdminClient.CreatePost(ctx, titled(private.Id, "", "Private", "Hidden"))
		require.NoError(t, err)
		_, resp, err := th.Client.PatchForumTopic(ctx, question.Id, accept(""))
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("an empty patch", func(t *testing.T) {
		_, resp, err := th.Client.PatchForumTopic(ctx, model.NewId(), &model.ForumTopicPatch{})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})
}
