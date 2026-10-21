// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Focus mode (Antimatter) is Do not disturb with an end time, plus a marker: the reason becomes the custom status
// (🎯 and the reason, cleared at the end), notifications are muted as in Do not disturb (push, email, desktop and
// persistent notifications all skip a user in Do not disturb), and Fusion hides the user's own unread badges. It
// ends when the user ends it, when its time comes (with the Do not disturb expiry), or when the user picks another
// status; the status and custom status they had before come back (the custom status only if they didn't change it
// meanwhile), and their sessions get the focus_ended event, to show what they missed.

func (a *App) getFocus(rctx request.CTX, userID string) (*model.UserFocus, *model.AppError) {
	focus, err := a.Srv().Store().Focus().Get(RequestContextWithMaster(rctx), userID)
	if err != nil {
		var nfErr *store.ErrNotFound
		if errors.As(err, &nfErr) {
			return nil, nil
		}
		return nil, model.NewAppError("getFocus", "app.focus.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return focus, nil
}

// StartFocus turns focus mode on for someone, or, while they're focusing, changes its end and its reason.
func (a *App) StartFocus(rctx request.CTX, userID string, focusRequest *model.FocusRequest) (*model.FocusState, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableUserStatuses {
		return nil, model.NewAppError("StartFocus", "app.focus.statuses_disabled.app_error", nil, "", http.StatusNotImplemented)
	}
	user, appErr := a.GetUser(rctx, userID)
	if appErr != nil {
		return nil, appErr
	}
	if user.IsBot {
		return nil, model.NewAppError("StartFocus", "app.focus.bot.app_error", nil, "", http.StatusBadRequest)
	}

	now := time.Now()
	end, appErr := focusRequest.ResolveEnd(now, user.GetTimezoneLocation())
	if appErr != nil {
		return nil, appErr
	}
	// Lined up with the Do not disturb expiry, which runs every minute and ends both.
	endSeconds := end.Truncate(model.DNDExpiryInterval).Unix()
	focus := &model.UserFocus{UserId: userID, StartAt: now.UnixMilli(), EndAt: endSeconds * 1000, Reason: focusRequest.Reason}

	current, appErr := a.getFocus(rctx, userID)
	if appErr != nil {
		return nil, appErr
	}
	if current.IsActive() {
		// Focusing again changes the end and the reason; what comes back at the end is still what was there before.
		focus.StartAt = current.StartAt
		focus.PrevStatus, focus.PrevManual, focus.PrevDNDEndTime = current.PrevStatus, current.PrevManual, current.PrevDNDEndTime
		focus.PrevCustomStatus = current.PrevCustomStatus
	} else {
		focus.PrevStatus = model.StatusOffline
		if status, err := a.GetStatus(userID); err == nil {
			focus.PrevStatus, focus.PrevManual, focus.PrevDNDEndTime = status.Status, status.Manual, status.DNDEndTime
		}
		focus.PrevCustomStatus = user.Props[model.UserPropsKeyCustomStatus]
	}

	user.MakeNonNil()
	if *a.Config().TeamSettings.EnableCustomUserStatuses {
		customStatus := &model.CustomStatus{Emoji: model.FocusEmoji, Text: focus.Reason, Duration: "date_and_time", ExpiresAt: time.UnixMilli(focus.EndAt).UTC()}
		customStatusJSON, err := json.Marshal(customStatus)
		if err != nil {
			return nil, model.NewAppError("StartFocus", "api.marshal_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		focus.FocusCustomStatus = string(customStatusJSON)
		user.Props[model.UserPropsKeyCustomStatus] = focus.FocusCustomStatus
	}
	user.Props[model.UserPropsKeyFocusUntil] = strconv.FormatInt(focus.EndAt, 10)

	if err := a.Srv().Store().Focus().Save(rctx, focus); err != nil {
		return nil, model.NewAppError("StartFocus", "app.focus.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	// One update for the custom status and the marker: one user_updated event for everyone who sees them.
	if _, appErr := a.UpdateUser(rctx, user, true); appErr != nil {
		return nil, appErr
	}
	// Straight to the platform: the App's status setters end a focus.
	a.Srv().Platform().SetStatusDoNotDisturbTimed(userID, endSeconds)

	return focus.ToState(), nil
}

// EndFocus ends someone's focus now, bringing back their status and custom status. Ending no focus does nothing.
func (a *App) EndFocus(rctx request.CTX, userID string) (*model.FocusState, *model.AppError) {
	focus, appErr := a.getFocus(rctx, userID)
	if appErr != nil {
		return nil, appErr
	}
	if focus.IsActive() {
		a.endFocus(rctx, focus, model.FocusEndedByUser)
	}
	return focus.ToState(), nil
}

// GetFocus gives someone's current focus, or the last one, or nothing. A focus whose end came without the expiry
// ending it (statuses turned off, the status changed by a way that doesn't end it) ends here.
func (a *App) GetFocus(rctx request.CTX, userID string) (*model.FocusState, *model.AppError) {
	focus, appErr := a.getFocus(rctx, userID)
	if appErr != nil {
		return nil, appErr
	}
	if focus.IsActive() {
		if focus.EndAt <= model.GetMillis() {
			a.endFocus(rctx, focus, model.FocusEndedByTimer)
		} else if status, err := a.GetStatus(userID); err == nil && status.Status != model.StatusDnd {
			a.endFocus(rctx, focus, model.FocusEndedByStatus)
		}
	}
	return focus.ToState(), nil
}

// endFocus ends a focus once (a second caller finds it ended and does nothing), restores what it replaced and tells
// the user's sessions. The focus given gets its EndedAt.
func (a *App) endFocus(rctx request.CTX, focus *model.UserFocus, by string) {
	endedAt := model.GetMillis()
	if by == model.FocusEndedByTimer && focus.EndAt < endedAt {
		endedAt = focus.EndAt
	}
	ended, err := a.Srv().Store().Focus().MarkEnded(rctx, focus.UserId, focus.StartAt, endedAt)
	if err != nil {
		rctx.Logger().Warn("Failed to end a focus", mlog.String("user_id", focus.UserId), mlog.Err(err))
		return
	}
	if !ended {
		return
	}
	focus.EndedAt = endedAt

	// A status the user picked themself stays; otherwise the one from before the focus comes back.
	if by != model.FocusEndedByStatus {
		a.restoreStatusAfterFocus(focus, by == model.FocusEndedByTimer)
	}
	a.restoreCustomStatusAfterFocus(rctx, focus)

	message := model.NewWebSocketEvent(model.WebsocketEventFocusEnded, "", "", focus.UserId, nil, "")
	message.Add("start_at", focus.StartAt)
	message.Add("end_at", focus.EndedAt)
	message.Add("ended_by", by)
	a.Publish(message)
}

func (a *App) restoreStatusAfterFocus(focus *model.UserFocus, byTimer bool) {
	ps := a.Srv().Platform()
	userID := focus.UserId
	switch {
	case focus.PrevStatus == model.StatusDnd && focus.PrevDNDEndTime == 0:
		// Do not disturb until turned off, without the focus's end.
		status, err := ps.GetStatus(userID)
		if err != nil {
			status = &model.Status{UserId: userID}
		}
		restored := *status
		restored.Status, restored.Manual, restored.DNDEndTime = model.StatusDnd, true, 0
		ps.SaveAndBroadcastStatus(&restored)
	case focus.PrevStatus == model.StatusDnd && focus.PrevDNDEndTime > time.Now().Unix():
		ps.SetStatusDoNotDisturbTimed(userID, focus.PrevDNDEndTime)
	case focus.PrevStatus == model.StatusOutOfOffice:
		ps.SetStatusOutOfOffice(userID)
	case focus.PrevManual && focus.PrevStatus == model.StatusAway:
		ps.SetStatusAwayIfNeeded(userID, true)
	case focus.PrevManual && focus.PrevStatus == model.StatusOffline:
		ps.SetStatusOffline(userID, true, false)
	default:
		// The status was automatic. At the end of its time, the expiry already put back the status from before (not
		// manual), unless it was Do not disturb that ended meanwhile; ended by the user, they're there: online.
		if byTimer {
			if status, err := ps.GetStatus(userID); err != nil || status.Status != model.StatusDnd {
				return
			}
		}
		ps.SetStatusOnline(userID, true)
	}
}

// restoreCustomStatusAfterFocus brings back the custom status from before the focus, unless the user changed the
// focus one meanwhile (theirs stays), and takes the focus marker off. A custom status from before that expired
// meanwhile isn't brought back.
func (a *App) restoreCustomStatusAfterFocus(rctx request.CTX, focus *model.UserFocus) {
	user, appErr := a.GetUser(rctx, focus.UserId)
	if appErr != nil {
		rctx.Logger().Warn("Failed to restore the custom status after a focus", mlog.String("user_id", focus.UserId), mlog.Err(appErr))
		return
	}
	user.MakeNonNil()
	changed := false
	if focus.FocusCustomStatus != "" && user.Props[model.UserPropsKeyCustomStatus] == focus.FocusCustomStatus {
		previous := ""
		if focus.PrevCustomStatus != "" {
			var customStatus *model.CustomStatus
			if err := json.Unmarshal([]byte(focus.PrevCustomStatus), &customStatus); err == nil && customStatus != nil && (customStatus.ExpiresAt.IsZero() || customStatus.ExpiresAt.After(time.Now())) {
				previous = focus.PrevCustomStatus
			}
		}
		user.Props[model.UserPropsKeyCustomStatus] = previous
		changed = true
	}
	if _, ok := user.Props[model.UserPropsKeyFocusUntil]; ok {
		delete(user.Props, model.UserPropsKeyFocusUntil)
		changed = true
	}
	if !changed {
		return
	}
	if _, appErr := a.UpdateUser(rctx, user, true); appErr != nil {
		rctx.Logger().Warn("Failed to restore the custom status after a focus", mlog.String("user_id", focus.UserId), mlog.Err(appErr))
	}
}

// endFocusesAfterDND ends the focuses whose Do not disturb just expired (the timer): the expiry job gives the
// statuses it changed.
func (a *App) endFocusesAfterDND(statuses []*model.Status) {
	if len(statuses) == 0 {
		return
	}
	rctx := request.EmptyContext(a.Log())
	for _, status := range statuses {
		focus, appErr := a.getFocus(rctx, status.UserId)
		if appErr != nil {
			rctx.Logger().Warn("Failed to get a focus after Do not disturb expired", mlog.String("user_id", status.UserId), mlog.Err(appErr))
			continue
		}
		if focus.IsActive() {
			a.endFocus(rctx, focus, model.FocusEndedByTimer)
		}
	}
}

// endFocusAfterStatusChange ends someone's focus when they pick a status themself (another one, or Do not disturb
// on its own): the status they picked stays.
func (a *App) endFocusAfterStatusChange(userID string) {
	rctx := request.EmptyContext(a.Log())
	focus, appErr := a.getFocus(rctx, userID)
	if appErr != nil || !focus.IsActive() {
		return
	}
	a.endFocus(rctx, focus, model.FocusEndedByStatus)
}

// keepFocusMarker keeps the focus marker the server set when someone's profile is saved with their props: only the
// server sets or clears it.
func keepFocusMarker(previous, user *model.User) {
	if previous == nil || user == nil {
		return
	}
	marker, had := previous.Props[model.UserPropsKeyFocusUntil]
	_, has := user.Props[model.UserPropsKeyFocusUntil]
	if !had && !has {
		return
	}
	user.MakeNonNil()
	if had {
		user.Props[model.UserPropsKeyFocusUntil] = marker
	} else {
		delete(user.Props, model.UserPropsKeyFocusUntil)
	}
}

// GetFocusSummary gives what someone missed between since and until: their channels (direct and group messages first,
// then those with mentions) and, with collapsed threads, the threads they follow, with messages from others they
// haven't read. Muted channels only show when they mention them.
func (a *App) GetFocusSummary(rctx request.CTX, userID string, window *model.FocusWindow) (*model.FocusSummary, *model.AppError) {
	if appErr := window.IsValid(model.GetMillis()); appErr != nil {
		return nil, appErr
	}
	collapsed := a.IsCRTEnabledForUser(rctx, userID)

	channels, err := a.Srv().Store().Focus().GetMissedChannels(rctx, userID, window.Since, window.Until, collapsed, model.FocusSummaryMaxChannels)
	if err != nil {
		return nil, model.NewAppError("GetFocusSummary", "app.focus.summary.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	threads := []*model.FocusSummaryThread{}
	if collapsed {
		threads, err = a.Srv().Store().Focus().GetMissedThreads(rctx, userID, window.Since, window.Until, model.FocusSummaryMaxThreads)
		if err != nil {
			return nil, model.NewAppError("GetFocusSummary", "app.focus.summary.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
	}

	summary := &model.FocusSummary{Since: window.Since, Until: window.Until, Channels: []*model.FocusSummaryChannel{}, Threads: threads}
	for _, channel := range channels {
		if channel.MarkUnread == model.ChannelMarkUnreadMention && channel.MentionCount == 0 {
			continue
		}
		summary.Channels = append(summary.Channels, channel)
		summary.MentionCount += channel.MentionCount
	}
	for _, thread := range threads {
		summary.MentionCount += thread.MentionCount
	}
	// The store gives the most recent first; keep that order within each group.
	rank := func(c *model.FocusSummaryChannel) int {
		switch {
		case c.IsDirectOrGroup():
			return 0
		case c.MentionCount > 0:
			return 1
		default:
			return 2
		}
	}
	slices.SortStableFunc(summary.Channels, func(x, y *model.FocusSummaryChannel) int { return rank(x) - rank(y) })
	return summary, nil
}

// MarkFocusSummaryRead marks read what a summary lists, up to its end only: what came after stays unread.
func (a *App) MarkFocusSummaryRead(rctx request.CTX, userID, sessionID string, window *model.FocusWindow) *model.AppError {
	summary, appErr := a.GetFocusSummary(rctx, userID, window)
	if appErr != nil {
		return appErr
	}
	collapsed := a.IsCRTEnabledForUser(rctx, userID)

	toView := []string{}
	partly := []string{}
	for _, channel := range summary.Channels {
		if channel.LastViewedAt >= window.Until {
			continue
		}
		if channel.ChannelLastPostAt <= window.Until {
			toView = append(toView, channel.ChannelId)
		} else {
			partly = append(partly, channel.ChannelId)
		}
	}
	// A channel with messages after the end is read up to the first of them, as "mark as unread" from it.
	firsts, err := a.Srv().Store().Focus().GetFirstPostsAfter(rctx, partly, window.Until, collapsed)
	if err != nil {
		return model.NewAppError("MarkFocusSummaryRead", "app.focus.mark_read.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	for _, channelID := range partly {
		postID, ok := firsts[channelID]
		if !ok {
			toView = append(toView, channelID)
			continue
		}
		if _, appErr := a.MarkChannelAsUnreadFromPost(rctx, postID, userID, true); appErr != nil {
			return appErr
		}
	}
	if len(toView) > 0 {
		if _, appErr := a.MarkChannelsAsViewed(rctx, toView, userID, sessionID, true, collapsed); appErr != nil {
			return appErr
		}
	}
	for _, thread := range summary.Threads {
		if thread.LastViewedAt >= window.Until {
			continue
		}
		if _, appErr := a.UpdateThreadReadForUser(rctx, sessionID, userID, thread.TeamId, thread.PostId, window.Until); appErr != nil {
			return appErr
		}
	}
	return nil
}
