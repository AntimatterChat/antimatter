// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// FocusEmoji is the custom status emoji while focusing (🎯).
	FocusEmoji = "dart"

	// UserPropsKeyFocusUntil is the user prop the server keeps, while someone focuses, with the end of their focus
	// (milliseconds): every client sees it on the user, so they can show the focusing dot. It's a marker only: the
	// focus itself is in the UserFocus table, and the prop can't be changed through the user's profile.
	UserPropsKeyFocusUntil = "focus_until"

	// FocusDurationTomorrow focuses until 09:00 the next day, in the user's timezone.
	FocusDurationTomorrow = "tomorrow"
	FocusTomorrowHour     = 9

	FocusMinDuration = time.Minute
	FocusMaxDuration = 7 * 24 * time.Hour

	// FocusReasonMaxRunes is the custom status text's limit: the reason becomes the custom status.
	FocusReasonMaxRunes = CustomStatusTextMaxRunes

	// FocusSummaryMaxWindow bounds the window a summary looks at: the longest focus, and a day's slack.
	FocusSummaryMaxWindow = FocusMaxDuration + 24*time.Hour
	// FocusSummaryMaxChannels and FocusSummaryMaxThreads bound a summary's lists (the most recent first).
	FocusSummaryMaxChannels = 100
	FocusSummaryMaxThreads  = 50
	// FocusSummaryMaxCount bounds the count of messages per channel or thread (shown as "999+").
	FocusSummaryMaxCount = 1000

	// PreferenceCategoryFocus keeps, in PreferenceNameFocusSummarySeen, the start of the last focus whose summary the
	// user saw (or dismissed), so it's shown once across their devices.
	PreferenceCategoryFocus        = "focus"
	PreferenceNameFocusSummarySeen = "summary_seen"

	// How a focus ended, in the focus_ended event.
	FocusEndedByUser   = "user"
	FocusEndedByTimer  = "timer"
	FocusEndedByStatus = "status"
)

// UserFocus is someone's focus mode (Antimatter): Do not disturb until an end time, with a reason shown as their
// custom status (🎯 and the reason), notifications muted and their own unread badges hidden. The row stays after the
// focus ends (EndedAt set), so the summary of what they missed can be shown on the device they come back to. The
// previous status and custom status are kept to restore them at the end; they never leave the server.
type UserFocus struct {
	UserId  string `json:"user_id"`
	StartAt int64  `json:"start_at"`
	EndAt   int64  `json:"end_at"`
	Reason  string `json:"reason"`
	EndedAt int64  `json:"ended_at"`

	PrevStatus        string `json:"-"`
	PrevManual        bool   `json:"-"`
	PrevDNDEndTime    int64  `json:"-"`
	PrevCustomStatus  string `json:"-"`
	FocusCustomStatus string `json:"-"`
}

// IsActive tells whether the focus is still on.
func (f *UserFocus) IsActive() bool {
	return f != nil && f.StartAt > 0 && f.EndedAt == 0
}

// FocusState is what GET /users/me/focus answers: the current focus, or the last one (Active false, EndedAt set), or
// nothing (all zero).
type FocusState struct {
	Active  bool   `json:"active"`
	StartAt int64  `json:"start_at"`
	EndAt   int64  `json:"end_at"`
	Reason  string `json:"reason"`
	EndedAt int64  `json:"ended_at"`
}

// ToState gives the public part of a focus.
func (f *UserFocus) ToState() *FocusState {
	if f == nil {
		return &FocusState{}
	}
	return &FocusState{Active: f.IsActive(), StartAt: f.StartAt, EndAt: f.EndAt, Reason: f.Reason, EndedAt: f.EndedAt}
}

// FocusRequest starts a focus: for a Duration ("15m", "1h30m", or "tomorrow" for 09:00 the next day in the user's
// timezone), or until EndAt (milliseconds); one of the two. The reason is optional.
type FocusRequest struct {
	Duration string `json:"duration,omitempty"`
	EndAt    int64  `json:"end_at,omitempty"`
	Reason   string `json:"reason"`
}

// ResolveEnd works out when the focus ends, from now in loc (the user's timezone), and checks it's between a minute
// and a week away and that the reason fits a custom status.
func (r *FocusRequest) ResolveEnd(now time.Time, loc *time.Location) (time.Time, *AppError) {
	invalid := func(id, field string) (time.Time, *AppError) {
		return time.Time{}, NewAppError("FocusRequest.ResolveEnd", id, map[string]any{"Field": field}, "", http.StatusBadRequest)
	}
	r.Reason = strings.TrimSpace(r.Reason)
	if utf8.RuneCountInString(r.Reason) > FocusReasonMaxRunes {
		return invalid("model.focus.reason.app_error", "reason")
	}
	if (r.Duration == "") == (r.EndAt == 0) {
		return invalid("model.focus.duration_or_end.app_error", "duration")
	}

	var end time.Time
	switch {
	case r.Duration == FocusDurationTomorrow:
		if loc == nil {
			loc = time.UTC
		}
		local := now.In(loc)
		end = time.Date(local.Year(), local.Month(), local.Day()+1, FocusTomorrowHour, 0, 0, 0, loc)
	case r.Duration != "":
		d, err := time.ParseDuration(r.Duration)
		if err != nil {
			return invalid("model.focus.duration.app_error", "duration")
		}
		end = now.Add(d)
	default:
		end = time.UnixMilli(r.EndAt)
	}

	if end.Sub(now) < FocusMinDuration || end.Sub(now) > FocusMaxDuration {
		return invalid("model.focus.duration.app_error", "duration")
	}
	return end, nil
}

// FocusSummary is what someone missed while focusing, between Since and Until (milliseconds): the channels (direct
// and group messages first) and the followed threads with messages from others they haven't read, with the mentions
// waiting for them there. Only channels they're still a member of are looked at.
type FocusSummary struct {
	Since        int64                  `json:"since"`
	Until        int64                  `json:"until"`
	MentionCount int64                  `json:"mention_count"`
	Channels     []*FocusSummaryChannel `json:"channels"`
	Threads      []*FocusSummaryThread  `json:"threads"`
}

// FocusSummaryChannel is a channel with messages from others in the window, after the user last read it. Count
// stops at FocusSummaryMaxCount; MentionCount is the channel's unread mentions.
type FocusSummaryChannel struct {
	ChannelId    string      `json:"channel_id"`
	TeamId       string      `json:"team_id"`
	Type         ChannelType `json:"type"`
	Name         string      `json:"name"`
	DisplayName  string      `json:"display_name"`
	Count        int64       `json:"count"`
	MentionCount int64       `json:"mention_count"`
	LastPostAt   int64       `json:"last_post_at"`
	LastSenderId string      `json:"last_sender_id"`

	// For the server: when the user last read the channel, its latest message, and whether it's muted.
	LastViewedAt      int64  `json:"-"`
	ChannelLastPostAt int64  `json:"-"`
	MarkUnread        string `json:"-"`
}

// IsDirectOrGroup tells whether the channel is a direct or group message, listed first.
func (c *FocusSummaryChannel) IsDirectOrGroup() bool {
	return c.Type == ChannelTypeDirect || c.Type == ChannelTypeGroup
}

// FocusSummaryThread is a followed thread with replies from others in the window, after the user last read it.
type FocusSummaryThread struct {
	PostId       string `json:"post_id"`
	ChannelId    string `json:"channel_id"`
	TeamId       string `json:"team_id"`
	Count        int64  `json:"count"`
	MentionCount int64  `json:"mention_count"`
	LastReplyAt  int64  `json:"last_reply_at"`
	LastSenderId string `json:"last_sender_id"`

	LastViewedAt int64 `json:"-"`
}

// FocusWindow is a summary's window, Since to Until (milliseconds), for the summary and to mark it read.
type FocusWindow struct {
	Since int64 `json:"since"`
	Until int64 `json:"until"`
}

// IsValid checks the window is in the past or now, in order, and no longer than FocusSummaryMaxWindow.
func (w *FocusWindow) IsValid(now int64) *AppError {
	if w.Since <= 0 || w.Until <= w.Since || w.Until > now+int64(time.Minute/time.Millisecond) || w.Until-w.Since > FocusSummaryMaxWindow.Milliseconds() {
		return NewAppError("FocusWindow.IsValid", "model.focus.window.app_error", nil, "", http.StatusBadRequest)
	}
	return nil
}
