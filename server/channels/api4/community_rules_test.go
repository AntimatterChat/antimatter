// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// Antimatter community rules over the API: who reads and changes them, who agrees to them, and that a member who
// hasn't agreed can't post through any endpoint while reading, reacting and direct messages stay open.

const rulesNotAcceptedID = "api.post.community_rules_not_accepted"

func TestCommunityRulesAPI(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()
	team := th.BasicTeam

	// The basic user is one of the community's admins; the others are plain members.
	_, appErr := th.App.UpdateTeamMemberSchemeRoles(th.Context, team.Id, th.BasicUser.Id, false, true, true)
	require.Nil(t, appErr)
	member := func(t *testing.T) (*model.User, *model.Client4) {
		t.Helper()
		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, team)
		th.AddUserToChannel(t, user, th.BasicChannel)
		return user, loggedInAs(t, th, user)
	}
	turnOn := func(t *testing.T, askAgain bool) *model.CommunityRules {
		t.Helper()
		rules, _, err := th.Client.UpdateCommunityRules(ctx, team.Id, &model.CommunityRulesRequest{Enabled: true, Text: "1. Be kind.\n2. No spam.", AskAgain: askAgain})
		require.NoError(t, err)
		return rules
	}

	t.Run("only the community's admins change the rules", func(t *testing.T) {
		_, client := member(t)
		_, resp, err := client.UpdateCommunityRules(ctx, team.Id, &model.CommunityRulesRequest{Enabled: true, Text: "Mine now"})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		outsider := th.CreateUser(t)
		_, resp, err = loggedInAs(t, th, outsider).UpdateCommunityRules(ctx, team.Id, &model.CommunityRulesRequest{Enabled: true, Text: "Mine now"})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		rules, resp, err := th.Client.UpdateCommunityRules(ctx, team.Id, &model.CommunityRulesRequest{Enabled: true, Text: "Be kind."})
		require.NoError(t, err, "a community admin")
		CheckOKStatus(t, resp)
		assert.True(t, rules.Enabled)

		_, _, err = th.SystemAdminClient.UpdateCommunityRules(ctx, team.Id, &model.CommunityRulesRequest{Enabled: true, Text: "Be kind."})
		require.NoError(t, err, "a system admin")

		_, resp, err = th.Client.UpdateCommunityRules(ctx, team.Id, &model.CommunityRulesRequest{Enabled: true, Text: ""})
		require.Error(t, err, "not turned on empty")
		CheckBadRequestStatus(t, resp)

		got, _, err := th.Client.GetCommunityRules(ctx, team.Id)
		require.NoError(t, err)
		assert.Equal(t, "Be kind.", got.Text, "a refused change changes nothing")
	})

	t.Run("members and those who may join read the rules; others don't", func(t *testing.T) {
		turnOn(t, false)
		_, client := member(t)
		rules, _, err := client.GetCommunityRules(ctx, team.Id)
		require.NoError(t, err)
		assert.True(t, rules.MustAccept)
		assert.Empty(t, rules.UpdatedBy, "who wrote them is for the admins")
		mine, _, err := th.Client.GetCommunityRules(ctx, team.Id)
		require.NoError(t, err)
		assert.False(t, mine.MustAccept, "admins are exempt")
		assert.Equal(t, th.BasicUser.Id, mine.UpdatedBy)

		outsider := loggedInAs(t, th, th.CreateUser(t))
		_, resp, err := outsider.GetCommunityRules(ctx, team.Id)
		require.Error(t, err, "an invite-only community's rules are its own")
		CheckForbiddenStatus(t, resp)

		open := th.CreateTeamWithClient(t, th.SystemAdminClient)
		_, _, err = th.SystemAdminClient.PatchTeam(ctx, open.Id, &model.TeamPatch{AllowOpenInvite: model.NewPointer(true)})
		require.NoError(t, err)
		_, _, err = th.SystemAdminClient.UpdateCommunityRules(ctx, open.Id, &model.CommunityRulesRequest{Enabled: true, Text: "Open rules"})
		require.NoError(t, err)
		rules, _, err = outsider.GetCommunityRules(ctx, open.Id)
		require.NoError(t, err, "anyone who may join an open community reads its rules first")
		assert.Equal(t, "Open rules", rules.Text)
		_, resp, err = outsider.AcceptCommunityRules(ctx, open.Id, rules.Version)
		require.Error(t, err, "but agrees only once a member")
		CheckForbiddenStatus(t, resp)
	})

	t.Run("only the community's admins see who agreed", func(t *testing.T) {
		turnOn(t, true)
		_, client := member(t)
		_, resp, err := client.GetCommunityRulesStats(ctx, team.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		before, _, err := th.Client.GetCommunityRulesStats(ctx, team.Id)
		require.NoError(t, err)
		_, _, err = client.AcceptCommunityRules(ctx, team.Id, 0)
		require.NoError(t, err)
		after, _, err := th.Client.GetCommunityRulesStats(ctx, team.Id)
		require.NoError(t, err)
		assert.Equal(t, before.AcceptedCount+1, after.AcceptedCount)
		assert.Equal(t, before.MemberCount, after.MemberCount)
	})

	t.Run("a member who hasn't agreed can't post through any endpoint", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.ScheduledPosts = model.NewPointer(true) })
		user, client := member(t)

		// A message written while the rules were off.
		_, _, err := th.Client.UpdateCommunityRules(ctx, team.Id, &model.CommunityRulesRequest{Enabled: false, Text: "1. Be kind."})
		require.NoError(t, err)
		old, _, err := client.CreatePost(ctx, &model.Post{ChannelId: th.BasicChannel.Id, Message: "before the rules"})
		require.NoError(t, err, "rules turned off hold nobody back")
		rules := turnOn(t, true)

		_, resp, err := client.CreatePost(ctx, &model.Post{ChannelId: th.BasicChannel.Id, Message: "hello"})
		CheckErrorID(t, err, rulesNotAcceptedID)
		CheckForbiddenStatus(t, resp)

		_, _, err = client.CreatePost(ctx, &model.Post{ChannelId: th.BasicChannel.Id, RootId: th.BasicPost.Id, Message: "a reply"})
		CheckErrorID(t, err, rulesNotAcceptedID)

		upload, _, err := client.UploadFile(ctx, []byte("data"), th.BasicChannel.Id, "notes.txt")
		require.NoError(t, err, "uploading alone posts nothing")
		_, _, err = client.CreatePost(ctx, &model.Post{ChannelId: th.BasicChannel.Id, FileIds: model.StringArray{upload.FileInfos[0].Id}})
		CheckErrorID(t, err, rulesNotAcceptedID)

		_, _, err = client.CreateScheduledPost(ctx, &model.ScheduledPost{
			Draft:       model.Draft{ChannelId: th.BasicChannel.Id, Message: "later"},
			ScheduledAt: model.GetMillis() + time.Hour.Milliseconds(),
		})
		CheckErrorID(t, err, rulesNotAcceptedID)

		_, _, err = client.ExecuteCommand(ctx, th.BasicChannel.Id, "/me waves")
		CheckErrorID(t, err, rulesNotAcceptedID)
		_, _, err = client.ExecuteCommand(ctx, th.BasicChannel.Id, "/shrug")
		CheckErrorID(t, err, rulesNotAcceptedID)

		_, _, err = client.PatchPost(ctx, old.Id, &model.PostPatch{Message: model.NewPointer("rewritten after the rules")})
		CheckErrorID(t, err, rulesNotAcceptedID)
		edited := old.Clone()
		edited.Message = "rewritten after the rules"
		_, _, err = client.UpdatePost(ctx, old.Id, edited)
		CheckErrorID(t, err, rulesNotAcceptedID)

		_, _, err = client.GetPostsForChannel(ctx, th.BasicChannel.Id, 0, 10, "", false, false)
		require.NoError(t, err, "reading is never held back")
		_, _, err = client.SaveReaction(ctx, &model.Reaction{UserId: user.Id, PostId: th.BasicPost.Id, EmojiName: "wave"})
		require.NoError(t, err, "reactions aren't held back")
		dm, _, err := client.CreateDirectChannel(ctx, user.Id, th.BasicUser2.Id)
		require.NoError(t, err)
		_, _, err = client.CreatePost(ctx, &model.Post{ChannelId: dm.Id, Message: "direct messages aren't the community's"})
		require.NoError(t, err)

		agreed, _, err := client.AcceptCommunityRules(ctx, team.Id, rules.Version)
		require.NoError(t, err)
		assert.False(t, agreed.MustAccept)
		_, _, err = client.CreatePost(ctx, &model.Post{ChannelId: th.BasicChannel.Id, Message: "hello"})
		require.NoError(t, err, "once they agree, they post")
		_, _, err = client.ExecuteCommand(ctx, th.BasicChannel.Id, "/me waves")
		require.NoError(t, err)
		_, _, err = client.PatchPost(ctx, old.Id, &model.PostPatch{Message: model.NewPointer("edited")})
		require.NoError(t, err)

		again := turnOn(t, true)
		_, _, err = client.CreatePost(ctx, &model.Post{ChannelId: th.BasicChannel.Id, Message: "hello again"})
		CheckErrorID(t, err, rulesNotAcceptedID)
		_, resp, err = client.AcceptCommunityRules(ctx, team.Id, rules.Version)
		require.Error(t, err, "agreeing to the old version doesn't do")
		checkHTTPStatus(t, resp, http.StatusConflict)
		_, _, err = client.AcceptCommunityRules(ctx, team.Id, again.Version)
		require.NoError(t, err)
		_, _, err = client.CreatePost(ctx, &model.Post{ChannelId: th.BasicChannel.Id, Message: "hello again"})
		require.NoError(t, err)
	})

	t.Run("the community's admins post without agreeing", func(t *testing.T) {
		turnOn(t, true)
		_, _, err := th.Client.CreatePost(ctx, &model.Post{ChannelId: th.BasicChannel.Id, Message: "from an admin"})
		require.NoError(t, err)
		_, _, err = th.Client.ExecuteCommand(ctx, th.BasicChannel.Id, "/me waves")
		require.NoError(t, err)
	})

	t.Run("clients hear when the rules change, and when they agreed elsewhere", func(t *testing.T) {
		user, client := member(t)
		wsClient := th.CreateConnectedWebSocketClientWithClient(t, client)
		waitFor := func(eventType model.WebsocketEventType) *model.WebSocketEvent {
			t.Helper()
			timeout := time.After(5 * time.Second)
			for {
				select {
				case event := <-wsClient.EventChannel:
					if event.EventType() == eventType {
						return event
					}
				case <-timeout:
					require.FailNow(t, "no websocket event", string(eventType))
					return nil
				}
			}
		}

		rules := turnOn(t, true)
		event := waitFor(model.WebsocketEventCommunityRulesUpdated)
		assert.Equal(t, team.Id, event.GetData()["team_id"])
		assert.Equal(t, team.Id, event.GetBroadcast().TeamId, "the community's members only")

		// Clients without a screen for the rules are told, where the message was refused, how to agree.
		_, _, err := client.CreatePost(ctx, &model.Post{ChannelId: th.BasicChannel.Id, Message: "hello"})
		CheckErrorID(t, err, rulesNotAcceptedID)
		event = waitFor(model.WebsocketEventEphemeralMessage)
		reminder, ok := event.GetData()["post"].(string)
		require.True(t, ok)
		assert.Contains(t, reminder, "/rules agree")
		assert.Contains(t, reminder, th.BasicChannel.Id)

		_, _, err = client.AcceptCommunityRules(ctx, team.Id, rules.Version)
		require.NoError(t, err)
		event = waitFor(model.WebsocketEventCommunityRulesAccepted)
		assert.Equal(t, team.Id, event.GetData()["team_id"])
		assert.Equal(t, user.Id, event.GetBroadcast().UserId, "nobody else learns who agreed")
	})
}
