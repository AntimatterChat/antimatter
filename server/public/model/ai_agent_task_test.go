// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAIAgentTaskRequestIsValid(t *testing.T) {
	post, channel := NewId(), NewId()
	for name, tc := range map[string]struct {
		req   AIAgentTaskRequest
		valid bool
	}{
		"a thread's summary":               {AIAgentTaskRequest{Task: AIAgentTaskSummarizeThread, PostId: post}, true},
		"its action items":                 {AIAgentTaskRequest{Task: AIAgentTaskFindActionItems, PostId: post}, true},
		"its open questions":               {AIAgentTaskRequest{Task: AIAgentTaskFindOpenQuestions, PostId: post}, true},
		"a transcription's summary":        {AIAgentTaskRequest{Task: AIAgentTaskSummarizeTranscription, PostId: post}, true},
		"a channel's summary":              {AIAgentTaskRequest{Task: AIAgentTaskSummarizeChannel, ChannelId: channel}, true},
		"a channel's summary since":        {AIAgentTaskRequest{Task: AIAgentTaskSummarizeChannel, ChannelId: channel, Since: 1700000000000}, true},
		"a question about a channel":       {AIAgentTaskRequest{Task: AIAgentTaskAskChannel, ChannelId: channel, Question: "Who owns the launch?"}, true},
		"an unknown task":                  {AIAgentTaskRequest{Task: "delete_everything", PostId: post}, false},
		"no task":                          {AIAgentTaskRequest{PostId: post}, false},
		"a thread's task without its post": {AIAgentTaskRequest{Task: AIAgentTaskSummarizeThread}, false},
		"a thread's task with a channel":   {AIAgentTaskRequest{Task: AIAgentTaskSummarizeThread, PostId: post, ChannelId: channel}, false},
		"a channel's action items":         {AIAgentTaskRequest{Task: AIAgentTaskFindActionItems, ChannelId: channel, Since: 1700000000000}, true},
		"a channel's open questions":       {AIAgentTaskRequest{Task: AIAgentTaskFindOpenQuestions, ChannelId: channel}, true},
		"action items of both":             {AIAgentTaskRequest{Task: AIAgentTaskFindActionItems, PostId: post, ChannelId: channel}, false},
		"action items of neither":          {AIAgentTaskRequest{Task: AIAgentTaskFindActionItems}, false},
		"a thread's action items since":    {AIAgentTaskRequest{Task: AIAgentTaskFindActionItems, PostId: post, Since: 1}, false},
		"action items with a question":     {AIAgentTaskRequest{Task: AIAgentTaskFindOpenQuestions, ChannelId: channel, Question: "why?"}, false},
		"a bad post ID":                    {AIAgentTaskRequest{Task: AIAgentTaskSummarizeThread, PostId: "x"}, false},
		"a channel's task with a post":     {AIAgentTaskRequest{Task: AIAgentTaskSummarizeChannel, ChannelId: channel, PostId: post}, false},
		"a channel's task without one":     {AIAgentTaskRequest{Task: AIAgentTaskSummarizeChannel}, false},
		"a negative since":                 {AIAgentTaskRequest{Task: AIAgentTaskSummarizeChannel, ChannelId: channel, Since: -1}, false},
		"a summary with a question":        {AIAgentTaskRequest{Task: AIAgentTaskSummarizeChannel, ChannelId: channel, Question: "why?"}, false},
		"a question without one":           {AIAgentTaskRequest{Task: AIAgentTaskAskChannel, ChannelId: channel, Question: "  "}, false},
		"a question too long":              {AIAgentTaskRequest{Task: AIAgentTaskAskChannel, ChannelId: channel, Question: strings.Repeat("é", AIAgentTaskQuestionMaxRunes+1)}, false},
	} {
		t.Run(name, func(t *testing.T) {
			appErr := tc.req.IsValid()
			if tc.valid {
				assert.Nil(t, appErr)
			} else {
				require.NotNil(t, appErr)
				assert.Equal(t, 400, appErr.StatusCode)
			}
		})
	}
}

// The task kept on a message comes back as it was, also once the message went through JSON (numbers as float64);
// a message without a task, or with a broken one, asks for none.
func TestAIAgentTaskFromPost(t *testing.T) {
	req := &AIAgentTaskRequest{Task: AIAgentTaskAskChannel, ChannelId: NewId(), Since: 1700000000000, Question: " Who owns it? "}
	post := &Post{}
	post.AddProp(PostPropAgentTask, req.ToProp())
	got := AIAgentTaskFromPost(post)
	require.NotNil(t, got)
	assert.Equal(t, AIAgentTaskAskChannel, got.Task)
	assert.Equal(t, req.ChannelId, got.ChannelId)
	assert.Equal(t, int64(1700000000000), got.Since)
	assert.Equal(t, "Who owns it?", got.Question)

	raw, err := json.Marshal(post)
	require.NoError(t, err)
	var decoded Post
	require.NoError(t, json.Unmarshal(raw, &decoded))
	got = AIAgentTaskFromPost(&decoded)
	require.NotNil(t, got)
	assert.Equal(t, int64(1700000000000), got.Since)

	assert.Nil(t, AIAgentTaskFromPost(&Post{}))
	assert.Nil(t, AIAgentTaskFromPost(nil))
	broken := &Post{}
	broken.AddProp(PostPropAgentTask, map[string]any{"task": AIAgentTaskSummarizeThread, "post_id": "not-an-id"})
	assert.Nil(t, AIAgentTaskFromPost(broken))
	wrongShape := &Post{}
	wrongShape.AddProp(PostPropAgentTask, "summarize_thread")
	assert.Nil(t, AIAgentTaskFromPost(wrongShape))
}

func TestAIAgentConversationTitle(t *testing.T) {
	assert.Equal(t, "Summarise this thread", AIAgentConversationTitle("\n\n  Summarise this thread  \nwith details"))
	assert.Equal(t, "", AIAgentConversationTitle("   \n "))
	long := AIAgentConversationTitle(strings.Repeat("word ", 100))
	assert.Equal(t, AIAgentConversationTitleMaxRunes, len([]rune(long)))
	assert.True(t, strings.HasSuffix(long, "…"))
}

func TestAIAgentConversationLastActivity(t *testing.T) {
	assert.Equal(t, int64(20), (&AIAgentConversation{CreateAt: 10, LastReplyAt: 20}).LastActivityAt())
	assert.Equal(t, int64(10), (&AIAgentConversation{CreateAt: 10}).LastActivityAt(), "not answered yet")
}

func TestAIAgentTaskKinds(t *testing.T) {
	post, channel := NewId(), NewId()
	thread := &AIAgentTaskRequest{Task: AIAgentTaskFindActionItems, PostId: post}
	assert.True(t, thread.IsThreadTask())
	assert.False(t, thread.IsChannelTask())
	inChannel := &AIAgentTaskRequest{Task: AIAgentTaskFindOpenQuestions, ChannelId: channel}
	assert.True(t, inChannel.IsChannelTask())
	assert.False(t, inChannel.IsThreadTask())
	transcription := &AIAgentTaskRequest{Task: AIAgentTaskSummarizeTranscription, PostId: post}
	assert.False(t, transcription.IsThreadTask(), "a transcription is read from its files")
	assert.False(t, transcription.IsChannelTask())
}
