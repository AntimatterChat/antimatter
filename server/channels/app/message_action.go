// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/throttled/throttled/v2"
	"github.com/throttled/throttled/v2/store/memstore"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app/imaging"
)

// Message actions (Antimatter) are outgoing webhooks triggered from the message menu
// (model.OutgoingWebhookTriggerWhenMessageAction): someone picks one for a message they can read, and the server
// sends that message to the hook's callback URLs. They cover the channels outgoing webhooks cover: the hook's
// channel, or every public channel of its community.

const (
	// How often each person may run message actions: a burst, then one every 6 seconds.
	messageActionsPerMinute = 10
	messageActionsBurst     = 5

	// The people whose recent runs are remembered for the limit.
	messageActionLimiterKeys = 10000

	// The longest ephemeral_text shown to the person who ran the action.
	maxMessageActionEphemeralRunes = 300
)

// messageActionLimiter limits how often each person runs message actions. It is built on first use.
type messageActionLimiter struct {
	once    sync.Once
	limiter *throttled.GCRARateLimiterCtx
	err     error
}

func (l *messageActionLimiter) allow(ctx context.Context, userID string) (bool, error) {
	l.once.Do(func() {
		store, err := memstore.NewCtx(messageActionLimiterKeys)
		if err != nil {
			l.err = err
			return
		}
		l.limiter, l.err = throttled.NewGCRARateLimiterCtx(store, throttled.RateQuota{
			MaxRate:  throttled.PerMin(messageActionsPerMinute),
			MaxBurst: messageActionsBurst,
		})
	})
	if l.err != nil {
		return false, l.err
	}
	limited, _, err := l.limiter.RateLimitCtx(ctx, userID, 1)
	if err != nil {
		return false, err
	}
	return !limited, nil
}

// messageActionCovers tells whether hook is a message action that user may pick for the messages of channel: the
// hook's own channel, or any public channel of its community when it has none. Guests only get the actions bound to
// a channel they are in: a community-wide action was not set up for them.
func messageActionCovers(hook *model.OutgoingWebhook, channel *model.Channel, user *model.User) bool {
	if hook == nil || channel == nil || user == nil || !hook.IsMessageAction() || hook.DeleteAt != 0 {
		return false
	}
	if channel.TeamId == "" || channel.TeamId != hook.TeamId || channel.Type != model.ChannelTypeOpen {
		return false
	}
	if hook.ChannelId != "" {
		return hook.ChannelId == channel.Id
	}
	return !user.IsGuest()
}

// GetMessageActionsForChannel returns the message actions user may pick for the messages of channel, by name. The
// caller has checked that user can read the channel.
func (a *App) GetMessageActionsForChannel(rctx request.CTX, channel *model.Channel, user *model.User) ([]*model.MessageAction, *model.AppError) {
	actions := []*model.MessageAction{}
	if !*a.Config().ServiceSettings.EnableOutgoingWebhooks || channel.TeamId == "" || channel.Type != model.ChannelTypeOpen {
		return actions, nil
	}

	hooks, err := a.Srv().Store().Webhook().GetOutgoingByTeam(channel.TeamId, -1, -1)
	if err != nil {
		return nil, model.NewAppError("GetMessageActionsForChannel", "app.webhooks.get_outgoing_by_team.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	for _, hook := range hooks {
		if messageActionCovers(hook, channel, user) {
			actions = append(actions, hook.ToMessageAction())
		}
	}
	slices.SortStableFunc(actions, func(x, y *model.MessageAction) int {
		return strings.Compare(strings.ToLower(x.DisplayName), strings.ToLower(y.DisplayName))
	})
	return actions, nil
}

// RunMessageAction sends post to the callback URLs of the message action hookID, picked by user. The caller has
// checked that user can read post (in channel). The message is sent even in a read-only or archived channel: it only
// goes out. Only that message is sent (never the rest of its thread nor other channels), without the text and files
// of a burn-on-read message, and without the files user may not download.
func (a *App) RunMessageAction(rctx request.CTX, hookID string, post *model.Post, channel *model.Channel, user *model.User) (*model.MessageActionResult, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableOutgoingWebhooks {
		return nil, model.NewAppError("RunMessageAction", "api.outgoing_webhook.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	hook, err := a.Srv().Store().Webhook().GetOutgoing(hookID)
	if err != nil || !messageActionCovers(hook, channel, user) || post.ChannelId != channel.Id {
		// The same answer for a hook that doesn't exist and one this person may not use here.
		return nil, model.NewAppError("RunMessageAction", "app.message_action.not_found.app_error", nil, "", http.StatusNotFound).Wrap(err)
	}

	if post.IsSystemMessage() || post.Type == model.PostTypeEphemeral {
		return nil, model.NewAppError("RunMessageAction", "app.message_action.system_message.app_error", nil, "", http.StatusBadRequest)
	}

	allowed, limitErr := a.Srv().messageActions.allow(rctx.Context(), user.Id)
	if limitErr != nil {
		return nil, model.NewAppError("RunMessageAction", "app.message_action.rate_limit.app_error", nil, "", http.StatusInternalServerError).Wrap(limitErr)
	}
	if !allowed {
		return nil, model.NewAppError("RunMessageAction", "app.message_action.rate_limited.app_error", nil, "", http.StatusTooManyRequests)
	}

	team, appErr := a.GetTeam(channel.TeamId)
	if appErr != nil {
		return nil, appErr
	}

	payload, appErr := a.buildMessageActionPayload(rctx, hook, post, channel, team, user)
	if appErr != nil {
		return nil, appErr
	}

	responses, sent := a.sendMessageAction(rctx, hook, payload)
	if sent == 0 {
		return nil, model.NewAppError("RunMessageAction", "app.message_action.failed.app_error", map[string]any{"Name": hook.DisplayName}, "", http.StatusBadGateway)
	}
	a.RecordPostDeliveryToWebhook(rctx, hook.Id, post)

	result := &model.MessageActionResult{}
	for _, resp := range responses {
		if resp == nil {
			continue
		}
		if result.EphemeralText == "" && strings.TrimSpace(resp.EphemeralText) != "" {
			result.EphemeralText = truncateMessageActionText(strings.TrimSpace(resp.EphemeralText), maxMessageActionEphemeralRunes)
		}
		a.answerMessageAction(rctx, hook, post, channel, user, resp)
	}
	return result, nil
}

// buildMessageActionPayload is what a message action sends of post.
func (a *App) buildMessageActionPayload(rctx request.CTX, hook *model.OutgoingWebhook, post *model.Post, channel *model.Channel, team *model.Team, user *model.User) (*model.MessageActionPayload, *model.AppError) {
	siteURL := strings.TrimSuffix(*a.Config().ServiceSettings.SiteURL, "/")
	payload := &model.MessageActionPayload{
		Token:        hook.Token,
		Trigger:      model.MessageActionTrigger,
		TeamId:       team.Id,
		TeamDomain:   team.Name,
		ChannelId:    channel.Id,
		ChannelName:  channel.Name,
		Timestamp:    model.GetMillis(),
		UserId:       user.Id,
		UserName:     user.Username,
		PostId:       post.Id,
		PostRootId:   post.RootId,
		PostCreateAt: post.CreateAt,
		PostUserId:   post.UserId,
		Permalink:    siteURL + "/" + team.Name + "/pl/" + post.Id,
		Files:        []model.MessageActionFile{},
		BurnOnRead:   post.Type == model.PostTypeBurnOnRead,
	}

	if author, err := a.GetUser(rctx, post.UserId); err == nil {
		payload.PostUserName = author.Username
	}

	if priority, err := a.GetPriorityForPost(post.Id); err == nil && priority != nil {
		if priority.Priority != nil {
			payload.Priority = *priority.Priority
		}
		if priority.RequestedAck != nil {
			payload.RequestedAck = *priority.RequestedAck
		}
	}

	// A burn-on-read message is gone once read: its text and files never leave the server.
	if payload.BurnOnRead {
		return payload, nil
	}

	payload.Text = post.Message
	if len(post.FileIds) > 0 && a.hasFileAttachmentAccess(rctx, user.Id, channel.Id) {
		infos, err := a.GetFileInfosForPost(rctx, post, false, false)
		if err != nil {
			return nil, err
		}
		for _, info := range infos {
			// Only the files of this message, in this channel.
			if info.PostId != post.Id || (info.ChannelId != "" && info.ChannelId != channel.Id) {
				continue
			}
			payload.Files = append(payload.Files, model.MessageActionFile{
				Id:   info.Id,
				Name: info.Name,
				Link: siteURL + "/api/v4/files/" + info.Id,
			})
		}
	}
	return payload, nil
}

// sendMessageAction posts the payload to each callback URL of the hook, through the client for untrusted URLs
// (internal addresses are refused unless ServiceSettings.AllowedUntrustedInternalConnections lists them). It returns
// the answers, and how many URLs took the payload.
func (a *App) sendMessageAction(rctx request.CTX, hook *model.OutgoingWebhook, payload *model.MessageActionPayload) ([]*model.OutgoingWebhookResponse, int) {
	logger := rctx.Logger().With(mlog.String("outgoing_webhook_id", hook.Id), mlog.String("post_id", payload.PostId), mlog.String("channel_id", payload.ChannelId))

	contentType := "application/x-www-form-urlencoded"
	var body []byte
	if hook.ContentType == "application/json" {
		contentType = "application/json"
		js, err := json.Marshal(payload)
		if err != nil {
			logger.Warn("Failed to encode the message action to JSON", mlog.Err(err))
			return nil, 0
		}
		body = js
	} else {
		body = []byte(payload.ToFormValues())
	}

	responses := make([]*model.OutgoingWebhookResponse, len(hook.CallbackURLs))
	sent := make([]bool, len(hook.CallbackURLs))
	var wg sync.WaitGroup
	for i, callbackURL := range hook.CallbackURLs {
		wg.Go(func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Recovered from panic in a message action request", mlog.Any("panic", r))
				}
			}()

			var accessToken *model.OutgoingOAuthConnectionToken
			if model.SafeDereference(a.Config().ServiceSettings.EnableOutgoingOAuthConnections) && a.OutgoingOAuthConnections() != nil {
				connection, err := a.OutgoingOAuthConnections().GetConnectionForAudience(rctx, callbackURL)
				if err != nil {
					logger.Error("Failed to find an outgoing OAuth connection for the message action", mlog.Err(err))
					return
				}
				if connection != nil {
					accessToken, err = a.OutgoingOAuthConnections().RetrieveTokenForConnection(rctx, connection)
					if err != nil {
						logger.Error("Failed to retrieve the token of the outgoing OAuth connection", mlog.Err(err))
						return
					}
				}
			}

			resp, err := a.doOutgoingWebhookRequest(callbackURL, bytes.NewReader(body), contentType, accessToken)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					logger.Warn("A message action timed out. Consider increasing ServiceSettings.OutgoingIntegrationRequestsTimeout.", mlog.Err(err))
				} else {
					logger.Warn("A message action failed", mlog.Err(err))
				}
				return
			}
			responses[i] = resp
			sent[i] = true
		})
	}
	wg.Wait()

	count := 0
	for _, ok := range sent {
		if ok {
			count++
		}
	}
	return responses, count
}

// answerMessageAction posts what a message action answered with (text or attachments), as outgoing webhooks do: in
// the channel, or as a reply with response_type "comment". Where the person who ran it can't post (a read-only or
// archived channel), the answer is shown to them alone.
func (a *App) answerMessageAction(rctx request.CTX, hook *model.OutgoingWebhook, post *model.Post, channel *model.Channel, user *model.User, resp *model.OutgoingWebhookResponse) {
	if resp.Text == nil && len(resp.Attachments) == 0 {
		return
	}

	rootID := ""
	if resp.ResponseType == model.OutgoingHookResponseTypeComment {
		rootID = post.Id
		if post.RootId != "" {
			rootID = post.RootId
		}
	}

	props := resp.Props
	if len(props) == 0 {
		props = make(model.StringInterface)
	}
	props[model.PostPropsWebhookDisplayName] = hook.DisplayName

	text := ""
	if resp.Text != nil {
		text = a.ProcessSlackText(rctx, *resp.Text)
	}
	attachments := a.ProcessMessageAttachments(rctx, resp.Attachments)
	if len(attachments) > 0 {
		props[model.PostPropsAttachments] = attachments
	}

	username := resp.Username
	if *a.Config().ServiceSettings.EnablePostUsernameOverride && hook.Username != "" && username == "" {
		username = hook.Username
	}
	iconURL := resp.IconURL
	if *a.Config().ServiceSettings.EnablePostIconOverride && hook.IconURL != "" && iconURL == "" {
		iconURL = hook.IconURL
	}

	canPost := channel.DeleteAt == 0
	if canPost {
		canPost, _ = a.HasPermissionToChannel(rctx, user.Id, channel.Id, model.PermissionCreatePost)
	}
	if canPost {
		if _, err := a.CreateWebhookPost(rctx, hook.CreatorId, channel, text, username, iconURL, "", props, resp.Type, rootID, resp.Priority, false); err != nil {
			rctx.Logger().Warn("Failed to post the answer of a message action", mlog.String("outgoing_webhook_id", hook.Id), mlog.Err(err))
		}
		return
	}

	props[model.PostPropsFromWebhook] = "true"
	if *a.Config().ServiceSettings.EnablePostUsernameOverride && username != "" {
		props[model.PostPropsOverrideUsername] = username
	}
	if *a.Config().ServiceSettings.EnablePostIconOverride && iconURL != "" {
		props[model.PostPropsOverrideIconURL] = iconURL
	}
	ephemeral := &model.Post{UserId: hook.CreatorId, ChannelId: channel.Id, RootId: rootID, Message: linkWithTextRegex.ReplaceAllString(text, "[${2}](${1})")}
	ephemeral.SetProps(props)
	a.SendEphemeralPost(rctx, user.Id, ephemeral)
}

// SetOutgoingWebhookIcon stores an uploaded image as the hook's icon (a 128 px PNG) and points the hook's IconURL at
// it.
func (a *App) SetOutgoingWebhookIcon(rctx request.CTX, hook *model.OutgoingWebhook, file io.ReadSeeker) (*model.OutgoingWebhook, *model.AppError) {
	width, height, dimErr := imaging.GetDimensions(file)
	if dimErr != nil {
		return nil, model.NewAppError("SetOutgoingWebhookIcon", "app.message_action.icon.decode.app_error", nil, "", http.StatusBadRequest).Wrap(dimErr)
	}
	if limitErr := checkImageResolutionLimit(width, height, *a.Config().FileSettings.MaxImageResolution); limitErr != nil {
		return nil, model.NewAppError("SetOutgoingWebhookIcon", "app.message_action.icon.too_large.app_error", nil, "", http.StatusBadRequest).Wrap(limitErr)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, model.NewAppError("SetOutgoingWebhookIcon", "app.message_action.icon.read.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}

	buf, appErr := a.AdjustImage(rctx, file)
	if appErr != nil {
		return nil, model.NewAppError("SetOutgoingWebhookIcon", "app.message_action.icon.decode.app_error", nil, "", http.StatusBadRequest).Wrap(appErr)
	}
	if _, appErr = a.WriteFile(buf, outgoingWebhookIconFilePath(hook.Id)); appErr != nil {
		return nil, appErr
	}

	updated := *hook
	updated.IconURL = model.OutgoingWebhookIconPath(hook.Id) + "?_=" + strconv.FormatInt(model.GetMillis(), 10)
	updated.UpdateAt = model.GetMillis()
	saved, err := a.Srv().Store().Webhook().UpdateOutgoing(&updated)
	if err != nil {
		return nil, model.NewAppError("SetOutgoingWebhookIcon", "app.webhooks.update_outgoing.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return saved, nil
}

// GetOutgoingWebhookIcon returns the hook's uploaded icon, as PNG.
func (a *App) GetOutgoingWebhookIcon(hook *model.OutgoingWebhook) ([]byte, *model.AppError) {
	if !hook.HasUploadedIcon() {
		return nil, model.NewAppError("GetOutgoingWebhookIcon", "app.message_action.icon.not_found.app_error", nil, "", http.StatusNotFound)
	}
	data, appErr := a.ReadFile(outgoingWebhookIconFilePath(hook.Id))
	if appErr != nil {
		return nil, model.NewAppError("GetOutgoingWebhookIcon", "app.message_action.icon.not_found.app_error", nil, "", http.StatusNotFound).Wrap(appErr)
	}
	return data, nil
}

func (a *App) removeOutgoingWebhookIconFile(rctx request.CTX, hookID string) {
	if appErr := a.RemoveFile(outgoingWebhookIconFilePath(hookID)); appErr != nil {
		rctx.Logger().Debug("Failed to remove an outgoing webhook's icon", mlog.String("outgoing_webhook_id", hookID), mlog.Err(appErr))
	}
}

func outgoingWebhookIconFilePath(hookID string) string {
	return "outgoing_hooks/" + hookID + "/icon.png"
}

func truncateMessageActionText(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}
