// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"database/sql"
	"errors"
	"maps"
	"net/http"
	"strconv"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Ignoring, blocking and message requests (Antimatter): see model/user_block.go for what they are and where they're
// kept. The server enforces them, so that push notifications and e-mails, which no app filters, follow them too.

// UserBlockingEnabled tells whether people can ignore and block each other and get message requests
// (PrivacySettings.EnableUserBlocking). Off, nothing of it is enforced, but what people chose is kept.
func (a *App) UserBlockingEnabled() bool {
	return *a.Config().PrivacySettings.EnableUserBlocking
}

// GetUserBlockLevel tells whether userID ignores or blocks otherUserID: model.UserBlockIgnore, model.UserBlockBlock,
// or "" for neither.
func (a *App) GetUserBlockLevel(userID, otherUserID string) (string, *model.AppError) {
	pref, err := a.Srv().Store().Preference().Get(userID, model.PreferenceCategoryUserBlock, otherUserID)
	if err != nil {
		var nfErr *store.ErrNotFound
		if errors.As(err, &nfErr) || errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", model.NewAppError("GetUserBlockLevel", "app.preference.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if !model.IsValidUserBlockLevel(pref.Value) {
		return "", nil
	}
	return pref.Value, nil
}

// usersIgnoring maps the people who ignore or block userID to how they do: one query, by the preferences' name.
func (a *App) usersIgnoring(userID string) (map[string]string, error) {
	prefs, err := a.Srv().Store().Preference().GetCategoryAndName(model.PreferenceCategoryUserBlock, userID)
	if err != nil {
		return nil, err
	}
	levels := make(map[string]string, len(prefs))
	for _, pref := range prefs {
		if model.IsValidUserBlockLevel(pref.Value) {
			levels[pref.UserId] = pref.Value
		}
	}
	return levels, nil
}

// checkUserBlocks tells whether senderID may write to recipientID directly: not when the recipient blocks them (they're
// told they can't message this person, not why), nor when they block the recipient themselves. System admins always
// reach people, for moderation and notices.
func (a *App) checkUserBlocks(rctx request.CTX, senderID, recipientID string) *model.AppError {
	if !a.UserBlockingEnabled() || senderID == recipientID || recipientID == "" {
		return nil
	}

	level, appErr := a.GetUserBlockLevel(recipientID, senderID)
	if appErr != nil {
		return appErr
	}
	if level == model.UserBlockBlock {
		sender, err := a.GetUser(rctx, senderID)
		if err != nil {
			return err
		}
		if !sender.IsSystemAdmin() {
			return model.NewAppError("checkUserBlocks", "app.user_block.blocked.app_error", nil, "", http.StatusForbidden)
		}
	}

	level, appErr = a.GetUserBlockLevel(senderID, recipientID)
	if appErr != nil {
		return appErr
	}
	if level == model.UserBlockBlock {
		return model.NewAppError("checkUserBlocks", "app.user_block.you_blocked.app_error", nil, "", http.StatusForbidden)
	}
	return nil
}

// checkGroupMessageBlocks tells whether creatorID may start a group message with userIDs: not when one of them blocks
// them. A group message someone else starts with both of them is allowed, as in Discord: the one who blocks sees the
// other's messages folded and isn't notified of them.
func (a *App) checkGroupMessageBlocks(rctx request.CTX, creatorID string, userIDs []string) *model.AppError {
	if !a.UserBlockingEnabled() || creatorID == "" {
		return nil
	}
	levels, err := a.usersIgnoring(creatorID)
	if err != nil {
		return model.NewAppError("checkGroupMessageBlocks", "app.preference.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	for _, id := range userIDs {
		if id != creatorID && levels[id] == model.UserBlockBlock {
			creator, appErr := a.GetUser(rctx, creatorID)
			if appErr != nil {
				return appErr
			}
			if creator.IsSystemAdmin() {
				return nil
			}
			return model.NewAppError("checkGroupMessageBlocks", "app.user_block.group_blocked.app_error", nil, "", http.StatusForbidden)
		}
	}
	return nil
}

// wantsMessageRequestFrom tells whether a direct message from sender, whom recipient doesn't talk with yet, waits as a
// message request. It does when recipient asks first (model.DMPrivacyAsk) in every community they share with sender,
// each community taking their choice for it or else their choice for all; with no community in common, their choice
// for all decides. Bots and system admins always get through: notices and moderation must reach people.
func (a *App) wantsMessageRequestFrom(recipientID string, sender *model.User) (bool, error) {
	if sender.IsBot || sender.IsSystemAdmin() {
		return false, nil
	}
	prefs, err := a.Srv().Store().Preference().GetCategory(recipientID, model.PreferenceCategoryDMPrivacy)
	if err != nil {
		return false, err
	}
	all := model.DMPrivacyEveryone
	communities := map[string]string{}
	anyAsk := false
	for _, pref := range prefs {
		if pref.Name == model.PreferenceNameDMPrivacyDefault {
			all = pref.Value
		} else {
			communities[pref.Name] = pref.Value
		}
		anyAsk = anyAsk || pref.Value == model.DMPrivacyAsk
	}
	if !anyAsk {
		return false, nil
	}

	common, err := a.Srv().Store().Team().GetCommonTeamIDsForTwoUsers(recipientID, sender.Id)
	if err != nil {
		return false, err
	}
	if len(common) == 0 {
		return all == model.DMPrivacyAsk, nil
	}
	for _, teamID := range common {
		choice, ok := communities[teamID]
		if !ok {
			choice = all
		}
		if choice != model.DMPrivacyAsk {
			return false, nil
		}
	}
	return true, nil
}

// setDMRequestState writes where a direct message stands for one of its members, and tells their apps.
func (a *App) setDMRequestState(rctx request.CTX, channelID, userID string, props map[string]string) (*model.ChannelMember, *model.AppError) {
	member, err := a.Srv().Store().Channel().UpdateMemberNotifyProps(channelID, userID, props)
	if err != nil {
		var nfErr *store.ErrNotFound
		if errors.As(err, &nfErr) {
			return nil, model.NewAppError("setDMRequestState", MissingChannelMemberError, nil, "", http.StatusNotFound).Wrap(err)
		}
		return nil, model.NewAppError("setDMRequestState", "app.channel.update_member.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	a.Srv().Platform().InvalidateChannelCacheForUser(userID)
	a.invalidateCacheForChannelMembersNotifyProps(channelID)
	if err := a.sendUpdateChannelMemberEvent(member); err != nil {
		rctx.Logger().Warn("Failed to send the channel member update of a message request", mlog.String("channel_id", channelID), mlog.Err(err))
	}
	return member, nil
}

// startDirectChannelRequest decides, as creatorID starts a direct message with otherUserID, whether it waits as a
// message request for otherUserID; it's accepted for its creator.
func (a *App) startDirectChannelRequest(rctx request.CTX, channel *model.Channel, creatorID, otherUserID string) {
	if !a.UserBlockingEnabled() || creatorID == otherUserID || channel.Type != model.ChannelTypeDirect {
		return
	}
	creator, appErr := a.GetUser(rctx, creatorID)
	if appErr != nil {
		rctx.Logger().Warn("Failed to get the creator of a direct message", mlog.String("channel_id", channel.Id), mlog.Err(appErr))
		return
	}
	state, err := a.dmRequestStateFor(otherUserID, creator)
	if err != nil {
		rctx.Logger().Warn("Failed to decide whether a direct message is a message request", mlog.String("channel_id", channel.Id), mlog.Err(err))
		return
	}
	if _, appErr := a.setDMRequestState(rctx, channel.Id, creatorID, map[string]string{model.ChannelMemberDMRequestProp: model.DMRequestAccepted}); appErr != nil {
		rctx.Logger().Warn("Failed to accept a direct message for its creator", mlog.String("channel_id", channel.Id), mlog.Err(appErr))
	}
	if _, appErr := a.setDMRequestState(rctx, channel.Id, otherUserID, map[string]string{model.ChannelMemberDMRequestProp: state}); appErr != nil {
		rctx.Logger().Warn("Failed to set whether a direct message is a message request", mlog.String("channel_id", channel.Id), mlog.Err(appErr))
	}
}

// dmRequestStateFor tells where a new direct message from sender stands for recipient: a request when they ignore or
// block sender, or ask first for people like them; else accepted.
func (a *App) dmRequestStateFor(recipientID string, sender *model.User) (string, error) {
	level, appErr := a.GetUserBlockLevel(recipientID, sender.Id)
	if appErr != nil {
		return "", appErr
	}
	if level != "" {
		return model.DMRequestPending, nil
	}
	ask, err := a.wantsMessageRequestFrom(recipientID, sender)
	if err != nil {
		return "", err
	}
	if ask {
		return model.DMRequestPending, nil
	}
	return model.DMRequestAccepted, nil
}

// updateDMRequestsForPost keeps where a direct message stands up to date as someone writes in it: replying to a
// request accepts it; a message from someone ignored makes it a request again, and a request ignored before comes
// back with a new message. A direct message from before message requests is accepted when its recipient started it or
// opened it already, and else decided as a new one. It returns the notify props of the channel's members, a copy
// updated when they changed.
func (a *App) updateDMRequestsForPost(rctx request.CTX, post *model.Post, channel *model.Channel, sender *model.User, ignoring map[string]string, notifyProps map[string]model.StringMap) map[string]model.StringMap {
	if !a.UserBlockingEnabled() || channel.Type != model.ChannelTypeDirect || post.IsSystemMessage() || post.Type == model.PostTypeAutoResponder {
		return notifyProps
	}
	recipientID := channel.GetOtherUserIdForDM(sender.Id)
	if recipientID == "" || recipientID == sender.Id {
		return notifyProps
	}

	updated := notifyProps
	copied := false
	set := func(userID, state string) {
		member, appErr := a.setDMRequestState(rctx, channel.Id, userID, map[string]string{model.ChannelMemberDMRequestProp: state})
		if appErr != nil {
			rctx.Logger().Warn("Failed to update a message request", mlog.String("channel_id", channel.Id), mlog.String("user_id", userID), mlog.Err(appErr))
			return
		}
		// The map may be shared with the cache: change a copy.
		if !copied {
			updated = maps.Clone(notifyProps)
			copied = true
		}
		updated[userID] = member.NotifyProps
	}

	if model.IsDMRequestWaiting(notifyProps[sender.Id]) {
		set(sender.Id, model.DMRequestAccepted)
	}

	state := notifyProps[recipientID][model.ChannelMemberDMRequestProp]
	next := state
	switch state {
	case model.DMRequestPending:
	case model.DMRequestDismissed:
		next = model.DMRequestPending
	case model.DMRequestAccepted:
		if ignoring[recipientID] != "" {
			next = model.DMRequestPending
		}
	default:
		member, appErr := a.GetChannelMember(rctx, channel.Id, recipientID)
		if appErr != nil {
			rctx.Logger().Warn("Failed to get the recipient of a direct message", mlog.String("channel_id", channel.Id), mlog.Err(appErr))
			return updated
		}
		switch {
		case ignoring[recipientID] != "":
			next = model.DMRequestPending
		case member.LastViewedAt > 0 || channel.CreatorId == recipientID:
			next = model.DMRequestAccepted
		default:
			ask, err := a.wantsMessageRequestFrom(recipientID, sender)
			if err != nil {
				rctx.Logger().Warn("Failed to decide whether a direct message is a message request", mlog.String("channel_id", channel.Id), mlog.Err(err))
				return updated
			}
			next = model.DMRequestAccepted
			if ask {
				next = model.DMRequestPending
			}
		}
	}
	if next != state {
		set(recipientID, next)
	}
	return updated
}

// silencedRecipients lists the people of a channel who get no notification for a post: those who ignore or block its
// author, and the recipient of a direct message waiting as a message request.
func silencedRecipients(post *model.Post, channel *model.Channel, ignoring map[string]string, notifyProps map[string]model.StringMap) model.StringSet {
	silenced := make(model.StringSet, len(ignoring)+1)
	for id := range ignoring {
		silenced.Add(id)
	}
	if channel.Type == model.ChannelTypeDirect {
		if recipientID := channel.GetOtherUserIdForDM(post.UserId); recipientID != "" && model.IsDMRequestWaiting(notifyProps[recipientID]) {
			silenced.Add(recipientID)
		}
	}
	return silenced
}

// notificationSilenced tells whether userID wants no notification from senderID in a channel: they ignore or block
// them, or it's a direct message waiting as a message request. For notifications sent outside of posts, such as the
// ringing of a call.
func (a *App) notificationSilenced(rctx request.CTX, userID, senderID, channelID string) bool {
	if !a.UserBlockingEnabled() || senderID == "" || senderID == userID {
		return false
	}
	if level, appErr := a.GetUserBlockLevel(userID, senderID); appErr == nil && level != "" {
		return true
	}
	if channelID == "" {
		return false
	}
	member, appErr := a.GetChannelMember(rctx, channelID, userID)
	return appErr == nil && model.IsDMRequestWaiting(member.NotifyProps)
}

// UpdateMessageRequest acts on a direct message for one of its members (model.MessageRequestAction*): accepting it
// takes it out of the message requests, ignoring it hides it until the other person writes again (counted, so that
// the apps can offer to block them), and putting it back among the requests is for when the other person gets ignored.
func (a *App) UpdateMessageRequest(rctx request.CTX, channel *model.Channel, userID, action string) (*model.ChannelMember, *model.AppError) {
	if !a.UserBlockingEnabled() {
		return nil, model.NewAppError("UpdateMessageRequest", "app.message_request.disabled.app_error", nil, "", http.StatusNotImplemented)
	}
	if channel.Type != model.ChannelTypeDirect {
		return nil, model.NewAppError("UpdateMessageRequest", "app.message_request.not_direct.app_error", nil, "", http.StatusBadRequest)
	}
	member, appErr := a.GetChannelMember(rctx, channel.Id, userID)
	if appErr != nil {
		return nil, appErr
	}

	props := map[string]string{}
	switch action {
	case model.MessageRequestActionAccept:
		props[model.ChannelMemberDMRequestProp] = model.DMRequestAccepted
		props[model.ChannelMemberDMRequestDismissalsProp] = "0"
	case model.MessageRequestActionDismiss:
		props[model.ChannelMemberDMRequestProp] = model.DMRequestDismissed
		dismissals, _ := strconv.Atoi(member.NotifyProps[model.ChannelMemberDMRequestDismissalsProp])
		props[model.ChannelMemberDMRequestDismissalsProp] = strconv.Itoa(max(dismissals, 0) + 1)
	case model.MessageRequestActionRequest:
		if model.IsDMRequestWaiting(member.NotifyProps) {
			return member, nil
		}
		props[model.ChannelMemberDMRequestProp] = model.DMRequestPending
	default:
		return nil, model.NewAppError("UpdateMessageRequest", "app.message_request.action.app_error", nil, "action="+action, http.StatusBadRequest)
	}
	return a.setDMRequestState(rctx, channel.Id, userID, props)
}
