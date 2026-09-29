// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package messageexport

import (
	"errors"
	"fmt"
	"sort"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Values of the "updated type" of an exported message.
const (
	UpdatedTypeEditedNewMsg       = "EditedNewMsg"
	UpdatedTypeEditedOriginalMsg  = "EditedOriginalMsg"
	UpdatedTypeUpdatedNoMsgChange = "UpdatedNoMsgChange"
	UpdatedTypeDeleted            = "Deleted"
	UpdatedTypeFileDeleted        = "FileDeleted"
)

const (
	userTypeUser = "user"
	userTypeBot  = "bot"

	fileInfoChunkSize = 1000
)

// batch is the data exported for one batch of posts.
type batch struct {
	number int
	start  int64 // exclusive lower bound of the posts' UpdateAt
	end    int64 // inclusive upper bound of the posts' UpdateAt

	posts    []*post
	channels []*channel // sorted by channel ID
}

// post is an exported post with the fields every format needs.
type post struct {
	*model.MessageExport
	channel *channel
	files   []*model.FileInfo
}

func (p *post) id() string          { return model.SafeDereference(p.PostId) }
func (p *post) createAt() int64     { return model.SafeDereference(p.PostCreateAt) }
func (p *post) updateAt() int64     { return model.SafeDereference(p.PostUpdateAt) }
func (p *post) deleteAt() int64     { return model.SafeDereference(p.PostDeleteAt) }
func (p *post) editAt() int64       { return model.SafeDereference(p.PostEditAt) }
func (p *post) message() string     { return model.SafeDereference(p.PostMessage) }
func (p *post) rootID() string      { return model.SafeDereference(p.PostRootId) }
func (p *post) originalID() string  { return model.SafeDereference(p.PostOriginalId) }
func (p *post) userID() string      { return model.SafeDereference(p.UserId) }
func (p *post) userEmail() string   { return model.SafeDereference(p.UserEmail) }
func (p *post) username() string    { return model.SafeDereference(p.Username) }
func (p *post) previewedID() string { return safePreviewID(p.MessageExport) }
func (p *post) userType() string    { return userType(p.IsBot) }
func (p *post) loginName() string   { return loginName(p.userEmail(), p.username(), p.userID()) }
func (p *post) displayUser() string { return displayUser(p.username(), p.userEmail()) }
func (p *post) channelID() string   { return model.SafeDereference(p.ChannelId) }
func (p *post) fileIDs() []string   { return []string(p.PostFileIds) }

// updatedType classifies the post: an empty type means it is a new, never
// modified message. For the original (pre-edit) version of an edited message,
// the ID of the post holding the edited content is also returned.
func (p *post) updatedType() (string, string) {
	switch {
	case p.deleteAt() > 0 && p.originalID() != "":
		// When a message is edited, its previous content is kept in a
		// deleted post whose OriginalId is the edited post.
		return UpdatedTypeEditedOriginalMsg, p.originalID()
	case p.deleteAt() > 0:
		return UpdatedTypeDeleted, ""
	case p.editAt() > 0:
		return UpdatedTypeEditedNewMsg, ""
	case p.updateAt() > p.createAt():
		return UpdatedTypeUpdatedNoMsgChange, ""
	default:
		return "", ""
	}
}

func safePreviewID(m *model.MessageExport) (id string) {
	defer func() {
		// PreviewID type-asserts the prop value; be defensive with bad data.
		if recover() != nil {
			id = ""
		}
	}()
	return m.PreviewID()
}

func userType(isBot bool) string {
	if isBot {
		return userTypeBot
	}
	return userTypeUser
}

func loginName(email, username, userID string) string {
	switch {
	case email != "":
		return email
	case username != "":
		return username
	default:
		return userID
	}
}

func displayUser(username, email string) string {
	switch {
	case username != "" && email != "":
		return fmt.Sprintf("%s (%s)", username, email)
	case username != "":
		return username
	default:
		return email
	}
}

// channel is an exported channel with its participants during the batch.
type channel struct {
	ID              string
	Name            string
	DisplayName     string
	Type            model.ChannelType
	TeamID          string
	TeamName        string
	TeamDisplayName string

	posts        []*post
	participants []*participant // sorted by username
}

func (c *channel) typeName() string {
	return channelTypeName(c.Type)
}

func (c *channel) roomID() string {
	return c.typeName() + " - " + c.ID
}

func channelTypeName(t model.ChannelType) string {
	switch t {
	case model.ChannelTypeOpen:
		return "public"
	case model.ChannelTypePrivate:
		return "private"
	case model.ChannelTypeDirect:
		return "direct"
	case model.ChannelTypeGroup:
		return "group"
	default:
		return string(t)
	}
}

func channelDisplayName(ch *model.Channel) string {
	switch ch.Type {
	case model.ChannelTypeDirect:
		return "Direct Message"
	case model.ChannelTypeGroup:
		return "Group Message"
	default:
		return ch.DisplayName
	}
}

// membership is one join/leave interval of a participant.
type membership struct {
	JoinTime  int64
	LeaveTime int64 // 0 when the user did not leave during the batch
}

// participant is a user who was a member of the channel, or posted in it,
// during the batch.
type participant struct {
	UserID    string
	Email     string
	Username  string
	IsBot     bool
	Intervals []membership // sorted by JoinTime
	Messages  int
}

func (p *participant) loginName() string   { return loginName(p.Email, p.Username, p.UserID) }
func (p *participant) displayUser() string { return displayUser(p.Username, p.Email) }
func (p *participant) userType() string    { return userType(p.IsBot) }

// channelEvent is a join/leave event (or a "previously joined" marker) of a
// participant, used by the formats which list them chronologically.
type channelEvent struct {
	Kind        string // eventPreviouslyJoined, eventJoin or eventLeave
	Time        int64
	Participant *participant
}

const (
	eventPreviouslyJoined = "previously-joined"
	eventJoin             = "enter"
	eventLeave            = "leave"
)

// events returns the membership events of the channel's participants during
// the batch, sorted chronologically.
func (c *channel) events(b *batch) []channelEvent {
	var events []channelEvent
	for _, p := range c.participants {
		for _, m := range p.Intervals {
			if m.JoinTime <= b.start {
				events = append(events, channelEvent{Kind: eventPreviouslyJoined, Time: b.start, Participant: p})
			} else {
				events = append(events, channelEvent{Kind: eventJoin, Time: m.JoinTime, Participant: p})
			}
			if m.LeaveTime > 0 && m.LeaveTime <= b.end {
				events = append(events, channelEvent{Kind: eventLeave, Time: m.LeaveTime, Participant: p})
			}
		}
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].Time < events[j].Time })
	return events
}

// batchBuilder gathers everything a batch needs from the store.
type batchBuilder struct {
	store                   store.Store
	channelBatchSize        int
	channelHistoryBatchSize int
}

// build assembles the batch of the given posts, covering (start, end].
func (bb *batchBuilder) build(number int, start, end int64, messages []*model.MessageExport) (*batch, error) {
	b := &batch{number: number, start: start, end: end}

	channels := map[string]*channel{}
	for _, m := range messages {
		p := &post{MessageExport: m}
		cid := p.channelID()
		ch, ok := channels[cid]
		if !ok {
			ch = &channel{
				ID:              cid,
				Name:            model.SafeDereference(m.ChannelName),
				DisplayName:     model.SafeDereference(m.ChannelDisplayName),
				Type:            model.SafeDereference(m.ChannelType),
				TeamID:          model.SafeDereference(m.TeamId),
				TeamName:        model.SafeDereference(m.TeamName),
				TeamDisplayName: model.SafeDereference(m.TeamDisplayName),
			}
			channels[cid] = ch
		}
		p.channel = ch
		ch.posts = append(ch.posts, p)
		b.posts = append(b.posts, p)
	}

	if err := bb.addFiles(b); err != nil {
		return nil, err
	}

	// Channels which had membership changes (or posts) during the batch
	// window but no post in this batch are exported too, so that no join or
	// leave event is lost.
	activeIDs, err := bb.store.ChannelMemberHistory().GetChannelsWithActivityDuring(start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get the channels with activity: %w", err)
	}
	var missing []string
	for _, id := range activeIDs {
		if _, ok := channels[id]; !ok {
			missing = append(missing, id)
		}
	}
	if err := bb.addChannels(channels, missing); err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(channels))
	for id := range channels {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		b.channels = append(b.channels, channels[id])
	}

	if err := bb.addParticipants(b); err != nil {
		return nil, err
	}
	return b, nil
}

// addChannels loads the channels (and their teams) which have no post in the
// batch.
func (bb *batchBuilder) addChannels(channels map[string]*channel, ids []string) error {
	size := bb.channelBatchSize
	if size <= 0 {
		size = model.ComplianceExportChannelBatchSizeDefault
	}
	for start := 0; start < len(ids); start += size {
		chunk := ids[start:min(start+size, len(ids))]
		list, err := bb.store.Channel().GetChannelsByIds(chunk, true)
		if err != nil {
			return fmt.Errorf("failed to get channels: %w", err)
		}

		teamIDs := []string{}
		for _, ch := range list {
			if ch.TeamId != "" {
				teamIDs = append(teamIDs, ch.TeamId)
			}
		}
		teams := map[string]*model.Team{}
		if len(teamIDs) > 0 {
			teamList, err := bb.store.Team().GetMany(teamIDs)
			var nfErr *store.ErrNotFound
			if err != nil && !errors.As(err, &nfErr) {
				return fmt.Errorf("failed to get teams: %w", err)
			}
			for _, t := range teamList {
				teams[t.Id] = t
			}
		}

		for _, ch := range list {
			c := &channel{
				ID:          ch.Id,
				Name:        ch.Name,
				DisplayName: channelDisplayName(ch),
				Type:        ch.Type,
				TeamID:      ch.TeamId,
			}
			if t := teams[ch.TeamId]; t != nil {
				c.TeamName = t.Name
				c.TeamDisplayName = t.DisplayName
			}
			channels[ch.Id] = c
		}
	}
	return nil
}

// addFiles attaches the file infos to the posts of the batch.
func (bb *batchBuilder) addFiles(b *batch) error {
	var ids []string
	for _, p := range b.posts {
		ids = append(ids, p.fileIDs()...)
	}
	if len(ids) == 0 {
		return nil
	}
	infos := map[string]*model.FileInfo{}
	for start := 0; start < len(ids); start += fileInfoChunkSize {
		chunk := ids[start:min(start+fileInfoChunkSize, len(ids))]
		list, err := bb.store.FileInfo().GetByIds(chunk, true, false, false)
		if err != nil {
			return fmt.Errorf("failed to get file infos: %w", err)
		}
		for _, info := range list {
			infos[info.Id] = info
		}
	}
	for _, p := range b.posts {
		for _, id := range p.fileIDs() {
			if info := infos[id]; info != nil {
				p.files = append(p.files, info)
			}
		}
	}
	return nil
}

// addParticipants loads the channel member history of the batch's channels
// and adds the authors of the posts who were not members.
func (bb *batchBuilder) addParticipants(b *batch) error {
	size := bb.channelHistoryBatchSize
	if size <= 0 {
		size = model.ComplianceExportChannelHistoryBatchSizeDefault
	}

	byChannel := map[string]map[string]*participant{}
	for _, c := range b.channels {
		byChannel[c.ID] = map[string]*participant{}
	}

	for start := 0; start < len(b.channels); start += size {
		chunk := b.channels[start:min(start+size, len(b.channels))]
		ids := make([]string, len(chunk))
		for i, c := range chunk {
			ids[i] = c.ID
		}
		history, err := bb.store.ChannelMemberHistory().GetUsersInChannelDuring(b.start, b.end, ids)
		if err != nil {
			return fmt.Errorf("failed to get the channel member history: %w", err)
		}
		for _, h := range history {
			members := byChannel[h.ChannelId]
			if members == nil {
				continue
			}
			p := members[h.UserId]
			if p == nil {
				p = &participant{UserID: h.UserId, Email: h.UserEmail, Username: h.Username, IsBot: h.IsBot}
				members[h.UserId] = p
			}
			m := membership{JoinTime: h.JoinTime}
			if h.LeaveTime != nil {
				m.LeaveTime = *h.LeaveTime
			}
			p.Intervals = append(p.Intervals, m)
		}
	}

	for _, pst := range b.posts {
		members := byChannel[pst.channelID()]
		p := members[pst.userID()]
		if p == nil {
			// The author was not a member (e.g. a webhook or a bot posting
			// without membership): consider they joined when posting.
			p = &participant{
				UserID:    pst.userID(),
				Email:     pst.userEmail(),
				Username:  pst.username(),
				IsBot:     pst.IsBot,
				Intervals: []membership{{JoinTime: pst.createAt()}},
			}
			members[pst.userID()] = p
		}
		p.Messages++
	}

	for _, c := range b.channels {
		for _, p := range byChannel[c.ID] {
			sort.Slice(p.Intervals, func(i, j int) bool { return p.Intervals[i].JoinTime < p.Intervals[j].JoinTime })
			c.participants = append(c.participants, p)
		}
		sort.Slice(c.participants, func(i, j int) bool {
			if c.participants[i].Username != c.participants[j].Username {
				return c.participants[i].Username < c.participants[j].Username
			}
			return c.participants[i].UserID < c.participants[j].UserID
		})
	}
	return nil
}
