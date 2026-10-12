// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Antimatter's channel export queries (ComplianceStore.ChannelExportCount and
// ChannelExportPosts).

type channelExportFixture struct {
	team    *model.Team
	user    *model.User
	bot     *model.User
	channel *model.Channel
	other   *model.Channel
	start   int64
}

func newChannelExportFixture(t *testing.T, rctx request.CTX, ss store.Store) *channelExportFixture {
	f := &channelExportFixture{start: model.GetMillis()}
	var err error
	f.team, err = ss.Team().Save(&model.Team{
		DisplayName: "DisplayName",
		Name:        model.NewRandomTeamName(),
		Email:       MakeEmail(),
		Type:        model.TeamOpen,
	})
	require.NoError(t, err)
	f.user, err = ss.User().Save(rctx, &model.User{Email: MakeEmail(), Username: model.NewUsername()})
	require.NoError(t, err)
	f.bot, err = ss.User().Save(rctx, &model.User{Email: MakeEmail(), Username: model.NewUsername(), IsBot: true})
	require.NoError(t, err)
	_, err = ss.Bot().Save(&model.Bot{UserId: f.bot.Id, Username: f.bot.Username, OwnerId: f.user.Id})
	require.NoError(t, err)

	f.channel, err = ss.Channel().Save(rctx, &model.Channel{TeamId: f.team.Id, Name: model.NewId(), DisplayName: "Exported", Type: model.ChannelTypeOpen}, -1)
	require.NoError(t, err)
	f.other, err = ss.Channel().Save(rctx, &model.Channel{TeamId: f.team.Id, Name: model.NewId(), DisplayName: "Other", Type: model.ChannelTypeOpen}, -1)
	require.NoError(t, err)
	return f
}

// save saves a post of the user (or of the given one) in the channel, created at start + offset.
func (f *channelExportFixture) save(t *testing.T, rctx request.CTX, ss store.Store, post *model.Post, offset int64) *model.Post {
	if post.ChannelId == "" {
		post.ChannelId = f.channel.Id
	}
	if post.UserId == "" {
		post.UserId = f.user.Id
	}
	if post.Message == "" {
		post.Message = model.NewId()
	}
	post.CreateAt = f.start + offset
	post.UpdateAt = post.CreateAt
	saved, err := ss.Post().Save(rctx, post)
	require.NoError(t, err)
	return saved
}

func channelExportIDs(posts []*model.ChannelExportPost) []string {
	ids := make([]string, 0, len(posts))
	for _, p := range posts {
		ids = append(ids, p.PostId)
	}
	return ids
}

func testChannelExport(t *testing.T, rctx request.CTX, ss store.Store) {
	defer cleanupStoreState(t, rctx, ss)
	f := newChannelExportFixture(t, rctx, ss)

	before := f.save(t, rctx, ss, &model.Post{}, 0)
	first := f.save(t, rctx, ss, &model.Post{Message: "first", FileIds: model.StringArray{"file1"}}, 10)
	f.save(t, rctx, ss, &model.Post{Type: model.PostTypeJoinChannel}, 12)
	burnOnRead := &model.Post{Type: model.PostTypeBurnOnRead}
	burnOnRead.AddProp(model.PostPropsExpireAt, model.GetMillis()+3600000)
	f.save(t, rctx, ss, burnOnRead, 14)
	deleted := f.save(t, rctx, ss, &model.Post{}, 16)
	require.NoError(t, ss.Post().Delete(rctx, deleted.Id, f.start+17, f.user.Id))
	reply := f.save(t, rctx, ss, &model.Post{UserId: f.bot.Id, RootId: first.Id, Message: "beep"}, 20)

	webhook := &model.Post{Message: "built"}
	webhook.AddProp(model.PostPropsFromWebhook, "true")
	webhook.AddProp(model.PostPropsOverrideUsername, "CI")
	webhook.AddProp(model.PostPropsAttachments, []*model.MessageAttachment{{Title: "#42", Text: "passed"}})
	webhook = f.save(t, rctx, ss, webhook, 30)

	// Attachments too large to read are left out (the post isn't).
	large := &model.Post{}
	large.AddProp(model.PostPropsAttachments, []*model.MessageAttachment{{Text: strings.Repeat("x", model.ChannelExportMaxAttachmentsSize)}})
	large = f.save(t, rctx, ss, large, 30)

	after := f.save(t, rctx, ss, &model.Post{}, 40)
	f.save(t, rctx, ss, &model.Post{ChannelId: f.other.Id}, 20)

	filter := model.ChannelExportFilter{ChannelId: f.channel.Id, SinceCreateAt: f.start + 10, UntilCreateAt: f.start + 30}

	t.Run("counts the messages the channel shows in the range", func(t *testing.T) {
		count, err := ss.Compliance().ChannelExportCount(rctx, filter, 0)
		require.NoError(t, err)
		assert.Equal(t, 4, count)

		count, err = ss.Compliance().ChannelExportCount(rctx, filter, 2)
		require.NoError(t, err)
		assert.Equal(t, 2, count, "counting stops at the limit")

		count, err = ss.Compliance().ChannelExportCount(rctx, model.ChannelExportFilter{ChannelId: f.channel.Id}, 0)
		require.NoError(t, err)
		assert.Equal(t, 6, count)
	})

	t.Run("reads them by creation time and id", func(t *testing.T) {
		posts, err := ss.Compliance().ChannelExportPosts(rctx, filter, model.ChannelExportCursor{}, 100)
		require.NoError(t, err)
		lastTwo := []string{webhook.Id, large.Id}
		if large.Id < webhook.Id {
			lastTwo = []string{large.Id, webhook.Id}
		}
		assert.Equal(t, append([]string{first.Id, reply.Id}, lastTwo...), channelExportIDs(posts))

		byID := map[string]*model.ChannelExportPost{}
		for _, p := range posts {
			byID[p.PostId] = p
		}

		p := byID[first.Id]
		assert.Equal(t, f.start+10, p.PostCreateAt)
		assert.Equal(t, "first", p.PostMessage)
		assert.Equal(t, model.StringArray{"file1"}, p.PostFileIds)
		assert.Equal(t, f.user.Id, p.UserId)
		assert.Equal(t, f.user.Username, p.Username)
		assert.Equal(t, f.user.Email, p.UserEmail)
		assert.False(t, p.IsBot)
		assert.Empty(t, p.PostRootId)
		assert.Empty(t, p.FromWebhook)
		assert.Empty(t, p.Attachments)

		p = byID[reply.Id]
		assert.True(t, p.IsBot)
		assert.Equal(t, first.Id, p.PostRootId)

		p = byID[webhook.Id]
		assert.Equal(t, "true", p.FromWebhook)
		assert.Equal(t, "CI", p.OverrideUsername)
		var attachments []*model.MessageAttachment
		require.NoError(t, json.Unmarshal([]byte(p.Attachments), &attachments))
		require.Len(t, attachments, 1)
		assert.Equal(t, "passed", attachments[0].Text)

		assert.Empty(t, byID[large.Id].Attachments)

		// After a cursor.
		posts, err = ss.Compliance().ChannelExportPosts(rctx, filter, model.ChannelExportCursor{AfterCreateAt: first.CreateAt, AfterId: first.Id}, 1)
		require.NoError(t, err)
		assert.Equal(t, []string{reply.Id}, channelExportIDs(posts))

		// Open bounds: all the channel's messages, and none of the other channel's.
		posts, err = ss.Compliance().ChannelExportPosts(rctx, model.ChannelExportFilter{ChannelId: f.channel.Id}, model.ChannelExportCursor{}, 100)
		require.NoError(t, err)
		ids := channelExportIDs(posts)
		assert.Len(t, ids, 6)
		assert.Equal(t, before.Id, ids[0])
		assert.Equal(t, after.Id, ids[5])
	})
}

// testChannelExportChangesBetweenPages reads a channel a page at a time while
// its messages are edited and replied to: each is read once, in its current
// version.
func testChannelExportChangesBetweenPages(t *testing.T, rctx request.CTX, ss store.Store) {
	defer cleanupStoreState(t, rctx, ss)
	f := newChannelExportFixture(t, rctx, ss)

	root := f.save(t, rctx, ss, &model.Post{Message: "root"}, 0)
	var posts []*model.Post
	for i := int64(1); i <= 5; i++ {
		posts = append(posts, f.save(t, rctx, ss, &model.Post{}, i*10))
	}
	filter := model.ChannelExportFilter{ChannelId: f.channel.Id, UntilCreateAt: f.start + 1000}

	edit := func(post *model.Post, message string) {
		old, err := ss.Post().GetSingle(rctx, post.Id, false)
		require.NoError(t, err)
		edited := old.Clone()
		edited.Message = message
		edited.EditAt = model.GetMillis()
		_, err = ss.Post().Update(rctx, edited, old)
		require.NoError(t, err)
	}

	read := map[string]int{}
	messages := map[string]string{}
	var cursor model.ChannelExportCursor
	for page := 0; ; page++ {
		got, err := ss.Compliance().ChannelExportPosts(rctx, filter, cursor, 2)
		require.NoError(t, err)
		for _, p := range got {
			read[p.PostId]++
			messages[p.PostId] = p.PostMessage
		}
		if len(got) < 2 {
			break
		}
		cursor = model.ChannelExportCursor{AfterCreateAt: got[len(got)-1].PostCreateAt, AfterId: got[len(got)-1].PostId}

		if page == 0 {
			// Between the pages: the root, already read, is edited and
			// replied to (which updates it); a message not read yet is edited.
			edit(root, "root, edited")
			f.save(t, rctx, ss, &model.Post{RootId: root.Id, Message: "a reply"}, 100)
			edit(posts[3], "edited before it's read")
		}
	}

	for id, n := range read {
		assert.Equal(t, 1, n, "post %s read %d times", id, n)
	}
	assert.Len(t, read, 7, "the root, its 5 followers and the reply")
	assert.Equal(t, "root", messages[root.Id], "the root was read before its edit")
	assert.Equal(t, "edited before it's read", messages[posts[3].Id])
}
