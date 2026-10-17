// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// Antimatter community rules: who is held back from posting in a community until they agree to its rules, and who
// isn't. A regression here either silences a community or lets people post without agreeing, so each case is spelt
// out.

const rulesNotAccepted = "api.post.community_rules_not_accepted"

func turnOnRules(t *testing.T, th *TestHelper, teamID string, askAgain bool) *model.CommunityRules {
	t.Helper()
	rules, appErr := th.App.SaveCommunityRules(th.Context, teamID, th.SystemAdminUser.Id, &model.CommunityRulesRequest{Enabled: true, Text: "Be kind. No spam.", AskAgain: askAgain})
	require.Nil(t, appErr)
	return rules
}

func requireRulesRefusal(t *testing.T, appErr *model.AppError, msgAndArgs ...any) {
	t.Helper()
	require.NotNil(t, appErr, msgAndArgs...)
	assert.Equal(t, rulesNotAccepted, appErr.Id, msgAndArgs...)
	assert.Equal(t, http.StatusForbidden, appErr.StatusCode)
}

func TestSaveCommunityRules(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	teamID := th.BasicTeam.Id

	rules, appErr := th.App.GetCommunityRules(th.Context, teamID)
	require.Nil(t, appErr)
	assert.False(t, rules.IsActive(), "a community has no rules until its admins write them")
	assert.Zero(t, rules.Version)

	_, appErr = th.App.SaveCommunityRules(th.Context, teamID, th.SystemAdminUser.Id, &model.CommunityRulesRequest{Enabled: true, Text: "  "})
	require.NotNil(t, appErr, "rules aren't turned on empty")
	assert.Equal(t, "model.community_rules.empty.app_error", appErr.Id)

	rules = turnOnRules(t, th, teamID, false)
	assert.Equal(t, int64(1), rules.Version)
	assert.Equal(t, "Be kind. No spam.", rules.Text)

	got, appErr := th.App.GetCommunityRules(th.Context, teamID)
	require.Nil(t, appErr)
	assert.True(t, got.IsActive(), "the cache follows a save at once")

	rules = turnOnRules(t, th, teamID, false)
	assert.Equal(t, int64(1), rules.Version, "a save that doesn't ask again keeps the version")
	rules = turnOnRules(t, th, teamID, true)
	assert.Equal(t, int64(2), rules.Version)

	rules, appErr = th.App.SaveCommunityRules(th.Context, teamID, th.SystemAdminUser.Id, &model.CommunityRulesRequest{Enabled: false, Text: "Be kind."})
	require.Nil(t, appErr)
	assert.False(t, rules.IsActive())
	assert.Equal(t, int64(2), rules.Version)
}

func TestAcceptCommunityRules(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	teamID := th.BasicTeam.Id

	_, appErr := th.App.AcceptCommunityRules(th.Context, teamID, th.BasicUser.Id, 0)
	require.NotNil(t, appErr, "no rules, nothing to agree to")
	assert.Equal(t, "app.community_rules.accept.off.app_error", appErr.Id)

	rules := turnOnRules(t, th, teamID, false)
	mine, appErr := th.App.GetCommunityRulesForUser(th.Context, teamID, th.BasicUser.Id)
	require.Nil(t, appErr)
	assert.True(t, mine.MustAccept)
	assert.Zero(t, mine.AcceptedVersion)

	outsider := th.CreateUser(t)
	_, appErr = th.App.AcceptCommunityRules(th.Context, teamID, outsider.Id, rules.Version)
	require.NotNil(t, appErr, "only members agree")
	assert.Equal(t, http.StatusForbidden, appErr.StatusCode)

	_, appErr = th.App.AcceptCommunityRules(th.Context, teamID, th.BasicUser.Id, rules.Version+1)
	require.NotNil(t, appErr, "nobody agrees to a version they didn't read")
	assert.Equal(t, http.StatusConflict, appErr.StatusCode)

	mine, appErr = th.App.AcceptCommunityRules(th.Context, teamID, th.BasicUser.Id, rules.Version)
	require.Nil(t, appErr)
	assert.False(t, mine.MustAccept)
	assert.Equal(t, rules.Version, mine.AcceptedVersion)

	rules = turnOnRules(t, th, teamID, true)
	_, appErr = th.App.AcceptCommunityRules(th.Context, teamID, th.BasicUser.Id, rules.Version-1)
	require.NotNil(t, appErr, "the rules changed since they read them")
	assert.Equal(t, "app.community_rules.accept.changed.app_error", appErr.Id)
	mine, appErr = th.App.GetCommunityRulesForUser(th.Context, teamID, th.BasicUser.Id)
	require.Nil(t, appErr)
	assert.True(t, mine.MustAccept, "asking again needs them to agree again")
	assert.Equal(t, rules.Version-1, mine.AcceptedVersion)

	stats, appErr := th.App.GetCommunityRulesStats(th.Context, teamID)
	require.Nil(t, appErr)
	assert.Equal(t, int64(2), stats.MemberCount)
	assert.Zero(t, stats.AcceptedCount, "nobody agreed to the new version yet")
	_, appErr = th.App.AcceptCommunityRules(th.Context, teamID, th.BasicUser.Id, 0)
	require.Nil(t, appErr, "0 agrees to the current version")
	stats, appErr = th.App.GetCommunityRulesStats(th.Context, teamID)
	require.Nil(t, appErr)
	assert.Equal(t, int64(1), stats.AcceptedCount)

	admin, appErr := th.App.GetCommunityRulesForUser(th.Context, teamID, th.SystemAdminUser.Id)
	require.Nil(t, appErr)
	assert.False(t, admin.MustAccept, "admins aren't held back")
}

func TestCommunityRulesEnforcement(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	team, channel := th.BasicTeam, th.BasicChannel
	post := func(userID, channelID string, flags model.CreatePostFlags, edit func(*model.Post)) *model.AppError {
		ch, appErr := th.App.GetChannel(th.Context, channelID)
		require.Nil(t, appErr)
		p := &model.Post{UserId: userID, ChannelId: channelID, Message: "hello " + model.NewId()}
		if edit != nil {
			edit(p)
		}
		_, _, appErr = th.App.CreatePost(th.Context, p, ch, flags)
		return appErr
	}
	member := func(t *testing.T) *model.User {
		t.Helper()
		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, team)
		th.AddUserToChannel(t, user, channel)
		return user
	}

	rules := turnOnRules(t, th, team.Id, false)

	t.Run("a member who hasn't agreed can't post, reply or send files", func(t *testing.T) {
		user := member(t)
		requireRulesRefusal(t, post(user.Id, channel.Id, model.CreatePostFlags{}, nil))
		requireRulesRefusal(t, post(user.Id, channel.Id, model.CreatePostFlags{}, func(p *model.Post) { p.RootId = th.BasicPost.Id }), "a reply")
		file := th.CreateFileInfo(t, user.Id, "", channel.Id)
		requireRulesRefusal(t, post(user.Id, channel.Id, model.CreatePostFlags{}, func(p *model.Post) { p.Message = ""; p.FileIds = []string{file.Id} }), "a file")
		requireRulesRefusal(t, post(user.Id, channel.Id, model.CreatePostFlags{FromPlugin: true}, nil), "an app posting as them")
	})

	t.Run("once they agree they can; asking again holds them back again", func(t *testing.T) {
		user := member(t)
		_, appErr := th.App.AcceptCommunityRules(th.Context, team.Id, user.Id, rules.Version)
		require.Nil(t, appErr)
		require.Nil(t, post(user.Id, channel.Id, model.CreatePostFlags{}, nil), "right after agreeing")

		again := turnOnRules(t, th, team.Id, true)
		requireRulesRefusal(t, post(user.Id, channel.Id, model.CreatePostFlags{}, nil), "the new version isn't agreed to")
		_, appErr = th.App.AcceptCommunityRules(th.Context, team.Id, user.Id, again.Version)
		require.Nil(t, appErr)
		require.Nil(t, post(user.Id, channel.Id, model.CreatePostFlags{}, nil))
	})

	t.Run("the community's admins and system admins are exempt", func(t *testing.T) {
		admin := member(t)
		_, appErr := th.App.UpdateTeamMemberSchemeRoles(th.Context, team.Id, admin.Id, false, true, true)
		require.Nil(t, appErr)
		require.Nil(t, post(admin.Id, channel.Id, model.CreatePostFlags{}, nil))

		th.LinkUserToTeam(t, th.SystemAdminUser, team)
		th.AddUserToChannel(t, th.SystemAdminUser, channel)
		require.Nil(t, post(th.SystemAdminUser.Id, channel.Id, model.CreatePostFlags{}, nil))
	})

	t.Run("bots, webhooks and system messages are exempt", func(t *testing.T) {
		bot := th.CreateBot(t)
		botUser, appErr := th.App.GetUser(th.Context, bot.UserId)
		require.Nil(t, appErr)
		th.LinkUserToTeam(t, botUser, team)
		th.AddUserToChannel(t, botUser, channel)
		require.Nil(t, post(bot.UserId, channel.Id, model.CreatePostFlags{FromPlugin: true}, nil), "an app acting as itself")

		user := member(t)
		require.Nil(t, post(user.Id, channel.Id, model.CreatePostFlags{FromIncomingWebhook: true}, nil), "an incoming webhook made by a member")
		require.Nil(t, post(user.Id, channel.Id, model.CreatePostFlags{}, func(p *model.Post) { p.Type = model.PostTypeJoinChannel }), "their joining the channel")
	})

	t.Run("direct and group messages aren't a community's", func(t *testing.T) {
		user := member(t)
		dm, appErr := th.App.GetOrCreateDirectChannel(th.Context, user.Id, th.BasicUser2.Id)
		require.Nil(t, appErr)
		require.Nil(t, post(user.Id, dm.Id, model.CreatePostFlags{}, nil))
		gm, appErr := th.App.CreateGroupChannel(th.Context, []string{user.Id, th.BasicUser.Id, th.BasicUser2.Id}, user.Id)
		require.Nil(t, appErr)
		require.Nil(t, post(user.Id, gm.Id, model.CreatePostFlags{}, nil))
	})

	t.Run("reading and reacting aren't held back", func(t *testing.T) {
		user := member(t)
		_, appErr := th.App.GetPostsPage(th.Context, model.GetPostsOptions{ChannelId: channel.Id, Page: 0, PerPage: 10})
		require.Nil(t, appErr)
		_, appErr = th.App.SaveReactionForPost(th.Context, &model.Reaction{UserId: user.Id, PostId: th.BasicPost.Id, EmojiName: "smile"})
		require.Nil(t, appErr, "a reaction carries no message")
	})

	t.Run("other communities aren't affected", func(t *testing.T) {
		user := member(t)
		other := th.CreateTeam(t)
		th.LinkUserToTeam(t, user, other)
		otherChannel := th.CreateChannel(t, other)
		th.AddUserToChannel(t, user, otherChannel)
		require.Nil(t, post(user.Id, otherChannel.Id, model.CreatePostFlags{}, nil))
	})

	t.Run("rules turned off hold nobody back", func(t *testing.T) {
		user := member(t)
		_, appErr := th.App.SaveCommunityRules(th.Context, team.Id, th.SystemAdminUser.Id, &model.CommunityRulesRequest{Enabled: false, Text: "Be kind."})
		require.Nil(t, appErr)
		t.Cleanup(func() { turnOnRules(t, th, team.Id, false) })
		require.Nil(t, post(user.Id, channel.Id, model.CreatePostFlags{}, nil))
	})
}

func TestCommunityRulesScheduledPosts(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	scheduled, err := th.Server.Store().ScheduledPost().CreateScheduledPost(th.Context, &model.ScheduledPost{
		Draft: model.Draft{
			CreateAt:  model.GetMillis(),
			UserId:    th.BasicUser.Id,
			ChannelId: th.BasicChannel.Id,
			Message:   "written before the rules",
		},
		ScheduledAt: model.GetMillis() - 1000,
	})
	require.NoError(t, err)
	turnOnRules(t, th, th.BasicTeam.Id, false)

	th.App.ProcessScheduledPosts(th.Context)

	failed, err := th.Server.Store().ScheduledPost().Get(th.Context, scheduled.Id)
	require.NoError(t, err, "a scheduled message that can't be sent is kept, with why")
	assert.Equal(t, model.ScheduledPostErrorCommunityRulesNotAccepted, failed.ErrorCode)
	posts, appErr := th.App.GetPostsPage(th.Context, model.GetPostsOptions{ChannelId: th.BasicChannel.Id, Page: 0, PerPage: 20})
	require.Nil(t, appErr)
	for _, p := range posts.Posts {
		assert.NotEqual(t, "written before the rules", p.Message, "it wasn't sent")
	}

	// Once its author agrees, a new one goes out.
	_, appErr = th.App.AcceptCommunityRules(th.Context, th.BasicTeam.Id, th.BasicUser.Id, 0)
	require.Nil(t, appErr)
	again, err := th.Server.Store().ScheduledPost().CreateScheduledPost(th.Context, &model.ScheduledPost{
		Draft: model.Draft{
			CreateAt:  model.GetMillis(),
			UserId:    th.BasicUser.Id,
			ChannelId: th.BasicChannel.Id,
			Message:   "written after agreeing",
		},
		ScheduledAt: model.GetMillis() - 1000,
	})
	require.NoError(t, err)
	th.App.ProcessScheduledPosts(th.Context)
	left, err := th.Server.Store().ScheduledPost().GetScheduledPostsForUser(th.Context, th.BasicUser.Id, th.BasicTeam.Id)
	require.NoError(t, err)
	for _, sp := range left {
		assert.NotEqual(t, again.Id, sp.Id, "sent, so no longer scheduled")
	}
}

func TestCommunityRulesMoveThread(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	team := th.BasicTeam
	target := th.CreateChannel(t, team)
	th.AddUserToChannel(t, th.BasicUser2, target)

	// A thread written before the rules, by someone who never agreed to them.
	root, _, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: th.BasicUser2.Id, ChannelId: th.BasicChannel.Id, Message: "before the rules"}, th.BasicChannel, model.CreatePostFlags{})
	require.Nil(t, appErr)
	turnOnRules(t, th, team.Id, false)

	_, appErr = th.App.MoveThreadToChannel(th.Context, root.Id, target.Id, false, th.BasicUser2)
	requireRulesRefusal(t, appErr, "moving one's own thread is posting it anew")

	th.LinkUserToTeam(t, th.SystemAdminUser, team)
	th.AddUserToChannel(t, th.SystemAdminUser, th.BasicChannel)
	th.AddUserToChannel(t, th.SystemAdminUser, target)
	moved, appErr := th.App.MoveThreadToChannel(th.Context, root.Id, target.Id, false, th.SystemAdminUser)
	require.Nil(t, appErr, "an admin moving someone's thread: the copy isn't its author speaking")
	assert.Equal(t, th.BasicUser2.Id, moved.UserId)
}

func TestCommunityRulesGoWithTheTeam(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	team := th.CreateTeam(t)
	th.LinkUserToTeam(t, th.BasicUser, team)
	turnOnRules(t, th, team.Id, false)
	_, appErr := th.App.AcceptCommunityRules(th.Context, team.Id, th.BasicUser.Id, 0)
	require.Nil(t, appErr)

	require.Nil(t, th.App.PermanentDeleteTeam(th.Context, team))
	_, err := th.Server.Store().CommunityRules().Get(th.Context, team.Id)
	require.Error(t, err)
	acceptances, err := th.Server.Store().CommunityRules().GetAcceptancesForUser(th.Context, th.BasicUser.Id)
	require.NoError(t, err)
	assert.Empty(t, acceptances)
}

func TestCommunityRulesCommandResponses(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	turnOnRules(t, th, th.BasicTeam.Id, false)
	args := &model.CommandArgs{UserId: th.BasicUser.Id, ChannelId: th.BasicChannel.Id, TeamId: th.BasicTeam.Id, Command: "/weather"}
	inChannel := &model.CommandResponse{ResponseType: model.CommandResponseTypeInChannel, Text: "Sunny"}

	// An integration's answer is flagged from_webhook, but it's posted as the one who ran the command.
	_, appErr := th.App.HandleCommandResponsePost(th.Context, &model.Command{}, args, inChannel, false)
	requireRulesRefusal(t, appErr, "an integration's answer in the channel")
	_, appErr = th.App.HandleCommandResponsePost(th.Context, &model.Command{}, args, inChannel, true)
	requireRulesRefusal(t, appErr, "a built-in command's answer in the channel")
	_, appErr = th.App.HandleCommandResponse(th.Context, &model.Command{}, args, inChannel, false)
	requireRulesRefusal(t, appErr, "said as such, not as a failed command")

	_, appErr = th.App.HandleCommandResponsePost(th.Context, &model.Command{}, args, &model.CommandResponse{ResponseType: model.CommandResponseTypeEphemeral, Text: "Only you see this"}, false)
	require.Nil(t, appErr, "an answer only they see isn't posting")

	_, appErr = th.App.AcceptCommunityRules(th.Context, th.BasicTeam.Id, th.BasicUser.Id, 0)
	require.Nil(t, appErr)
	_, appErr = th.App.HandleCommandResponsePost(th.Context, &model.Command{}, args, inChannel, false)
	require.Nil(t, appErr)
}
