// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Antimatter forums: a forum channel holds only topics, each a title and a first post (its root), with replies under
// it. Votes are 👍 reactions. In Q&A forums the asker or a moderator accepts one reply as the answer; in Ideas forums
// moderators set a status. The accepted answer and the status are props of the root that only the server writes
// (PostPropsServerOwned), through PATCH /posts/{root_id}/topic.

const (
	// PostPropsThreadTitle is a topic's title (a named thread, in a text channel), on its root post.
	PostPropsThreadTitle = "am_thread_title"
	// PostPropsAcceptedAnswer is the id of the reply accepted as a Q&A topic's answer.
	PostPropsAcceptedAnswer = "am_accepted_answer"
	// PostPropsTopicStatus is an Ideas topic's status (TopicStatusOpen when missing), with who set it and when.
	PostPropsTopicStatus   = "am_topic_status"
	PostPropsTopicStatusBy = "am_topic_status_by"
	PostPropsTopicStatusAt = "am_topic_status_at"

	// ForumTopicTitleMaxRunes is the longest a topic's title may be, trimmed.
	ForumTopicTitleMaxRunes = 100

	// The system replies a topic gets when its answer or status changes.
	PostTypeTopicAnswer = "system_am_topic_answer"
	PostTypeTopicStatus = "system_am_topic_status"
)

// IsTopicSystemReply tells whether a post is the system reply of a topic's answer or status change.
func IsTopicSystemReply(post *Post) bool {
	return post != nil && (post.Type == PostTypeTopicAnswer || post.Type == PostTypeTopicStatus)
}

// PostPropsServerOwned are the props of a topic that only the server writes: creating, updating or patching a post,
// from a client or an app, drops incoming values and keeps the stored ones.
var PostPropsServerOwned = []string{
	PostPropsAcceptedAnswer,
	PostPropsTopicStatus,
	PostPropsTopicStatusBy,
	PostPropsTopicStatusAt,
}

// Ideas topics' statuses, in their order on the board.
const (
	TopicStatusOpen       = "open"
	TopicStatusPlanned    = "planned"
	TopicStatusInProgress = "in_progress"
	TopicStatusDone       = "done"
	TopicStatusDeclined   = "declined"
)

// TopicStatuses gives the statuses an Ideas topic can have.
func TopicStatuses() []string {
	return []string{TopicStatusOpen, TopicStatusPlanned, TopicStatusInProgress, TopicStatusDone, TopicStatusDeclined}
}

// IsValidTopicStatus tells whether s is one of TopicStatuses.
func IsValidTopicStatus(s string) bool {
	return slices.Contains(TopicStatuses(), s)
}

// VoteEmojiNames are the reactions that count as a vote: 👍 under both its names (Fusion sends +1).
var VoteEmojiNames = []string{"+1", "thumbsup"}

// IsVoteEmoji tells whether a reaction's emoji is a vote.
func IsVoteEmoji(name string) bool {
	return slices.Contains(VoteEmojiNames, name)
}

// IsForum tells whether the channel type is a forum, where root posts are topics.
func (i ChannelTypeInfo) IsForum() bool {
	return i.Type == AMChannelTypeForum
}

// TopicTitle gives a post's topic title, trimmed ("" when it has none, or one that isn't text).
func (o *Post) TopicTitle() string {
	title, _ := o.GetProp(PostPropsThreadTitle).(string)
	return strings.TrimSpace(title)
}

// RemoveServerOwnedProps drops the props only the server writes (PostPropsServerOwned).
func (o *Post) RemoveServerOwnedProps() {
	if o == nil {
		return
	}
	for _, key := range PostPropsServerOwned {
		if o.GetProp(key) != nil {
			o.DelProp(key)
		}
	}
}

// KeepServerOwnedPropsFrom puts back the server-owned props of old, the stored post an update starts from, whatever
// the update brought.
func (o *Post) KeepServerOwnedPropsFrom(old *Post) {
	if o == nil {
		return
	}
	o.RemoveServerOwnedProps()
	if old == nil {
		return
	}
	for _, key := range PostPropsServerOwned {
		if v := old.GetProp(key); v != nil {
			o.AddProp(key, v)
		}
	}
}

// Topic list sorts (GET /channels/{channel_id}/topics?sort=): unanswered is for Q&A forums, status for Ideas ones.
const (
	ForumTopicSortActivity   = "activity"
	ForumTopicSortNewest     = "newest"
	ForumTopicSortVotes      = "votes"
	ForumTopicSortUnanswered = "unanswered"
	ForumTopicSortStatus     = "status"

	ForumTopicsPerPageDefault = 25
	ForumTopicsPerPageMax     = 100
	ForumTopicRepliesMax      = 5
	ForumTopicParticipantsMax = 5
)

// ForumTopicsOptions is a topic list's request, for a channel and the person asking.
type ForumTopicsOptions struct {
	ChannelId string
	UserId    string // who asks: mine and following are theirs
	Sort      string
	Answered  *bool  // Q&A: only topics with (true) or without (false) an accepted answer
	Status    string // Ideas: only topics of this status
	Mine      bool   // only topics the person opened
	Following bool   // only topics the person follows
	Query     string // a case-insensitive match on the title or the first post's message
	Before    *ForumTopicCursor
	PerPage   int
	Replies   int // the latest replies to give with each topic, 0 to ForumTopicRepliesMax
}

// ForumTopicCursor is where a page of topics ends: the sort keys of its last topic. Topics on top come first (OnTop
// 1), then by Key1, Key2 and the id, all descending.
type ForumTopicCursor struct {
	OnTop int
	Key1  int64
	Key2  int64
	Id    string
}

// String gives the cursor as the opaque text the API hands out (next_cursor, before).
func (c *ForumTopicCursor) String() string {
	if c == nil {
		return ""
	}
	raw := fmt.Sprintf("%d:%d:%d:%s", c.OnTop, c.Key1, c.Key2, c.Id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// ParseForumTopicCursor reads a cursor the API handed out; "" is none.
func ParseForumTopicCursor(s string) (*ForumTopicCursor, error) {
	if s == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor: %w", err)
	}
	parts := strings.Split(string(raw), ":")
	if len(parts) != 4 || !IsValidId(parts[3]) {
		return nil, errors.New("invalid cursor")
	}
	c := &ForumTopicCursor{Id: parts[3]}
	onTop, err := strconv.Atoi(parts[0])
	if err != nil || (onTop != 0 && onTop != 1) {
		return nil, errors.New("invalid cursor")
	}
	c.OnTop = onTop
	if c.Key1, err = strconv.ParseInt(parts[1], 10, 64); err != nil {
		return nil, fmt.Errorf("invalid cursor: %w", err)
	}
	if c.Key2, err = strconv.ParseInt(parts[2], 10, 64); err != nil {
		return nil, fmt.Errorf("invalid cursor: %w", err)
	}
	return c, nil
}

// ForumTopic is a topic as the topic list gives it. Votes don't count the author's own 👍. AcceptedAnswerId is given
// in Q&A forums and Status in Ideas ones only (the props stay on the post in other modes).
type ForumTopic struct {
	Post             *Post    `json:"post"`
	ReplyCount       int64    `json:"reply_count"`
	LastReplyAt      int64    `json:"last_reply_at"`
	ParticipantIds   []string `json:"participant_ids"`
	Votes            int64    `json:"votes"`
	AcceptedAnswerId string   `json:"accepted_answer_id,omitempty"`
	Status           string   `json:"status,omitempty"`
	OnTop            bool     `json:"on_top"`
	Replies          []*Post  `json:"replies,omitempty"`
}

// ForumTopicList is a page of topics, with the cursor of the next page ("" for the last one).
type ForumTopicList struct {
	Topics     []*ForumTopic `json:"topics"`
	NextCursor string        `json:"next_cursor"`
}

// ForumTopicPatch is what PATCH /posts/{root_id}/topic takes: the accepted answer of a Q&A topic ("" takes it back)
// and the status of an Ideas topic. A field left out stays as it is.
type ForumTopicPatch struct {
	AcceptedAnswerId *string `json:"accepted_answer_id,omitempty"`
	Status           *string `json:"status,omitempty"`
}

// ValidateForumTopicTitle checks a forum root post's title: it must be there, 1 to ForumTopicTitleMaxRunes runes once
// trimmed. Moving and importing check for a missing one first, with their own errors.
func ValidateForumTopicTitle(title string) *AppError {
	title = strings.TrimSpace(title)
	if title == "" {
		return NewAppError("ValidateForumTopicTitle", "app.post.forum.title_required.app_error", nil, "", http.StatusBadRequest)
	}
	if utf8.RuneCountInString(title) > ForumTopicTitleMaxRunes {
		return NewAppError("ValidateForumTopicTitle", "app.post.forum.title_too_long.app_error", map[string]any{"Max": ForumTopicTitleMaxRunes}, "", http.StatusBadRequest)
	}
	return nil
}
