// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// The values of OutgoingWebhook.TriggerWhen.
const (
	// The first word of a message matches a trigger word exactly.
	OutgoingWebhookTriggerWhenExactMatch = 0
	// The first word of a message starts with a trigger word.
	OutgoingWebhookTriggerWhenStartsWith = 1
	// The hook never fires on posted messages: it is an action of the message menu in the channels it covers (its
	// channel, or every public channel of its community), sending the message someone picks it for.
	OutgoingWebhookTriggerWhenMessageAction = 2
)

// MessageActionTrigger is the trigger of a message action's payload.
const MessageActionTrigger = "message_action"

type OutgoingWebhook struct {
	Id           string      `json:"id"`
	Token        string      `json:"token"`
	CreateAt     int64       `json:"create_at"`
	UpdateAt     int64       `json:"update_at"`
	DeleteAt     int64       `json:"delete_at"`
	CreatorId    string      `json:"creator_id"`
	ChannelId    string      `json:"channel_id"`
	TeamId       string      `json:"team_id"`
	TriggerWords StringArray `json:"trigger_words"`
	TriggerWhen  int         `json:"trigger_when"`
	CallbackURLs StringArray `json:"callback_urls"`
	DisplayName  string      `json:"display_name"`
	Description  string      `json:"description"`
	ContentType  string      `json:"content_type"`
	Username     string      `json:"username"`
	IconURL      string      `json:"icon_url"`
}

func (o *OutgoingWebhook) Auditable() map[string]any {
	return map[string]any{
		"id":            o.Id,
		"create_at":     o.CreateAt,
		"update_at":     o.UpdateAt,
		"delete_at":     o.DeleteAt,
		"creator_id":    o.CreatorId,
		"channel_id":    o.ChannelId,
		"team_id":       o.TeamId,
		"trigger_words": o.TriggerWords,
		"trigger_when":  o.TriggerWhen,
		"callback_urls": o.CallbackURLs,
		"display_name":  o.DisplayName,
		"description":   o.Description,
		"content_type":  o.ContentType,
		"username":      o.Username,
		"icon_url":      o.IconURL,
	}
}

type OutgoingWebhookPayload struct {
	Token       string `json:"token"`
	TeamId      string `json:"team_id"`
	TeamDomain  string `json:"team_domain"`
	ChannelId   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	Timestamp   int64  `json:"timestamp"`
	UserId      string `json:"user_id"`
	UserName    string `json:"user_name"`
	PostId      string `json:"post_id"`
	Text        string `json:"text"`
	TriggerWord string `json:"trigger_word"`
	FileIds     string `json:"file_ids"`
}

type OutgoingWebhookResponse struct {
	Text         *string              `json:"text"`
	Username     string               `json:"username"`
	IconURL      string               `json:"icon_url"`
	Props        StringInterface      `json:"props"`
	Attachments  []*MessageAttachment `json:"attachments"`
	Type         string               `json:"type"`
	ResponseType string               `json:"response_type"`
	Priority     *PostPriority        `json:"priority"`

	// EphemeralText, for message actions only, is shown to the person who picked the action alone.
	EphemeralText string `json:"ephemeral_text"`
}

const OutgoingHookResponseTypeComment = "comment"

func (o *OutgoingWebhookPayload) ToFormValues() string {
	v := url.Values{}
	v.Set("token", o.Token)
	v.Set("team_id", o.TeamId)
	v.Set("team_domain", o.TeamDomain)
	v.Set("channel_id", o.ChannelId)
	v.Set("channel_name", o.ChannelName)
	v.Set("timestamp", strconv.FormatInt(o.Timestamp/1000, 10))
	v.Set("user_id", o.UserId)
	v.Set("user_name", o.UserName)
	v.Set("post_id", o.PostId)
	v.Set("text", o.Text)
	v.Set("trigger_word", o.TriggerWord)
	v.Set("file_ids", o.FileIds)

	return v.Encode()
}

func (o *OutgoingWebhook) IsValid() *AppError {
	if !IsValidId(o.Id) {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.id.app_error", nil, "", http.StatusBadRequest)
	}

	if len(o.Token) != 26 {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.token.app_error", nil, "", http.StatusBadRequest)
	}

	if o.CreateAt == 0 {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.create_at.app_error", nil, "id="+o.Id, http.StatusBadRequest)
	}

	if o.UpdateAt == 0 {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.update_at.app_error", nil, "id="+o.Id, http.StatusBadRequest)
	}

	if !IsValidId(o.CreatorId) {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.user_id.app_error", nil, "", http.StatusBadRequest)
	}

	if o.ChannelId != "" && !IsValidId(o.ChannelId) {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.channel_id.app_error", nil, "", http.StatusBadRequest)
	}

	if !IsValidId(o.TeamId) {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.team_id.app_error", nil, "", http.StatusBadRequest)
	}

	if len(fmt.Sprintf("%s", o.TriggerWords)) > 1024 {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.words.app_error", nil, "", http.StatusBadRequest)
	}

	if len(o.TriggerWords) != 0 {
		if slices.Contains(o.TriggerWords, "") {
			return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.trigger_words.app_error", nil, "", http.StatusBadRequest)
		}
	}

	if len(o.CallbackURLs) == 0 || len(fmt.Sprintf("%s", o.CallbackURLs)) > 1024 {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.callback.app_error", nil, "", http.StatusBadRequest)
	}

	for _, callback := range o.CallbackURLs {
		if !IsValidHTTPURL(callback) {
			return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.url.app_error", nil, "", http.StatusBadRequest)
		}
	}

	if len(o.DisplayName) > 64 {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.display_name.app_error", nil, "", http.StatusBadRequest)
	}

	if len(o.Description) > 500 {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.description.app_error", nil, "", http.StatusBadRequest)
	}

	if len(o.ContentType) > 128 {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.content_type.app_error", nil, "", http.StatusBadRequest)
	}

	if o.TriggerWhen > OutgoingWebhookTriggerWhenMessageAction {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.content_type.app_error", nil, "", http.StatusBadRequest)
	}

	if o.IsMessageAction() {
		// A message action is picked from the menu by its name, and is never triggered by words.
		if len(o.TriggerWords) != 0 {
			return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.message_action_words.app_error", nil, "", http.StatusBadRequest)
		}
		if strings.TrimSpace(o.DisplayName) == "" {
			return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.message_action_name.app_error", nil, "", http.StatusBadRequest)
		}
	}

	if len(o.Username) > 64 {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.username.app_error", nil, "", http.StatusBadRequest)
	}

	if len(o.IconURL) > 1024 {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.icon_url.app_error", nil, "", http.StatusBadRequest)
	}

	return nil
}

// IsMessageAction tells whether the hook is an action of the message menu rather than triggered by posted messages.
func (o *OutgoingWebhook) IsMessageAction() bool {
	return o.TriggerWhen == OutgoingWebhookTriggerWhenMessageAction
}

// OutgoingWebhookIconPath is the API path of a hook's uploaded icon; IconURL holds it (with a version query) when the
// icon was uploaded rather than given as a URL.
func OutgoingWebhookIconPath(hookID string) string {
	return "/api/v4/hooks/outgoing/" + hookID + "/icon"
}

// HasUploadedIcon tells whether the hook's icon is the one uploaded for it.
func (o *OutgoingWebhook) HasUploadedIcon() bool {
	path := OutgoingWebhookIconPath(o.Id)
	return o.IconURL == path || strings.HasPrefix(o.IconURL, path+"?")
}

// MessageAction is what the message menu shows of a message action: never its token nor its callback URLs.
type MessageAction struct {
	Id          string `json:"id"`
	TeamId      string `json:"team_id"`
	ChannelId   string `json:"channel_id"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`

	// IconURL is the hook's uploaded icon, or an http(s) image URL; empty for none (the menu draws a neutral icon).
	IconURL string `json:"icon_url"`
}

// ToMessageAction is the hook as the message menu shows it. Only an uploaded icon or an http(s) URL is kept as its
// icon: anything else (a data: URI, another path of the server) is dropped.
func (o *OutgoingWebhook) ToMessageAction() *MessageAction {
	icon := ""
	if o.HasUploadedIcon() || IsValidHTTPURL(o.IconURL) {
		icon = o.IconURL
	}
	return &MessageAction{
		Id:          o.Id,
		TeamId:      o.TeamId,
		ChannelId:   o.ChannelId,
		DisplayName: o.DisplayName,
		Description: o.Description,
		IconURL:     icon,
	}
}

// MessageActionFile is a file of the message a message action sends: its name, and its link on the server (which
// needs a session or token to download).
type MessageActionFile struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Link string `json:"link"`
}

// MessageActionPayload is what a message action sends to its callback URLs when someone picks it for a message.
type MessageActionPayload struct {
	Token   string `json:"token"`
	Trigger string `json:"trigger"`

	// The community and channel of the message.
	TeamId      string `json:"team_id"`
	TeamDomain  string `json:"team_domain"`
	ChannelId   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`

	// When the action was picked, and by whom.
	Timestamp int64  `json:"timestamp"`
	UserId    string `json:"user_id"`
	UserName  string `json:"user_name"`

	// The message: a burn-on-read message's text and files are never sent.
	PostId       string              `json:"post_id"`
	PostRootId   string              `json:"post_root_id"`
	PostCreateAt int64               `json:"post_create_at"`
	PostUserId   string              `json:"post_user_id"`
	PostUserName string              `json:"post_user_name"`
	Permalink    string              `json:"permalink"`
	Text         string              `json:"text"`
	Files        []MessageActionFile `json:"files"`
	Priority     string              `json:"priority"`
	RequestedAck bool                `json:"requested_ack"`
	BurnOnRead   bool                `json:"burn_on_read"`
}

// ToFormValues encodes the payload for the application/x-www-form-urlencoded content type: the files as repeated
// file_name and file_link fields, in the same order.
func (p *MessageActionPayload) ToFormValues() string {
	v := url.Values{}
	v.Set("token", p.Token)
	v.Set("trigger", p.Trigger)
	v.Set("team_id", p.TeamId)
	v.Set("team_domain", p.TeamDomain)
	v.Set("channel_id", p.ChannelId)
	v.Set("channel_name", p.ChannelName)
	v.Set("timestamp", strconv.FormatInt(p.Timestamp, 10))
	v.Set("user_id", p.UserId)
	v.Set("user_name", p.UserName)
	v.Set("post_id", p.PostId)
	v.Set("post_root_id", p.PostRootId)
	v.Set("post_create_at", strconv.FormatInt(p.PostCreateAt, 10))
	v.Set("post_user_id", p.PostUserId)
	v.Set("post_user_name", p.PostUserName)
	v.Set("permalink", p.Permalink)
	v.Set("text", p.Text)
	for _, f := range p.Files {
		v.Add("file_id", f.Id)
		v.Add("file_name", f.Name)
		v.Add("file_link", f.Link)
	}
	v.Set("priority", p.Priority)
	v.Set("requested_ack", strconv.FormatBool(p.RequestedAck))
	v.Set("burn_on_read", strconv.FormatBool(p.BurnOnRead))
	return v.Encode()
}

// MessageActionResult is what the person who picked a message action is told: the hook's ephemeral_text, if any.
type MessageActionResult struct {
	EphemeralText string `json:"ephemeral_text"`
}

func (o *OutgoingWebhook) PreSave() {
	if o.Id == "" {
		o.Id = NewId()
	}

	if o.Token == "" {
		o.Token = NewId()
	}

	o.CreateAt = GetMillis()
	o.UpdateAt = o.CreateAt
}

func (o *OutgoingWebhook) PreUpdate() {
	o.UpdateAt = GetMillis()
}

func (o *OutgoingWebhook) TriggerWordExactMatch(word string) bool {
	if word == "" {
		return false
	}

	return slices.Contains(o.TriggerWords, word)
}

func (o *OutgoingWebhook) TriggerWordStartsWith(word string) bool {
	if word == "" {
		return false
	}

	for _, trigger := range o.TriggerWords {
		if strings.HasPrefix(word, trigger) {
			return true
		}
	}

	return false
}

func (o *OutgoingWebhook) GetTriggerWord(word string, isExactMatch bool) (triggerWord string) {
	if word == "" {
		return
	}

	if isExactMatch {
		for _, trigger := range o.TriggerWords {
			if trigger == word {
				triggerWord = trigger
				break
			}
		}
	} else {
		for _, trigger := range o.TriggerWords {
			if strings.HasPrefix(word, trigger) {
				triggerWord = trigger
				break
			}
		}
	}

	return triggerWord
}
