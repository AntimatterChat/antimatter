// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"
)

// Agent tasks (Antimatter) are what the AI chat and the message actions ask an agent to do with something the person
// can read: summarise a thread, find its action items or its open questions, summarise a channel (since a time) or
// answer a question about it, summarise a call's transcription. A task becomes the person's message in their direct
// message with the agent, marked with the task (PostPropAgentTask); the agent runtime answers it there like any
// message, reading the task's subject for the person, with their permissions, at each turn of that conversation.

const (
	AIAgentTaskSummarizeThread        = "summarize_thread"
	AIAgentTaskFindActionItems        = "find_action_items"
	AIAgentTaskFindOpenQuestions      = "find_open_questions"
	AIAgentTaskSummarizeChannel       = "summarize_channel"
	AIAgentTaskAskChannel             = "ask_channel"
	AIAgentTaskSummarizeTranscription = "summarize_transcription"

	// PostPropAgentTask marks the person's message that asks an agent for a task, with the task.
	PostPropAgentTask = "agent_task"

	// AIAgentTaskQuestionMaxRunes bounds the question of an ask_channel task.
	AIAgentTaskQuestionMaxRunes = 2000

	// PostTypeCallsTranscription is the post the Calls app makes with a call's transcription.
	PostTypeCallsTranscription = "custom_calls_transcription"

	// AIAgentConversationsPerPage and AIAgentConversationsMaxPerPage size a page of conversations.
	AIAgentConversationsPerPage    = 50
	AIAgentConversationsMaxPerPage = 100
	// AIAgentConversationTitleMaxRunes bounds a conversation's title (its first message's first line).
	AIAgentConversationTitleMaxRunes = 120
)

var aiAgentThreadTasks = []string{AIAgentTaskSummarizeThread, AIAgentTaskFindActionItems, AIAgentTaskFindOpenQuestions}

// AIAgentTaskRequest asks an agent for a task: on a post (a thread's tasks, a transcription) or on a channel.
type AIAgentTaskRequest struct {
	Task      string `json:"task"`
	PostId    string `json:"post_id,omitempty"`
	ChannelId string `json:"channel_id,omitempty"`
	// Since, for a channel's summary: the messages from then on (milliseconds); 0 for the latest ones.
	Since int64 `json:"since,omitempty"`
	// Question, for ask_channel.
	Question string `json:"question,omitempty"`
}

// IsThreadTask tells whether the task is about a thread (the post's).
func (r *AIAgentTaskRequest) IsThreadTask() bool {
	return slices.Contains(aiAgentThreadTasks, r.Task)
}

// IsChannelTask tells whether the task is about a channel.
func (r *AIAgentTaskRequest) IsChannelTask() bool {
	return r.Task == AIAgentTaskSummarizeChannel || r.Task == AIAgentTaskAskChannel
}

// IsValid checks a task's request: a known task, with the subject it needs and nothing else.
func (r *AIAgentTaskRequest) IsValid() *AppError {
	// The error IDs are written out, for the string extractor.
	invalid := func(id string) *AppError {
		return NewAppError("AIAgentTaskRequest.IsValid", id, nil, "", http.StatusBadRequest)
	}
	switch {
	case r.IsThreadTask() || r.Task == AIAgentTaskSummarizeTranscription:
		if !IsValidId(r.PostId) || r.ChannelId != "" || r.Since != 0 || r.Question != "" {
			return invalid("model.ai_agent_task.post_id.app_error")
		}
	case r.IsChannelTask():
		if !IsValidId(r.ChannelId) || r.PostId != "" || r.Since < 0 {
			return invalid("model.ai_agent_task.channel_id.app_error")
		}
		question := strings.TrimSpace(r.Question)
		if r.Task == AIAgentTaskAskChannel && (question == "" || utf8.RuneCountInString(question) > AIAgentTaskQuestionMaxRunes) {
			return invalid("model.ai_agent_task.question.app_error")
		}
		if r.Task == AIAgentTaskSummarizeChannel && r.Question != "" {
			return invalid("model.ai_agent_task.question.app_error")
		}
	default:
		return invalid("model.ai_agent_task.task.app_error")
	}
	return nil
}

// ToProp gives the task as it's kept on the person's message.
func (r *AIAgentTaskRequest) ToProp() map[string]any {
	prop := map[string]any{"task": r.Task}
	if r.PostId != "" {
		prop["post_id"] = r.PostId
	}
	if r.ChannelId != "" {
		prop["channel_id"] = r.ChannelId
	}
	if r.Since != 0 {
		prop["since"] = r.Since
	}
	if r.Question != "" {
		prop["question"] = strings.TrimSpace(r.Question)
	}
	return prop
}

// AIAgentTaskFromPost reads the task a message asks for, if it's a valid one.
func AIAgentTaskFromPost(post *Post) *AIAgentTaskRequest {
	if post == nil {
		return nil
	}
	prop, ok := post.GetProp(PostPropAgentTask).(map[string]any)
	if !ok {
		return nil
	}
	str := func(key string) string {
		s, _ := prop[key].(string)
		return s
	}
	r := &AIAgentTaskRequest{Task: str("task"), PostId: str("post_id"), ChannelId: str("channel_id"), Question: str("question")}
	switch since := prop["since"].(type) {
	case float64:
		r.Since = int64(since)
	case int64:
		r.Since = since
	case int:
		r.Since = int64(since)
	}
	if r.IsValid() != nil {
		return nil
	}
	return r
}

// AIAgentConversation is one of a person's conversations with an agent, as the AI chat lists them: a thread they
// started in their direct message with the agent, or a thread elsewhere the agent replied in, that they follow.
type AIAgentConversation struct {
	RootId    string `json:"root_id"`
	ChannelId string `json:"channel_id"`
	AgentId   string `json:"agent_id"`
	// Title is the first line of the thread's first message, shortened.
	Title       string `json:"title"`
	Direct      bool   `json:"direct"`
	ReplyCount  int64  `json:"reply_count"`
	CreateAt    int64  `json:"create_at"`
	LastReplyAt int64  `json:"last_reply_at"`

	// Participants are the thread's participants, for the server to tell which agent the thread is with.
	Participants StringArray `json:"-"`
}

// LastActivityAt is when the conversation last moved.
func (c *AIAgentConversation) LastActivityAt() int64 {
	return max(c.CreateAt, c.LastReplyAt)
}

// AIAgentConversationTitle makes a conversation's title from its first message: the first non-empty line, shortened.
func AIAgentConversationTitle(message string) string {
	title := ""
	for line := range strings.SplitSeq(message, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			title = line
			break
		}
	}
	if runes := []rune(title); len(runes) > AIAgentConversationTitleMaxRunes {
		title = strings.TrimSpace(string(runes[:AIAgentConversationTitleMaxRunes-1])) + "…"
	}
	return title
}

// AIAgentConversationsOptions selects a page of a person's conversations with agents (see AIAgentStore).
type AIAgentConversationsOptions struct {
	UserId string
	// DirectChannelIds are the person's direct messages with the agents.
	DirectChannelIds []string
	// AgentIds are the agents whose replies in other threads count.
	AgentIds []string
	// Before is a cursor: only conversations that last moved before it (milliseconds), 0 for the latest.
	Before int64
	Limit  int
}
