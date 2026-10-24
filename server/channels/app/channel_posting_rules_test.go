// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// Antimatter channel types, rule 2: who starts conversations. In an announcement channel only its moderators post new
// messages; everyone replies. A regression either silences a channel or lets anyone shout in an announcement one,
// through any of the ways a message gets posted, so each is spelt out.

const (
	moderatorsOnly = "app.post.root_posts.moderators_only.app_error"
	rootPostDenied = "app.post.root_posts.denied.app_error"
)

func requireRootPostRefusal(t *testing.T, appErr *model.AppError, id string, msgAndArgs ...any) {
	t.Helper()
	require.NotNil(t, appErr, msgAndArgs...)
	assert.Equal(t, id, appErr.Id, msgAndArgs...)
	assert.Equal(t, http.StatusForbidden, appErr.StatusCode, msgAndArgs...)
}

type postingRulesFixture struct {
	th           *TestHelper
	team         *model.Team
	announcement *model.Channel
	chat         *model.Channel
	root         *model.Post // a root post in the announcement channel, to reply to
}

func setupPostingRules(t *testing.T) *postingRulesFixture {
	th := Setup(t).InitBasic(t)
	f := &postingRulesFixture{th: th, team: th.BasicTeam}
	f.announcement = createTypedChannel(t, th, model.ChannelPresetTextAnnouncement)
	f.chat = createTypedChannel(t, th, model.ChannelPresetTextChat)
	th.LinkUserToTeam(t, th.SystemAdminUser, f.team)
	th.AddUserToChannel(t, th.SystemAdminUser, f.announcement)
	root, _, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: th.SystemAdminUser.Id, ChannelId: f.announcement.Id, Message: "News"}, f.announcement, model.CreatePostFlags{})
	require.Nil(t, appErr, "a system admin starts conversations")
	f.root = root
	return f
}

// member gives a new member of the team and of the channels.
func (f *postingRulesFixture) member(t *testing.T, channels ...*model.Channel) *model.User {
	t.Helper()
	user := f.th.CreateUser(t)
	f.th.LinkUserToTeam(t, user, f.team)
	for _, c := range channels {
		f.th.AddUserToChannel(t, user, c)
	}
	return user
}

func (f *postingRulesFixture) post(userID string, channel *model.Channel, rootID string, flags model.CreatePostFlags) *model.AppError {
	_, _, appErr := f.th.App.CreatePost(f.th.Context, &model.Post{UserId: userID, ChannelId: channel.Id, RootId: rootID, Message: "hello " + model.NewId()}, channel, flags)
	return appErr
}

// role gives a community role holding the permissions, with the user in it.
func (f *postingRulesFixture) role(t *testing.T, userID string, permissions ...string) *model.Group {
	t.Helper()
	group, appErr := f.th.App.CreateGroup(&model.Group{
		Name: model.NewPointer("r" + model.NewId()), DisplayName: "Role", Source: model.GroupSourceCustom,
		RemoteId: model.NewPointer(model.NewId()), TeamId: f.team.Id,
	})
	require.Nil(t, appErr)
	require.True(t, group.IsRole())
	admin := model.Session{UserId: f.th.SystemAdminUser.Id, Roles: model.SystemAdminRoleId + " " + model.SystemUserRoleId}
	_, appErr = f.th.App.SetTeamRolePermissions(f.th.Context, admin, group, permissions)
	require.Nil(t, appErr)
	_, appErr = f.th.App.UpsertGroupMember(group.Id, userID)
	require.Nil(t, appErr)
	return group
}

func (f *postingRulesFixture) override(t *testing.T, channel *model.Channel, targetType, targetID string, allow, deny []string) {
	t.Helper()
	_, appErr := f.th.App.SaveChannelOverride(f.th.Context, channel, &model.ChannelOverride{TargetType: targetType, TargetId: targetID, Allow: allow, Deny: deny})
	require.Nil(t, appErr)
}

func TestRootPostsInAnnouncementChannels(t *testing.T) {
	mainHelper.Parallel(t)
	f := setupPostingRules(t)
	th := f.th

	t.Run("a member can't start a conversation, but replies", func(t *testing.T) {
		user := f.member(t, f.announcement)
		requireRootPostRefusal(t, f.post(user.Id, f.announcement, "", model.CreatePostFlags{}), moderatorsOnly)
		_, _, appErr := th.App.CreatePostAsUser(th.Context, &model.Post{UserId: user.Id, ChannelId: f.announcement.Id, Message: "as the API posts it"}, "", false)
		requireRootPostRefusal(t, appErr, moderatorsOnly, "the REST API's path")
		require.Nil(t, f.post(user.Id, f.announcement, f.root.Id, model.CreatePostFlags{}), "a reply needs create_post only")

		file := th.CreateFileInfo(t, user.Id, "", f.announcement.Id)
		_, _, appErr = th.App.CreatePost(th.Context, &model.Post{UserId: user.Id, ChannelId: f.announcement.Id, FileIds: []string{file.Id}}, f.announcement, model.CreatePostFlags{})
		requireRootPostRefusal(t, appErr, moderatorsOnly, "a file alone is a new message too")
	})

	t.Run("an incoming webhook made by a member", func(t *testing.T) {
		user := f.member(t, f.announcement)
		_, appErr := th.App.CreateWebhookPost(th.Context, user.Id, f.announcement, "From the hook", "", "", "", model.StringInterface{}, "", "", nil, false)
		requireRootPostRefusal(t, appErr, moderatorsOnly)
		_, appErr = th.App.CreateWebhookPost(th.Context, user.Id, f.announcement, "A reply from the hook", "", "", "", model.StringInterface{}, "", f.root.Id, nil, false)
		require.Nil(t, appErr, "a hook may reply")
	})

	t.Run("a slash command's answer in the channel", func(t *testing.T) {
		user := f.member(t, f.announcement)
		args := &model.CommandArgs{UserId: user.Id, ChannelId: f.announcement.Id, TeamId: f.team.Id, Command: "/weather"}
		inChannel := &model.CommandResponse{ResponseType: model.CommandResponseTypeInChannel, Text: "Sunny"}
		_, appErr := th.App.HandleCommandResponsePost(th.Context, &model.Command{}, args, inChannel, false)
		requireRootPostRefusal(t, appErr, moderatorsOnly, "an integration's answer")
		_, appErr = th.App.HandleCommandResponsePost(th.Context, &model.Command{}, args, inChannel, true)
		requireRootPostRefusal(t, appErr, moderatorsOnly, "a built-in command's answer")
		_, appErr = th.App.HandleCommandResponsePost(th.Context, &model.Command{}, args, &model.CommandResponse{ResponseType: model.CommandResponseTypeEphemeral, Text: "Only you"}, false)
		require.Nil(t, appErr, "an answer only they see isn't posting")
	})

	t.Run("a bot with its token, unless it moderates the channel or is allowed", func(t *testing.T) {
		bot := th.CreateBot(t)
		botUser, appErr := th.App.GetUser(th.Context, bot.UserId)
		require.Nil(t, appErr)
		th.LinkUserToTeam(t, botUser, f.team)
		th.AddUserToChannel(t, botUser, f.announcement)
		requireRootPostRefusal(t, f.post(bot.UserId, f.announcement, "", model.CreatePostFlags{}), moderatorsOnly)

		f.override(t, f.announcement, model.ChannelOverrideTargetUser, bot.UserId, []string{model.PermissionCreateRootPost.Id}, nil)
		require.Nil(t, f.post(bot.UserId, f.announcement, "", model.CreatePostFlags{}), "a news bot allowed by an override")
		f.override(t, f.announcement, model.ChannelOverrideTargetUser, bot.UserId, nil, nil)

		_, appErr = th.App.UpdateChannelMemberSchemeRoles(th.Context, f.announcement.Id, bot.UserId, false, true, true)
		require.Nil(t, appErr)
		require.Nil(t, f.post(bot.UserId, f.announcement, "", model.CreatePostFlags{}), "a bot that's a channel admin")
	})

	t.Run("the AI post tool, as the person who asked", func(t *testing.T) {
		user := f.member(t, f.announcement)
		agent := th.CreateBot(t)
		args, err := json.Marshal(map[string]string{"channel_id": f.announcement.Id, "message": "Posted for them"})
		require.NoError(t, err)
		call := &aiToolCall{agent: &model.AIAgent{UserId: agent.UserId}, user: user, channel: f.announcement, teamID: f.team.Id, arguments: args}
		_, err = th.App.aiToolPostMessage(th.Context, call)
		require.Error(t, err, "the agent can't start a conversation for a member")

		replyArgs, err := json.Marshal(map[string]string{"channel_id": f.announcement.Id, "message": "A reply for them", "root_id": f.root.Id})
		require.NoError(t, err)
		call.arguments = replyArgs
		_, err = th.App.aiToolPostMessage(th.Context, call)
		require.NoError(t, err, "nor is it kept from replying")
	})

	t.Run("moving one's thread into it", func(t *testing.T) {
		user := f.member(t, f.announcement, f.chat)
		thread, _, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: user.Id, ChannelId: f.chat.Id, Message: "mine"}, f.chat, model.CreatePostFlags{})
		require.Nil(t, appErr)
		_, appErr = th.App.MoveThreadToChannel(th.Context, thread.Id, f.announcement.Id, false, user)
		requireRootPostRefusal(t, appErr, moderatorsOnly, "a move starts a conversation there")
		_, appErr = th.App.MoveThreadToChannel(th.Context, thread.Id, f.announcement.Id, true, user)
		requireRootPostRefusal(t, appErr, moderatorsOnly, "flattened too")
		appErr = th.App.MoveThread(th.Context, thread.Id, f.chat.Id, f.announcement.Id, user)
		requireRootPostRefusal(t, appErr, moderatorsOnly, "the other move")
		_, appErr = th.App.GetSinglePost(th.Context, thread.Id, false)
		require.Nil(t, appErr, "the thread stayed where it was")

		th.AddUserToChannel(t, th.SystemAdminUser, f.chat)
		moved, appErr := th.App.MoveThreadToChannel(th.Context, thread.Id, f.announcement.Id, false, th.SystemAdminUser)
		require.Nil(t, appErr, "a moderator moves it in; its copies aren't its author speaking")
		assert.Equal(t, user.Id, moved.UserId)
	})

	t.Run("the people who moderate the channel start conversations", func(t *testing.T) {
		channelAdmin := f.member(t, f.announcement)
		_, appErr := th.App.UpdateChannelMemberSchemeRoles(th.Context, f.announcement.Id, channelAdmin.Id, false, true, true)
		require.Nil(t, appErr)
		require.Nil(t, f.post(channelAdmin.Id, f.announcement, "", model.CreatePostFlags{}), "a channel admin")

		moderator := f.member(t, f.announcement)
		f.role(t, moderator.Id, model.PermissionDeleteOthersPosts.Id)
		require.Nil(t, f.post(moderator.Id, f.announcement, "", model.CreatePostFlags{}), "a role that manages messages")

		teamAdmin := f.member(t, f.announcement)
		_, appErr = th.App.UpdateTeamMemberSchemeRoles(th.Context, f.team.Id, teamAdmin.Id, false, true, true)
		require.Nil(t, appErr)
		require.Nil(t, f.post(teamAdmin.Id, f.announcement, "", model.CreatePostFlags{}), "a community admin")

		require.Nil(t, f.post(th.SystemAdminUser.Id, f.announcement, "", model.CreatePostFlags{}), "a system admin")

		for _, id := range []string{channelAdmin.Id, moderator.Id, teamAdmin.Id, th.SystemAdminUser.Id} {
			assert.True(t, th.App.IsChannelModerator(th.Context, id, f.announcement))
			assert.Equal(t, th.App.IsChannelModerator(th.Context, id, f.announcement), th.App.IsThreadModerator(th.Context, id, f.announcement), "both mean the same")
		}
		plain := f.member(t, f.announcement)
		assert.False(t, th.App.IsChannelModerator(th.Context, plain.Id, f.announcement))
		assert.False(t, th.App.IsThreadModerator(th.Context, plain.Id, f.announcement))
	})

	t.Run("overrides allowing create_root_post to a role or a person", func(t *testing.T) {
		writer := f.member(t, f.announcement)
		group := f.role(t, writer.Id, model.PermissionCreatePublicChannel.Id)
		requireRootPostRefusal(t, f.post(writer.Id, f.announcement, "", model.CreatePostFlags{}), moderatorsOnly, "a role without the override")
		f.override(t, f.announcement, model.ChannelOverrideTargetRole, group.Id, []string{model.PermissionCreateRootPost.Id}, nil)
		require.Nil(t, f.post(writer.Id, f.announcement, "", model.CreatePostFlags{}), "the role allowed")

		person := f.member(t, f.announcement)
		f.override(t, f.announcement, model.ChannelOverrideTargetUser, person.Id, []string{model.PermissionCreateRootPost.Id}, nil)
		require.Nil(t, f.post(person.Id, f.announcement, "", model.CreatePostFlags{}), "the person allowed")

		f.override(t, f.announcement, model.ChannelOverrideTargetUser, writer.Id, nil, []string{model.PermissionCreateRootPost.Id})
		// Still not a moderator of a channel where moderators start conversations: §3.2 says so.
		requireRootPostRefusal(t, f.post(writer.Id, f.announcement, "", model.CreatePostFlags{}), moderatorsOnly, "their own deny beats their role's allow")
	})

	t.Run("a guest", func(t *testing.T) {
		guest := th.CreateGuest(t)
		th.LinkUserToTeam(t, guest, f.team)
		th.AddUserToChannel(t, guest, f.announcement)
		requireRootPostRefusal(t, f.post(guest.Id, f.announcement, "", model.CreatePostFlags{}), moderatorsOnly)
		require.Nil(t, f.post(guest.Id, f.announcement, f.root.Id, model.CreatePostFlags{}))
	})

	t.Run("a timed-out moderator", func(t *testing.T) {
		channelAdmin := f.member(t, f.announcement)
		_, appErr := th.App.UpdateChannelMemberSchemeRoles(th.Context, f.announcement.Id, channelAdmin.Id, false, true, true)
		require.Nil(t, appErr)
		admin := model.Session{UserId: th.SystemAdminUser.Id, Roles: model.SystemAdminRoleId + " " + model.SystemUserRoleId}
		_, appErr = th.App.SetMemberTimeout(th.Context, admin, f.team.Id, channelAdmin.Id, &model.MemberTimeoutRequest{Until: model.GetMillis() + int64(time.Hour/time.Millisecond)})
		require.Nil(t, appErr)
		requireRootPostRefusal(t, f.post(channelAdmin.Id, f.announcement, "", model.CreatePostFlags{}), rootPostDenied, "they can't post at all")
	})

	t.Run("apps and system messages", func(t *testing.T) {
		user := f.member(t, f.announcement)
		require.Nil(t, f.post(user.Id, f.announcement, "", model.CreatePostFlags{FromPlugin: true}), "the plugin API: apps are trusted")
		_, _, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: user.Id, ChannelId: f.announcement.Id, Type: model.PostTypeJoinChannel, Message: "joined"}, f.announcement, model.CreatePostFlags{})
		require.Nil(t, appErr, "the server's own messages")
	})
}

func TestRootPostsInChatChannels(t *testing.T) {
	mainHelper.Parallel(t)
	f := setupPostingRules(t)

	t.Run("members start conversations", func(t *testing.T) {
		user := f.member(t, f.chat)
		require.Nil(t, f.post(user.Id, f.chat, "", model.CreatePostFlags{}))
		require.Nil(t, f.post(user.Id, f.chat, "", model.CreatePostFlags{FromIncomingWebhook: true}))
	})

	t.Run("an override denying create_root_post leaves only replies", func(t *testing.T) {
		user := f.member(t, f.chat)
		root, _, appErr := f.th.App.CreatePost(f.th.Context, &model.Post{UserId: user.Id, ChannelId: f.chat.Id, Message: "before"}, f.chat, model.CreatePostFlags{})
		require.Nil(t, appErr)
		f.override(t, f.chat, model.ChannelOverrideTargetUser, user.Id, nil, []string{model.PermissionCreateRootPost.Id})
		requireRootPostRefusal(t, f.post(user.Id, f.chat, "", model.CreatePostFlags{}), rootPostDenied)
		require.Nil(t, f.post(user.Id, f.chat, root.Id, model.CreatePostFlags{}))

		other := f.member(t, f.chat)
		require.Nil(t, f.post(other.Id, f.chat, "", model.CreatePostFlags{}), "only the person denied")
	})

	t.Run("a role's deny, and a team admin passing over it", func(t *testing.T) {
		muted := f.member(t, f.chat)
		group := f.role(t, muted.Id, model.PermissionCreatePublicChannel.Id)
		f.override(t, f.chat, model.ChannelOverrideTargetRole, group.Id, nil, []string{model.PermissionCreateRootPost.Id})
		requireRootPostRefusal(t, f.post(muted.Id, f.chat, "", model.CreatePostFlags{}), rootPostDenied)

		teamAdmin := f.member(t, f.chat)
		_, appErr := f.th.App.UpsertGroupMember(group.Id, teamAdmin.Id)
		require.Nil(t, appErr)
		_, appErr = f.th.App.UpdateTeamMemberSchemeRoles(f.th.Context, f.team.Id, teamAdmin.Id, false, true, true)
		require.Nil(t, appErr)
		require.Nil(t, f.post(teamAdmin.Id, f.chat, "", model.CreatePostFlags{}), "team admins pass over overrides")
	})

	t.Run("switching the preset applies to new posts, and overrides stay", func(t *testing.T) {
		channel := createTypedChannel(t, f.th, model.ChannelPresetTextChat)
		allowed := f.member(t, channel)
		user := f.member(t, channel)
		f.override(t, channel, model.ChannelOverrideTargetUser, allowed.Id, []string{model.PermissionCreateRootPost.Id}, nil)
		require.Nil(t, f.post(user.Id, channel, "", model.CreatePostFlags{}))

		_, appErr := f.th.App.UpdateChannelType(f.th.Context, channel, model.ChannelTypeRequest{Preset: model.ChannelPresetTextAnnouncement}, f.th.SystemAdminUser.Id)
		require.Nil(t, appErr)
		requireRootPostRefusal(t, f.post(user.Id, channel, "", model.CreatePostFlags{}), moderatorsOnly)
		require.Nil(t, f.post(allowed.Id, channel, "", model.CreatePostFlags{}), "the override set before still applies")

		overrides, appErr := f.th.App.GetChannelOverrides(f.th.Context, channel.Id)
		require.Nil(t, appErr)
		assert.Len(t, overrides, 1, "a preset writes no override")
	})

	t.Run("direct messages have no such rule", func(t *testing.T) {
		dm, appErr := f.th.App.GetOrCreateDirectChannel(f.th.Context, f.th.BasicUser.Id, f.th.BasicUser2.Id)
		require.Nil(t, appErr)
		assert.Nil(t, f.th.App.CanStartConversations(f.th.Context, f.th.BasicUser.Id, dm))
	})
}

// A forum's Announcements preset is the same rule: moderators open topics, members reply.
func TestRootPostsInAnnouncementForums(t *testing.T) {
	mainHelper.Parallel(t)
	f := setupPostingRules(t)
	forum := createTypedChannel(t, f.th, model.ChannelPresetForumAnnouncement)
	user := f.member(t, forum)
	requireRootPostRefusal(t, f.th.App.CanStartConversations(f.th.Context, user.Id, forum), moderatorsOnly)
	assert.Nil(t, f.th.App.CanStartConversations(f.th.Context, f.th.SystemAdminUser.Id, forum))

	agora := createTypedChannel(t, f.th, model.ChannelPresetForumAgora)
	f.th.AddUserToChannel(t, user, agora)
	assert.Nil(t, f.th.App.CanStartConversations(f.th.Context, user.Id, agora))
}
