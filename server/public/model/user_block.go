// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

// Ignoring, blocking and message requests (Antimatter), as in Discord. They're on unless
// PrivacySettings.EnableUserBlocking turns them off.
//
// Whom someone ignores or blocks is one of their preferences: PreferenceCategoryUserBlock, named after the other
// person's id, worth UserBlockIgnore or UserBlockBlock. Nobody else can read it, the person ignored or blocked included.
//
//   - Ignoring someone is quiet: none of their messages notify you (mentions, replies, direct messages, by push,
//     e-mail or on the desktop), their direct messages wait as a message request, and the apps fold their messages.
//   - Blocking someone ignores them, and they can't start a direct or group message with you any more, nor post in
//     your direct message: they're told they can't message you, not why.
//
// How someone takes direct messages from people they don't know yet is a preference too: PreferenceCategoryDMPrivacy,
// named PreferenceNameDMPrivacyDefault for all communities and after a community's id where they chose otherwise
// there, worth DMPrivacyEveryone (the default), DMPrivacyAsk (they wait as message requests) or DMPrivacyRefuse (they
// can't start a direct or group message with them). Someone who shares several communities with them gets the most
// restrictive of those communities' choices, so that joining an open community never lets anyone reach them more;
// someone who shares none gets the choice for all. A conversation they accepted (or started, or answered) goes on.
//
// Whether a direct message waits as a request is kept on its recipient's channel member, in its notify props
// (ChannelMemberDMRequestProp), so that the apps can leave requests out of the unread counts: only that person sees it.
const (
	PreferenceCategoryUserBlock = "user_block"
	UserBlockIgnore             = "ignore"
	UserBlockBlock              = "block"

	PreferenceCategoryDMPrivacy    = "dm_privacy"
	PreferenceNameDMPrivacyDefault = "default"
	DMPrivacyEveryone              = "everyone"
	DMPrivacyAsk                   = "ask"
	DMPrivacyRefuse                = "refuse"

	// ChannelMemberDMRequestProp is where a direct message stands for its recipient: DMRequestAccepted, DMRequestPending
	// (a message request), DMRequestDismissed (a request they ignored, hidden until the sender writes again), or unset
	// for direct messages from before message requests, which are taken as accepted.
	ChannelMemberDMRequestProp = "dm_request"
	DMRequestAccepted          = "accepted"
	DMRequestPending           = "pending"
	DMRequestDismissed         = "dismissed"

	// ChannelMemberDMRequestDismissalsProp counts the requests of the conversation its recipient ignored, so that the
	// apps can offer to block someone whose messages they keep ignoring.
	ChannelMemberDMRequestDismissalsProp = "dm_request_dismissals"

	// The actions on a message request (POST /channels/{channel_id}/message_request): accept it, ignore it, or put a
	// direct message back among the requests (when its other person gets ignored).
	MessageRequestActionAccept  = "accept"
	MessageRequestActionDismiss = "dismiss"
	MessageRequestActionRequest = "request"
)

// MessageRequestAction is the body of POST /channels/{channel_id}/message_request.
type MessageRequestAction struct {
	Action string `json:"action"`
}

// IsValidDMPrivacy tells a value of PreferenceCategoryDMPrivacy apart from anything else.
func IsValidDMPrivacy(value string) bool {
	return value == DMPrivacyEveryone || value == DMPrivacyAsk || value == DMPrivacyRefuse
}

// DMPrivacyRank orders the choices from the most open: the most restrictive of several wins.
func DMPrivacyRank(value string) int {
	switch value {
	case DMPrivacyAsk:
		return 1
	case DMPrivacyRefuse:
		return 2
	default:
		return 0
	}
}

// IsValidUserBlockLevel tells a value of PreferenceCategoryUserBlock apart from anything else.
func IsValidUserBlockLevel(level string) bool {
	return level == UserBlockIgnore || level == UserBlockBlock
}

// IsDMRequestWaiting tells whether a direct message is a message request for the member whose notify props these are:
// waiting, or ignored and hidden.
func IsDMRequestWaiting(notifyProps StringMap) bool {
	state := notifyProps[ChannelMemberDMRequestProp]
	return state == DMRequestPending || state == DMRequestDismissed
}

// privateChannelMemberNotifyProps are the notify props that only their member may see.
var privateChannelMemberNotifyProps = []string{ChannelMemberDMRequestProp, ChannelMemberDMRequestDismissalsProp}
