// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// The hand-over from the AI chat app: an older app (the Agents plugin) answered for its own bots, which became
// agents, and keeps doing so while it runs; the app that leaves them to the server says so in its manifest.
func TestAIChatAnswersItsAgents(t *testing.T) {
	for name, tc := range map[string]struct {
		manifest *model.Manifest
		answers  bool
	}{
		"no app":                         {nil, false},
		"the older app":                  {&model.Manifest{Id: model.AIAgentsPluginId}, true},
		"the older app, other props":     {&model.Manifest{Id: model.AIAgentsPluginId, Props: map[string]any{"x": true}}, true},
		"an app saying false":            {&model.Manifest{Id: model.AIAgentsPluginId, Props: map[string]any{AIChatAgentsInServerProp: false}}, true},
		"the side-panel app":             {&model.Manifest{Id: model.AIAgentsPluginId, Props: map[string]any{AIChatAgentsInServerProp: true}}, false},
		"a malformed prop (not boolean)": {&model.Manifest{Id: model.AIAgentsPluginId, Props: map[string]any{AIChatAgentsInServerProp: "true"}}, true},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.answers, aiChatAnswersItsAgents(tc.manifest))
		})
	}
}

const aiChatTestPlugin = `
package main

import (
	"github.com/mattermost/mattermost/server/public/plugin"
)

type MyPlugin struct {
	plugin.MattermostPlugin
}

func main() {
	plugin.ClientMain(&MyPlugin{})
}
`

// The runtime answers the AI chat's agents exactly once: not while an older AI chat app runs (it answers them
// itself), and as any agent once the side-panel app runs, which never answers in its hooks.
func TestAgentRuntimeTakesOverTheAIChatsAgents(t *testing.T) {
	for name, tc := range map[string]struct {
		manifest string
		answered bool
	}{
		"the older app runs": {`{"id": "mattermost-ai", "server": {"executable": "backend.exe"}}`, false},
		"the side-panel app runs": {
			`{"id": "mattermost-ai", "server": {"executable": "backend.exe"}, "props": {"` + AIChatAgentsInServerProp + `": true}}`, true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			th, f, c := setupAgentTest(t, "Answered by the server.")
			agent := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil)
			_, appErr := th.App.UpdateBotOwner(th.Context, agent.UserId, model.AIAgentsPluginId)
			require.Nil(t, appErr)
			setupPluginAPITest(t, aiChatTestPlugin, tc.manifest, model.AIAgentsPluginId, th.App, th.Context)
			require.True(t, th.App.GetPluginsEnvironment().IsActive(model.AIAgentsPluginId))

			post := savePostQuietly(t, th, &model.Post{UserId: th.BasicUser.Id, ChannelId: th.BasicChannel.Id, Message: "@" + agent.Username + " hello"})
			th.App.handlePostForAgents(th.Context, post, th.BasicChannel)
			th.App.handlePostForAgents(th.Context, post, th.BasicChannel)
			if !tc.answered {
				assert.Empty(t, f.calls(), "the older app answers, not the runtime")
				return
			}
			reply := waitForAgentReply(t, th, agent.UserId, post.Id)
			assert.Equal(t, "Answered by the server.", reply.Message)
			assert.Len(t, f.calls(), 1, "once, however often the post comes")
		})
	}

	t.Run("no app runs", func(t *testing.T) {
		th, f, c := setupAgentTest(t, "Answered by the server.")
		agent := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil)
		_, appErr := th.App.UpdateBotOwner(th.Context, agent.UserId, model.AIAgentsPluginId)
		require.Nil(t, appErr)
		post := savePostQuietly(t, th, &model.Post{UserId: th.BasicUser.Id, ChannelId: th.BasicChannel.Id, Message: "@" + agent.Username + " hello"})
		th.App.handlePostForAgents(th.Context, post, th.BasicChannel)
		waitForAgentReply(t, th, agent.UserId, post.Id)
		assert.Len(t, f.calls(), 1)
	})
}
