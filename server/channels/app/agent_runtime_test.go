// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestMentionedUsernames(t *testing.T) {
	assert.Equal(t, []string{"researcher", "coder"}, mentionedUsernames("@Researcher, ask @coder. Then @researcher again."))
	assert.Equal(t, []string{"scout"}, mentionedUsernames("hey @scout."))
	assert.Empty(t, mentionedUsernames("mail me at maya@example.com"), "an email address isn't a mention")
	assert.Empty(t, mentionedUsernames("run `@researcher` and\n```\n@coder\n```"), "nor code")
	assert.Empty(t, mentionedUsernames("no one here"))
}

func TestPostMayStartAgentTurn(t *testing.T) {
	assert.True(t, postMayStartAgentTurn(&model.Post{Message: "@agent hi"}))
	for name, post := range map[string]*model.Post{
		"a system message": {Type: model.PostTypeJoinChannel},
		"a webhook's":      {Props: model.StringInterface{model.PostPropsFromWebhook: "true"}},
		"a bot's":          {Props: model.StringInterface{model.PostPropsFromBot: "true"}},
		"an app's":         {Props: model.StringInterface{model.PostPropsFromPlugin: "true"}},
		"a streaming one":  {Props: model.StringInterface{PostPropAgentStreaming: true}},
		"another server's": {RemoteId: model.NewPointer(model.NewId())},
		"a deleted one":    {DeleteAt: 1},
	} {
		assert.False(t, postMayStartAgentTurn(post), name)
	}
}

// waitForAgentReply waits for the agent's finished reply in a thread.
func waitForAgentReply(t *testing.T, th *TestHelper, agentID, rootID string) *model.Post {
	t.Helper()
	var reply *model.Post
	require.Eventually(t, func() bool {
		list, appErr := th.App.GetPostThread(th.Context, rootID, model.GetPostsOptions{}, th.BasicUser.Id)
		if appErr != nil {
			return false
		}
		for _, p := range list.Posts {
			if p.UserId == agentID && p.GetProp(PostPropAgentStreaming) == nil {
				reply = p
				return true
			}
		}
		return false
	}, 15*time.Second, 50*time.Millisecond, "the agent answers")
	return reply
}

// savePostQuietly saves a post without starting agent turns, for the tests that run the turn themselves.
func savePostQuietly(t *testing.T, th *TestHelper, post *model.Post) *model.Post {
	t.Helper()
	saved, err := th.App.Srv().Store().Post().Save(th.Context, post)
	require.NoError(t, err)
	return saved
}

func lastLLMRequestText(t *testing.T, f *fakeLLM) string {
	t.Helper()
	calls := f.calls()
	require.NotEmpty(t, calls, "the model was called")
	var all strings.Builder
	for _, m := range calls[len(calls)-1].messages() {
		content, _ := m["content"].(string)
		all.WriteString(content)
		all.WriteString("\n")
	}
	return all.String()
}

// A mention starts a turn: the answer streams into the agent's reply in the thread, which isn't marked edited, the
// call is the asker's (their usage), and the agent is idle again.
func TestAgentAnswersAMention(t *testing.T) {
	answer := "Paris is the capital of France. It sits on the Seine, and it has been the country's capital for centuries."
	th, f, c := setupAgentTest(t, answer)
	f.streamIn(answer[:40], answer[40:80], answer[80:])
	agent := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil)

	post := th.CreateMessagePost(t, th.BasicChannel, "@"+agent.Username+" what is the capital of France?")
	reply := waitForAgentReply(t, th, agent.UserId, post.Id)
	assert.Equal(t, answer, reply.Message)
	assert.Equal(t, post.Id, reply.RootId, "in the thread of the question")
	assert.Zero(t, reply.EditAt, "the streamed text isn't an edit")
	assert.Equal(t, "true", reply.GetProp(model.PostPropsFromBot))

	body := f.calls()[len(f.calls())-1].Body
	assert.Equal(t, "agent-model", body["model"])
	assert.Equal(t, true, body["stream"])
	messages := f.calls()[len(f.calls())-1].messages()
	require.Len(t, messages, 2)
	assert.Equal(t, "system", messages[0]["role"])
	assert.Contains(t, messages[0]["content"], "You are a careful researcher.", "the agent's own prompt")
	assert.Contains(t, messages[0]["content"], "@"+th.BasicUser.Username, "it knows who it answers")
	assert.Contains(t, messages[1]["content"], "what is the capital of France?")

	require.Eventually(t, func() bool {
		activity, appErr := th.App.GetAgentActivity(th.Context, agent.UserId, th.BasicUser.Id)
		return appErr == nil && activity.State == model.AgentStateIdle
	}, 5*time.Second, 50*time.Millisecond, "the agent is idle again")

	var usage *model.AIConnectionUsage
	require.Eventually(t, func() bool {
		var appErr *model.AppError
		usage, appErr = th.App.GetAIConnectionUsage(th.Context, c)
		return appErr == nil && usage.ByPurpose["agent:"+agent.UserId].Requests == 1
	}, 5*time.Second, 50*time.Millisecond, "the call is counted for the agent")
	require.NotEmpty(t, usage.Days)
	assert.Equal(t, th.BasicUser.Id, usage.Days[0].UserId, "against the person who asked")
}

func TestAgentAnswersADirectMessage(t *testing.T) {
	th, f, c := setupAgentTest(t, "Hello! How can I help?")
	agent := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil)
	dm, appErr := th.App.GetOrCreateDirectChannel(th.Context, th.BasicUser.Id, agent.UserId)
	require.Nil(t, appErr)

	post := th.CreateMessagePost(t, dm, "hi there")
	reply := waitForAgentReply(t, th, agent.UserId, post.Id)
	assert.Equal(t, "Hello! How can I help?", reply.Message)
	assert.Contains(t, lastLLMRequestText(t, f), "a direct message")
}

// An agent reads the conversation as the person who asked: a link to a message in a channel they can't read adds
// nothing, so the agent never quotes it; a link they can read is quoted.
func TestAgentNeverQuotesAChannelTheAskerCantRead(t *testing.T) {
	th, f, c := setupAgentTest(t, "Here is what I found.")
	agent := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil)

	secretChannel := th.CreatePrivateChannel(t, th.BasicTeam, func(ch *model.Channel) { ch.CreatorId = th.BasicUser2.Id })
	secret := th.CreatePost(t, secretChannel, func(p *model.Post) {
		p.UserId = th.BasicUser2.Id
		p.Message = "The merger code name is BLUEBIRD."
	})
	readable := th.CreateMessagePost(t, th.BasicChannel, "The launch is on Friday.")
	siteURL := "http://localhost:8065/" + th.BasicTeam.Name + "/pl/"

	post := savePostQuietly(t, th, &model.Post{
		UserId:    th.BasicUser.Id,
		ChannelId: th.BasicChannel.Id,
		Message:   "@" + agent.Username + " compare " + siteURL + secret.Id + " with " + siteURL + readable.Id,
	})
	th.App.handlePostForAgents(th.Context, post, th.BasicChannel)

	text := lastLLMRequestText(t, f)
	assert.NotContains(t, text, "BLUEBIRD", "never the content of a channel the asker can't read")
	assert.NotContains(t, text, secretChannel.Name, "nor its name")
	assert.Contains(t, text, "The launch is on Friday.", "a message they can read is quoted")
	waitForAgentReply(t, th, agent.UserId, post.Id)
}

// The thread the agent reads is the asker's channel's: replies there, its own answers as its turns.
func TestAgentReadsTheThread(t *testing.T) {
	th, f, c := setupAgentTest(t, "Second answer.")
	agent := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil)
	agentUser := mustGetUser(t, th, agent.UserId)

	root := savePostQuietly(t, th, &model.Post{UserId: th.BasicUser.Id, ChannelId: th.BasicChannel.Id, Message: "@" + agent.Username + " plan the release", CreateAt: model.GetMillis() - 3000})
	savePostQuietly(t, th, &model.Post{UserId: agentUser.Id, ChannelId: th.BasicChannel.Id, RootId: root.Id, Message: "First answer.", CreateAt: model.GetMillis() - 2000, Props: model.StringInterface{model.PostPropsFromBot: "true"}})
	followUp := savePostQuietly(t, th, &model.Post{UserId: th.BasicUser.Id, ChannelId: th.BasicChannel.Id, RootId: root.Id, Message: "and the date?", CreateAt: model.GetMillis() - 1000})

	assert.Len(t, th.App.agentsForPost(th.Context, followUp, th.BasicChannel), 1, "a reply in a thread its asker started with the agent, mentioning nobody")
	other := &model.Post{UserId: th.BasicUser.Id, ChannelId: th.BasicChannel.Id, RootId: root.Id, Message: "@" + th.BasicUser2.Username + " what do you think?"}
	assert.Empty(t, th.App.agentsForPost(th.Context, other, th.BasicChannel), "a reply to someone else isn't for the agent")

	th.App.handlePostForAgents(th.Context, followUp, th.BasicChannel)
	messages := f.calls()[len(f.calls())-1].messages()
	require.Len(t, messages, 4)
	assert.Equal(t, "user", messages[1]["role"])
	assert.Contains(t, messages[1]["content"], "plan the release")
	assert.Equal(t, "assistant", messages[2]["role"])
	assert.Equal(t, "First answer.", messages[2]["content"])
	assert.Contains(t, messages[3]["content"], "and the date?")
}

// Who may not talk to an agent gets a notice and no model call; external agents and bots get nothing.
func TestAgentTurnsThatDontRun(t *testing.T) {
	th, f, c := setupAgentTest(t, "should not be asked")
	adminsOnly := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, func(req *model.AIAgentRequest) {
		req.Access = model.AIAgentAccess{Roles: []string{model.SystemAdminRoleId}}
	})
	external := createTestAgent(t, th, model.AIAgentScopeServer, "", func(req *model.AIAgentRequest) { req.External = true })
	everyone := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil)

	post := savePostQuietly(t, th, &model.Post{UserId: th.BasicUser.Id, ChannelId: th.BasicChannel.Id, Message: "@" + adminsOnly.Username + " hi"})
	th.App.handlePostForAgents(th.Context, post, th.BasicChannel)
	post = savePostQuietly(t, th, &model.Post{UserId: th.BasicUser.Id, ChannelId: th.BasicChannel.Id, Message: "@" + external.Username + " hi"})
	th.App.handlePostForAgents(th.Context, post, th.BasicChannel)

	bot := th.CreateBot(t)
	post = savePostQuietly(t, th, &model.Post{UserId: bot.UserId, ChannelId: th.BasicChannel.Id, Message: "@" + everyone.Username + " hi"})
	th.App.handlePostForAgents(th.Context, post, th.BasicChannel)

	disabled := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil)
	_, appErr := th.App.SetAIAgentEnabled(th.Context, disabled.AIAgent, false)
	require.Nil(t, appErr)
	post = savePostQuietly(t, th, &model.Post{UserId: th.BasicUser.Id, ChannelId: th.BasicChannel.Id, Message: "@" + disabled.Username + " hi"})
	th.App.handlePostForAgents(th.Context, post, th.BasicChannel)

	assert.Empty(t, f.calls(), "no model call")
}

// A post is answered once per agent, whatever runs it twice.
func TestAgentTurnIsClaimedOnce(t *testing.T) {
	th, f, c := setupAgentTest(t, "Once.")
	agent := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil)
	post := savePostQuietly(t, th, &model.Post{UserId: th.BasicUser.Id, ChannelId: th.BasicChannel.Id, Message: "@" + agent.Username + " hi"})
	th.App.handlePostForAgents(th.Context, post, th.BasicChannel)
	th.App.handlePostForAgents(th.Context, post, th.BasicChannel)
	assert.Len(t, f.calls(), 1)
}

// When the model fails, the agent says so in the person's language, without the endpoint's details.
func TestAgentTurnFailure(t *testing.T) {
	th, f, c := setupAgentTest(t, "")
	f.fail(http.StatusInternalServerError, "upstream exploded with key sk-aitest-0123456789abcdefghij")
	agent := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil)
	post := savePostQuietly(t, th, &model.Post{UserId: th.BasicUser.Id, ChannelId: th.BasicChannel.Id, Message: "@" + agent.Username + " hi"})
	th.App.handlePostForAgents(th.Context, post, th.BasicChannel)

	reply := waitForAgentReply(t, th, agent.UserId, post.Id)
	assert.True(t, strings.HasPrefix(reply.Message, "Sorry, I couldn't answer:"), reply.Message)
	assert.NotContains(t, reply.Message, "sk-aitest", "never a key")
	assert.NotContains(t, reply.Message, "exploded", "nor the endpoint's message")
}
