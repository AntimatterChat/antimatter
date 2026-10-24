// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Antimatter forums: the topic list's order, pages, votes and filters, straight from the store.

type forumTopicsFixture struct {
	t         *testing.T
	rctx      request.CTX
	ss        store.Store
	channelID string
	author    string
	base      int64
}

func (f *forumTopicsFixture) save(post *model.Post) *model.Post {
	f.t.Helper()
	if post.ChannelId == "" {
		post.ChannelId = f.channelID
	}
	if post.UserId == "" {
		post.UserId = f.author
	}
	saved, err := f.ss.Post().Save(f.rctx, post)
	require.NoError(f.t, err)
	return saved
}

func (f *forumTopicsFixture) topic(title, message string, at int64) *model.Post {
	f.t.Helper()
	return f.save(&model.Post{Message: message, CreateAt: f.base + at, Props: model.StringInterface{model.PostPropsThreadTitle: title}})
}

func (f *forumTopicsFixture) vote(postID, userID, emoji string) {
	f.t.Helper()
	_, err := f.ss.Reaction().Save(&model.Reaction{UserId: userID, PostId: postID, EmojiName: emoji, ChannelId: f.channelID})
	require.NoError(f.t, err)
}

func (f *forumTopicsFixture) list(opts model.ForumTopicsOptions) *model.ForumTopicList {
	f.t.Helper()
	opts.ChannelId = f.channelID
	list, err := f.ss.Post().GetForumTopics(f.rctx, opts)
	require.NoError(f.t, err)
	return list
}

func topicIDs(list *model.ForumTopicList) []string {
	ids := make([]string, 0, len(list.Topics))
	for _, topic := range list.Topics {
		ids = append(ids, topic.Post.Id)
	}
	return ids
}

func testPostStoreGetForumTopics(t *testing.T, rctx request.CTX, ss store.Store) {
	channel, err := ss.Channel().Save(rctx, &model.Channel{TeamId: model.NewId(), DisplayName: "Forum", Name: "forum-" + model.NewId(), Type: model.ChannelTypeOpen}, -1)
	require.NoError(t, err)
	f := &forumTopicsFixture{t: t, rctx: rctx, ss: ss, channelID: channel.Id, author: model.NewId(), base: model.GetMillis() - 100000}
	voter1, voter2 := model.NewId(), model.NewId()

	first := f.topic("Beam alignment", "How do I align the beam?", 1)
	second := f.topic("Cryostat noise", "The cryostat hums", 2)
	third := f.topic("Lab rota", "Who takes Fridays?", 3)

	// Not topics: a system message, a deleted root, and the replies.
	f.save(&model.Post{Type: model.PostTypeJoinChannel, Message: "joined", CreateAt: f.base + 4})
	deleted := f.topic("Gone", "Removed", 5)
	require.NoError(t, ss.Post().Delete(rctx, deleted.Id, model.GetMillis(), f.author))
	reply := f.save(&model.Post{RootId: first.Id, UserId: voter1, Message: "Use the laser", CreateAt: f.base + 10})
	f.save(&model.Post{RootId: first.Id, UserId: voter2, Message: "And the mirror", CreateAt: f.base + 11})
	f.save(&model.Post{RootId: first.Id, Type: model.PostTypeTopicAnswer, Message: "Answer marked", CreateAt: f.base + 12})

	// Votes: two people on the second topic (one under both names), the author's own doesn't count.
	f.vote(second.Id, voter1, "+1")
	f.vote(second.Id, voter1, "thumbsup")
	f.vote(second.Id, voter2, "+1")
	f.vote(second.Id, f.author, "+1")
	f.vote(third.Id, voter1, "thumbsup")
	f.vote(third.Id, voter2, "heart")

	// Another channel's topic with the same words never shows.
	f.save(&model.Post{ChannelId: model.NewId(), Message: "How do I align the beam?", CreateAt: f.base + 6, Props: model.StringInterface{model.PostPropsThreadTitle: "Beam alignment"}})

	t.Run("activity: the latest reply or the first post", func(t *testing.T) {
		list := f.list(model.ForumTopicsOptions{})
		assert.Equal(t, []string{first.Id, third.Id, second.Id}, topicIDs(list), "system messages, deleted roots and replies aren't topics")
		assert.Empty(t, list.NextCursor)
		got := list.Topics[0]
		assert.Equal(t, int64(3), got.ReplyCount, "the thread's count")
		assert.Equal(t, f.base+12, got.LastReplyAt)
		assert.Contains(t, got.ParticipantIds, voter1)
		assert.Equal(t, model.TopicStatusOpen, got.Status, "no status is open")
	})

	t.Run("newest", func(t *testing.T) {
		assert.Equal(t, []string{third.Id, second.Id, first.Id}, topicIDs(f.list(model.ForumTopicsOptions{Sort: model.ForumTopicSortNewest})))
	})

	t.Run("votes, without the author's own and once per person", func(t *testing.T) {
		list := f.list(model.ForumTopicsOptions{Sort: model.ForumTopicSortVotes})
		assert.Equal(t, []string{second.Id, third.Id, first.Id}, topicIDs(list))
		assert.Equal(t, int64(2), list.Topics[0].Votes)
		assert.Equal(t, int64(1), list.Topics[1].Votes, "a heart isn't a vote")
		assert.Equal(t, int64(0), list.Topics[2].Votes)
	})

	t.Run("pages follow the cursor without gaps or repeats", func(t *testing.T) {
		for _, sort := range []string{model.ForumTopicSortActivity, model.ForumTopicSortNewest, model.ForumTopicSortVotes} {
			all := topicIDs(f.list(model.ForumTopicsOptions{Sort: sort}))
			var paged []string
			var cursor *model.ForumTopicCursor
			for range 5 {
				page := f.list(model.ForumTopicsOptions{Sort: sort, PerPage: 1, Before: cursor})
				paged = append(paged, topicIDs(page)...)
				if page.NextCursor == "" {
					break
				}
				var err error
				cursor, err = model.ParseForumTopicCursor(page.NextCursor)
				require.NoError(t, err)
			}
			assert.Equal(t, all, paged, sort)
		}
	})

	t.Run("q matches the title or the first post, in this channel only", func(t *testing.T) {
		assert.Equal(t, []string{first.Id}, topicIDs(f.list(model.ForumTopicsOptions{Query: "beam ALIGN"})), "title, any case")
		assert.Equal(t, []string{second.Id}, topicIDs(f.list(model.ForumTopicsOptions{Query: "hums"})), "first post")
		assert.Empty(t, f.list(model.ForumTopicsOptions{Query: "laser"}).Topics, "replies aren't searched")
		assert.Empty(t, f.list(model.ForumTopicsOptions{Query: "%"}).Topics, "wildcards are literal")
	})

	t.Run("mine and following", func(t *testing.T) {
		mine := f.save(&model.Post{UserId: voter2, Message: "Mine", CreateAt: f.base + 7, Props: model.StringInterface{model.PostPropsThreadTitle: "Voter's topic"}})
		assert.Equal(t, []string{mine.Id}, topicIDs(f.list(model.ForumTopicsOptions{UserId: voter2, Mine: true})))

		_, err := ss.Thread().MaintainMembership(voter2, third.Id, store.ThreadMembershipOpts{Following: true, UpdateFollowing: true})
		require.NoError(t, err)
		assert.Equal(t, []string{third.Id}, topicIDs(f.list(model.ForumTopicsOptions{UserId: voter2, Following: true})))

		require.NoError(t, ss.Post().Delete(rctx, mine.Id, model.GetMillis(), voter2))
	})

	t.Run("answered, unanswered and status", func(t *testing.T) {
		answered := first.Clone()
		answered.AddProp(model.PostPropsAcceptedAnswer, reply.Id)
		_, err := ss.Post().Overwrite(rctx, answered)
		require.NoError(t, err)
		planned := third.Clone()
		planned.AddProp(model.PostPropsTopicStatus, model.TopicStatusPlanned)
		_, err = ss.Post().Overwrite(rctx, planned)
		require.NoError(t, err)

		yes, no := true, false
		assert.Equal(t, []string{first.Id}, topicIDs(f.list(model.ForumTopicsOptions{Answered: &yes})))
		assert.Equal(t, []string{third.Id, second.Id}, topicIDs(f.list(model.ForumTopicsOptions{Answered: &no})))
		assert.Equal(t, []string{third.Id, second.Id, first.Id}, topicIDs(f.list(model.ForumTopicsOptions{Sort: model.ForumTopicSortUnanswered})), "unanswered first")
		assert.Equal(t, reply.Id, f.list(model.ForumTopicsOptions{Answered: &yes}).Topics[0].AcceptedAnswerId)

		assert.Equal(t, []string{third.Id}, topicIDs(f.list(model.ForumTopicsOptions{Status: model.TopicStatusPlanned})))
		assert.Equal(t, []string{first.Id, second.Id}, topicIDs(f.list(model.ForumTopicsOptions{Status: model.TopicStatusOpen})), "no status is open")
		assert.Equal(t, []string{third.Id, first.Id, second.Id}, topicIDs(f.list(model.ForumTopicsOptions{Sort: model.ForumTopicSortStatus})), "planned before open")
	})

	t.Run("the latest replies, without system messages, oldest first", func(t *testing.T) {
		list := f.list(model.ForumTopicsOptions{Replies: 1})
		require.Len(t, list.Topics[0].Replies, 1)
		assert.Equal(t, "And the mirror", list.Topics[0].Replies[0].Message)
		list = f.list(model.ForumTopicsOptions{Replies: 5})
		require.Len(t, list.Topics[0].Replies, 2)
		assert.Equal(t, reply.Id, list.Topics[0].Replies[0].Id)
		assert.Empty(t, list.Topics[1].Replies)
	})

	t.Run("topics on top come first", func(t *testing.T) {
		_, err := ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: second.Id, ChannelId: f.channelID, CreatedBy: f.author, CreateAt: model.GetMillis()}, 10)
		require.NoError(t, err)
		list := f.list(model.ForumTopicsOptions{})
		assert.Equal(t, []string{second.Id, first.Id, third.Id}, topicIDs(list))
		assert.True(t, list.Topics[0].OnTop)
		assert.False(t, list.Topics[1].OnTop)

		page := f.list(model.ForumTopicsOptions{PerPage: 1})
		assert.Equal(t, []string{second.Id}, topicIDs(page))
		cursor, err := model.ParseForumTopicCursor(page.NextCursor)
		require.NoError(t, err)
		assert.Equal(t, []string{first.Id}, topicIDs(f.list(model.ForumTopicsOptions{PerPage: 1, Before: cursor})), "and only once")
	})
}
