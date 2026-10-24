// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"slices"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Actions' offers (the owner's rule): when several people have the assistant open in a conversation, the same action
// (same channel, tool and arguments, within ten minutes) is one offer, kept server-side:
//
//   - everyone it was proposed for (who may use the tool) gets its card; it's offered to one of them at a time: first
//     the author of the message that asked for it, then those who spoke after them in the window, then the others by
//     who opened the assistant first; only someone viewing gets it;
//   - the holder dismissing it, or ignoring it for the offer's timeout, hands it to the next; someone who passed never
//     gets it back; nobody viewing leaves it with nobody until someone who may comes back;
//   - only the holder may run it; running marks it done for everyone, and a second run is refused (409);
//   - every change is a compare-and-set of the offer in the key-value store, so that two nodes (or two people) can't
//     both take it.
//
// Running an action is only claimed and closed in this stage: actions run from stage 3, through the confirmation
// flow of the tool registry.

// assistantOffer is an action's offer.
type assistantOffer struct {
	Id        string `json:"id"`
	ChannelId string `json:"channel_id"`
	DedupeKey string `json:"dedupe_key"`
	Tool      string `json:"tool"`
	AuthorId  string `json:"author_id"`
	// Speakers spoke after the author in the window, in order.
	Speakers []string `json:"speakers"`
	// Eligible got the card (in the order it was made for them); Cards gives each one's card.
	Eligible []string          `json:"eligible"`
	Cards    map[string]string `json:"cards"`
	// Passed dismissed it or let it time out.
	Passed    []string `json:"passed"`
	HolderId  string   `json:"holder_id"`
	HeldSince int64    `json:"held_since"`
	TimeoutMs int64    `json:"timeout_ms"`
	Status    string   `json:"status"`
	DoneBy    string   `json:"done_by,omitempty"`
	DoneAt    int64    `json:"done_at,omitempty"`
	CreateAt  int64    `json:"create_at"`
}

func assistantOfferKey(id string) string {
	return assistantOfferPrefix + id
}

func (a *App) assistantOffer(id string) *assistantOffer {
	raw, appErr := a.GetPluginKey(assistantKVNamespace, assistantOfferKey(id))
	if appErr != nil || raw == nil {
		return nil
	}
	var o assistantOffer
	if json.Unmarshal(raw, &o) != nil {
		return nil
	}
	return &o
}

// updateAssistantOffer changes an offer with compare-and-set; change tells whether it changed it.
func (a *App) updateAssistantOffer(rctx request.CTX, id string, change func(o *assistantOffer) bool) (*assistantOffer, bool, *model.AppError) {
	var result *assistantOffer
	written, appErr := a.assistantKVUpdate(rctx, assistantOfferKey(id), assistantOfferTTL, func(old []byte) ([]byte, bool) {
		result = nil
		if old == nil {
			return nil, false
		}
		var o assistantOffer
		if json.Unmarshal(old, &o) != nil {
			return nil, false
		}
		result = &o
		if !change(&o) {
			return nil, false
		}
		out, err := json.Marshal(&o)
		if err != nil {
			return nil, false
		}
		return out, true
	})
	return result, written, appErr
}

// view is the offer as a person sees it.
func (o *assistantOffer) view(userID string) *model.AssistantOfferView {
	v := &model.AssistantOfferView{Id: o.Id, Status: o.Status, HolderId: o.HolderId, DoneBy: o.DoneBy, DoneAt: o.DoneAt}
	open := o.Status == model.AssistantOfferStatusOpen
	v.OfferedToMe = open && o.HolderId == userID
	v.CanRun = v.OfferedToMe
	if open && o.HolderId != "" {
		v.MovesAt = o.HeldSince + o.TimeoutMs
	}
	return v
}

// assistantOfferView gives an offer as a person sees it; an offer that's gone is expired.
func (a *App) assistantOfferView(offerID, userID string) *model.AssistantOfferView {
	o := a.assistantOffer(offerID)
	if o == nil {
		return &model.AssistantOfferView{Id: offerID, Status: model.AssistantOfferStatusExpired}
	}
	return o.view(userID)
}

// assistantSpeakersAfter gives who spoke after a message in a window, in order, once each, but its author.
func assistantSpeakersAfter(posts []*model.Post, triggerID, authorID string) []string {
	var speakers []string
	after := false
	for _, p := range posts {
		if p.Id == triggerID {
			after = true
			continue
		}
		if after && p.UserId != authorID && !slices.Contains(speakers, p.UserId) {
			speakers = append(speakers, p.UserId)
		}
	}
	return speakers
}

// offerAssistantAction makes an action's offer, or joins the one of the same call, and gives the person its card
// (once). It gives the offer's ID, "" when the person gets nothing (the offer is done, or their budget spent).
func (a *App) offerAssistantAction(rctx request.CTX, channel *model.Channel, user *model.User, c *assistantCandidate, posts []*model.Post, cfg *assistantConfig) string {
	key := assistantOfferKeyPrefix + channel.Id + "_" + c.DedupeKey
	existing := ""
	if raw, _ := a.GetPluginKey(assistantKVNamespace, key); raw != nil {
		existing = string(raw)
		if o := a.assistantOffer(existing); o == nil || o.Status != model.AssistantOfferStatusOpen || o.Cards[user.Id] != "" {
			return existing
		}
	}
	if !a.assistantTakeBudget(rctx, "cards_user", user.Id, cfg.cardsPerUser) {
		return ""
	}
	card := newAssistantCard(user.Id, c)
	trigger := ""
	if len(c.PostIds) > 0 {
		trigger = c.PostIds[0]
	}

	offerID := existing
	if offerID == "" {
		now := model.GetMillis()
		o := &assistantOffer{
			Id: model.NewId(), ChannelId: channel.Id, DedupeKey: c.DedupeKey, Tool: c.Tool, AuthorId: c.AuthorId,
			Speakers: assistantSpeakersAfter(posts, trigger, c.AuthorId), Eligible: []string{user.Id},
			Cards: map[string]string{user.Id: card.Id}, Passed: []string{}, TimeoutMs: cfg.offerTimeout.Milliseconds(),
			Status: model.AssistantOfferStatusOpen, CreateAt: now,
		}
		data, err := json.Marshal(o)
		if err != nil {
			return ""
		}
		if appErr := a.SetPluginKeyWithExpiry(assistantKVNamespace, assistantOfferKey(o.Id), data, int64(assistantOfferTTL/time.Second)); appErr != nil {
			rctx.Logger().Warn("The assistant couldn't save an offer", mlog.Err(appErr))
			return ""
		}
		inserted, appErr := a.assistantKVInsert(key, []byte(o.Id), assistantDedupeWindow)
		if appErr != nil {
			_ = a.DeletePluginKey(assistantKVNamespace, assistantOfferKey(o.Id))
			return ""
		}
		if inserted {
			offerID = o.Id
		} else {
			// Another node made the same offer meanwhile: join it.
			_ = a.DeletePluginKey(assistantKVNamespace, assistantOfferKey(o.Id))
			raw, _ := a.GetPluginKey(assistantKVNamespace, key)
			if raw == nil {
				return ""
			}
			existing = string(raw)
			offerID = existing
		}
	}
	if existing != "" {
		joined := false
		if _, _, appErr := a.updateAssistantOffer(rctx, offerID, func(o *assistantOffer) bool {
			joined = false
			if o.Status != model.AssistantOfferStatusOpen || o.Cards[user.Id] != "" {
				return false
			}
			if o.Cards == nil {
				o.Cards = map[string]string{}
			}
			o.Cards[user.Id] = card.Id
			o.Eligible = append(o.Eligible, user.Id)
			joined = true
			return true
		}); appErr != nil || !joined {
			return offerID
		}
	}
	card.OfferId = offerID
	a.addAssistantCard(rctx, card)
	return offerID
}

// assistantOfferOrder gives who may hold an offer now, in the offer's order: the author, those who spoke after
// them, then the others by who opened the assistant first; only those who got the card, haven't passed, and are
// viewing.
func assistantOfferOrder(o *assistantOffer, viewers []assistantViewer) []string {
	viewing := map[string]bool{}
	for _, v := range viewers {
		viewing[v.userID] = true
	}
	candidates := append([]string{o.AuthorId}, o.Speakers...)
	for _, v := range viewers {
		candidates = append(candidates, v.userID)
	}
	var order []string
	for _, id := range candidates {
		if id == "" || slices.Contains(order, id) || !viewing[id] || o.Cards[id] == "" || slices.Contains(o.Passed, id) {
			continue
		}
		order = append(order, id)
	}
	return order
}

// assignAssistantOffer gives an offer to whom it's for now: it stays with a holder who's viewing and within the
// timeout; else it goes to the next in the order (the holder who timed out passes, unless nobody else may have it).
// Everyone with the card is told when it moves.
func (a *App) assignAssistantOffer(rctx request.CTX, offerID string) {
	current := a.assistantOffer(offerID)
	if current == nil || current.Status != model.AssistantOfferStatusOpen {
		return
	}
	o, written, appErr := a.updateAssistantOffer(rctx, offerID, func(o *assistantOffer) bool {
		if o.Status != model.AssistantOfferStatusOpen {
			return false
		}
		viewers := a.assistantViewers(o.ChannelId, false)
		order := assistantOfferOrder(o, viewers)
		now := model.GetMillis()
		holderViewing := o.HolderId != "" && slices.Contains(order, o.HolderId)
		timedOut := holderViewing && now-o.HeldSince >= o.TimeoutMs
		switch {
		case holderViewing && !timedOut:
			return false
		case timedOut:
			next := ""
			for _, id := range order {
				if id != o.HolderId {
					next = id
					break
				}
			}
			if next == "" {
				// Nobody else may have it: it stays with them.
				return false
			}
			o.Passed = append(o.Passed, o.HolderId)
			o.HolderId, o.HeldSince = next, now
			return true
		default:
			next := ""
			if len(order) > 0 {
				next = order[0]
			}
			if next == o.HolderId {
				return false
			}
			o.HolderId, o.HeldSince = next, now
			return true
		}
	})
	if appErr != nil {
		rctx.Logger().Debug("The assistant couldn't hand an offer on", mlog.String("offer_id", offerID), mlog.Err(appErr))
		return
	}
	if o == nil {
		return
	}
	if written {
		a.publishAssistantOffer(o)
	}
	a.scheduleAssistantOffer(o)
}

// refreshAssistantOffer hands an offer on if its holder left or timed out (when someone looks at it, in case the
// node that had its timer stopped).
func (a *App) refreshAssistantOffer(rctx request.CTX, offerID string) {
	a.assignAssistantOffer(rctx, offerID)
}

// scheduleAssistantOffer looks at an offer again when its holder's time is up.
func (a *App) scheduleAssistantOffer(o *assistantOffer) {
	st := a.assistant()
	st.mu.Lock()
	defer st.mu.Unlock()
	if old, ok := st.timers[o.Id]; ok {
		old.Stop()
		delete(st.timers, o.Id)
	}
	if st.stopped || o.Status != model.AssistantOfferStatusOpen || o.HolderId == "" {
		return
	}
	if st.timers == nil {
		st.timers = map[string]*time.Timer{}
	}
	wait := time.Until(time.UnixMilli(o.HeldSince+o.TimeoutMs)) + 50*time.Millisecond
	id := o.Id
	st.timers[id] = time.AfterFunc(max(wait, 0), func() {
		st.mu.Lock()
		delete(st.timers, id)
		stopped := st.stopped
		st.mu.Unlock()
		if !stopped {
			a.Srv().Go(func() { a.assignAssistantOffer(request.EmptyContext(a.Log()), id) })
		}
	})
}

// publishAssistantOffer tells everyone who has an offer's card where it stands, as each of them sees it.
func (a *App) publishAssistantOffer(o *assistantOffer) {
	for userID, cardID := range o.Cards {
		a.publishAssistant(model.WebsocketEventAssistantOfferUpdated, userID, map[string]any{
			"channel_id": o.ChannelId, "card_id": cardID, "offer": o.view(userID),
		})
	}
}

// passAssistantOffer is someone passing an offer (dismissing their card): they never get it again, and when they
// held it, it goes to the next.
func (a *App) passAssistantOffer(rctx request.CTX, offerID, userID string) {
	if _, _, appErr := a.updateAssistantOffer(rctx, offerID, func(o *assistantOffer) bool {
		if o.Status != model.AssistantOfferStatusOpen || o.Cards[userID] == "" || slices.Contains(o.Passed, userID) {
			return false
		}
		o.Passed = append(o.Passed, userID)
		if o.HolderId == userID {
			o.HolderId, o.HeldSince = "", model.GetMillis()
		}
		return true
	}); appErr != nil {
		rctx.Logger().Debug("The assistant couldn't pass an offer", mlog.String("offer_id", offerID), mlog.Err(appErr))
	}
	a.assignAssistantOffer(rctx, offerID)
	if o := a.assistantOffer(offerID); o != nil && o.HolderId == "" {
		// It's with nobody now: say so.
		a.publishAssistantOffer(o)
	}
}

// assistantActionCard gives one of a person's action cards, and checks they may still have the assistant there.
func (a *App) assistantActionCard(rctx request.CTX, userID, channelID, cardID string) (*model.User, *model.Channel, *model.AssistantCard, *model.AppError) {
	const where = "assistantActionCard"
	user, channel, appErr := a.assistantChannelFor(rctx, userID, channelID)
	if appErr != nil {
		return nil, nil, nil, appErr
	}
	card := a.findAssistantCard(userID, channelID, cardID)
	if card == nil || card.Status == model.AssistantCardStatusDismissed {
		return nil, nil, nil, model.NewAppError(where, "app.assistant.card_not_found.app_error", nil, "", http.StatusNotFound)
	}
	if card.OfferId == "" {
		return nil, nil, nil, model.NewAppError(where, "app.assistant.not_an_action.app_error", nil, "", http.StatusBadRequest)
	}
	return user, channel, card, nil
}

func errAssistantOfferDone() *model.AppError {
	return model.NewAppError("assistantOffer", "app.assistant.offer_done.app_error", nil, "", http.StatusConflict)
}

func errAssistantOfferNotHolder() *model.AppError {
	return model.NewAppError("assistantOffer", "app.assistant.offer_not_yours.app_error", nil, "", http.StatusForbidden)
}

func errAssistantOfferGone() *model.AppError {
	return model.NewAppError("assistantOffer", "app.assistant.offer_gone.app_error", nil, "", http.StatusGone)
}

// ClaimAssistantCard takes an action that's offered to nobody (nobody who may run it was viewing) for the person,
// if they may have it; one already theirs stays theirs. Someone else's is refused (403), a done one too (409).
func (a *App) ClaimAssistantCard(rctx request.CTX, userID, channelID, cardID string) (*model.AssistantCard, *model.AppError) {
	_, channel, card, appErr := a.assistantActionCard(rctx, userID, channelID, cardID)
	if appErr != nil {
		return nil, appErr
	}
	if cfg := a.assistantConfigFor(channel.TeamId); !cfg.enabled || channel.DeleteAt != 0 {
		return nil, model.NewAppError("ClaimAssistantCard", "app.assistant.off.app_error", nil, "", http.StatusForbidden)
	}
	a.assignAssistantOffer(rctx, card.OfferId)
	var claimErr *model.AppError
	o, written, appErr := a.updateAssistantOffer(rctx, card.OfferId, func(o *assistantOffer) bool {
		claimErr = nil
		switch {
		case o.Status != model.AssistantOfferStatusOpen:
			claimErr = errAssistantOfferDone()
			return false
		case o.HolderId == userID:
			return false
		case o.HolderId != "" || o.Cards[userID] == "" || slices.Contains(o.Passed, userID) || !slices.Contains(assistantOfferOrder(o, a.assistantViewers(o.ChannelId, false)), userID):
			claimErr = errAssistantOfferNotHolder()
			return false
		}
		o.HolderId, o.HeldSince = userID, model.GetMillis()
		return true
	})
	if appErr != nil {
		return nil, appErr
	}
	if o == nil {
		return nil, errAssistantOfferGone()
	}
	if claimErr != nil {
		return nil, claimErr
	}
	if written {
		a.publishAssistantOffer(o)
		a.scheduleAssistantOffer(o)
	}
	card.Offer = o.view(userID)
	return card, nil
}

// RunAssistantCard runs an action offered to the person: only its holder may, once; it's then done for everyone.
// The person, the channel and the tool are checked again. In this stage the action itself doesn't run (stage 3):
// the offer is claimed, closed and audited.
func (a *App) RunAssistantCard(rctx request.CTX, userID, channelID, cardID string) (*model.AssistantRunResult, *model.AppError) {
	const where = "RunAssistantCard"
	user, channel, card, appErr := a.assistantActionCard(rctx, userID, channelID, cardID)
	if appErr != nil {
		return nil, appErr
	}
	cfg := a.assistantConfigFor(channel.TeamId)
	if !cfg.enabled || channel.DeleteAt != 0 || !a.assistantChannelOn(userID, channelID) {
		return nil, model.NewAppError(where, "app.assistant.off.app_error", nil, "", http.StatusForbidden)
	}
	if !slices.ContainsFunc(a.assistantToolsFor(rctx, user, channel, cfg), func(rt *aiResolvedTool) bool { return rt.tool.Name == card.Tool }) {
		return nil, model.NewAppError(where, "app.ai.tools.call_not_allowed.app_error", nil, "", http.StatusForbidden)
	}
	a.assignAssistantOffer(rctx, card.OfferId)
	var runErr *model.AppError
	o, _, appErr := a.updateAssistantOffer(rctx, card.OfferId, func(o *assistantOffer) bool {
		runErr = nil
		switch {
		case o.Status != model.AssistantOfferStatusOpen:
			runErr = errAssistantOfferDone()
			return false
		case o.HolderId != userID:
			runErr = errAssistantOfferNotHolder()
			return false
		}
		o.Status, o.DoneBy, o.DoneAt = model.AssistantOfferStatusDone, userID, model.GetMillis()
		return true
	})
	if appErr != nil {
		return nil, appErr
	}
	if o == nil {
		return nil, errAssistantOfferGone()
	}
	if runErr != nil {
		return nil, runErr
	}

	rec := a.MakeAuditRecord(rctx, model.AuditEventAssistantRunAction, model.AuditStatusFail)
	rec.Actor.UserId = userID
	sum := sha256.Sum256(card.Arguments)
	model.AddEventParameterToAuditRec(rec, "tool", card.Tool)
	model.AddEventParameterToAuditRec(rec, "offer_id", o.Id)
	model.AddEventParameterToAuditRec(rec, "channel_id", channel.Id)
	model.AddEventParameterToAuditRec(rec, "team_id", channel.TeamId)
	model.AddEventParameterToAuditRec(rec, "arguments_bytes", len(card.Arguments))
	model.AddEventParameterToAuditRec(rec, "arguments_sha256", hex.EncodeToString(sum[:]))
	model.AddEventParameterToAuditRec(rec, "executed", false)
	rec.Success()
	a.LogAuditRecWithLevel(rctx, rec, mlog.LvlAuditAPI, nil)

	a.scheduleAssistantOffer(o)
	a.publishAssistantOffer(o)
	card.Offer = o.view(userID)
	return &model.AssistantRunResult{Card: card, Executed: false}, nil
}
