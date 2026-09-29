// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package engine

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

type postDocument struct {
	Id          string   `json:"id"`
	TeamId      string   `json:"team_id"`
	ChannelId   string   `json:"channel_id"`
	ChannelType string   `json:"channel_type"`
	UserId      string   `json:"user_id"`
	RootId      string   `json:"root_id"`
	Type        string   `json:"type"`
	CreateAt    int64    `json:"create_at"`
	Message     string   `json:"message"`
	Attachments string   `json:"attachments,omitempty"`
	Hashtags    []string `json:"hashtags"`
}

type channelDocument struct {
	Id           string   `json:"id"`
	TeamId       string   `json:"team_id"`
	Type         string   `json:"type"`
	Name         string   `json:"name"`
	DisplayName  string   `json:"display_name"`
	Purpose      string   `json:"purpose"`
	DeleteAt     int64    `json:"delete_at"`
	CreateAt     int64    `json:"create_at"`
	UserIds      []string `json:"user_ids"`
	Discoverable bool     `json:"discoverable"`
}

type userDocument struct {
	Id         string   `json:"id"`
	Username   string   `json:"username"`
	Nickname   string   `json:"nickname"`
	FirstName  string   `json:"first_name"`
	LastName   string   `json:"last_name"`
	Email      string   `json:"email,omitempty"`
	Roles      []string `json:"roles"`
	RolesRaw   string   `json:"roles_raw"`
	DeleteAt   int64    `json:"delete_at"`
	CreateAt   int64    `json:"create_at"`
	TeamIds    []string `json:"team_ids"`
	ChannelIds []string `json:"channel_ids"`
}

type fileDocument struct {
	Id        string `json:"id"`
	ChannelId string `json:"channel_id"`
	PostId    string `json:"post_id"`
	UserId    string `json:"user_id"`
	CreateAt  int64  `json:"create_at"`
	Name      string `json:"name"`
	Extension string `json:"extension"`
	Content   string `json:"content"`
}

// isSearchablePostType reports whether posts of this type can ever be found
// by a search: system messages, burn-on-read and card posts never are.
func isSearchablePostType(postType string) bool {
	if strings.HasPrefix(postType, model.PostSystemMessagePrefix) {
		return false
	}
	return postType != model.PostTypeBurnOnRead && postType != model.PostTypeCard
}

func newPostDocument(post *model.Post, teamID, channelType string) *postDocument {
	doc := &postDocument{
		Id:          post.Id,
		TeamId:      teamID,
		ChannelId:   post.ChannelId,
		ChannelType: channelType,
		UserId:      post.UserId,
		RootId:      post.RootId,
		Type:        post.Type,
		CreateAt:    post.CreateAt,
		Message:     post.Message,
		Attachments: attachmentsText(post),
		Hashtags:    []string{},
	}
	for tag := range strings.FieldsSeq(post.Hashtags) {
		doc.Hashtags = append(doc.Hashtags, strings.ToLower(tag))
	}
	return doc
}

func attachmentsText(post *model.Post) string {
	if post.GetProp(model.PostPropsAttachments) == nil {
		return ""
	}
	var parts []string
	add := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			parts = append(parts, s)
		}
	}
	for _, attachment := range post.Attachments() {
		if attachment == nil {
			continue
		}
		add(attachment.Pretext)
		add(attachment.Title)
		add(attachment.Text)
		for _, field := range attachment.Fields {
			if field == nil {
				continue
			}
			add(field.Title)
			if field.Value != nil {
				add(fmt.Sprint(field.Value))
			}
		}
		if attachment.Text == "" && attachment.Pretext == "" && attachment.Title == "" {
			add(attachment.Fallback)
		}
	}
	return strings.Join(parts, "\n")
}

// shouldIndexChannel reports whether a channel belongs in the channel index,
// which only serves channel autocomplete of public and private channels.
func shouldIndexChannel(channel *model.Channel) bool {
	return channel != nil && (channel.Type == model.ChannelTypeOpen || channel.Type == model.ChannelTypePrivate)
}

func newChannelDocument(channel *model.Channel, userIDs []string) *channelDocument {
	doc := &channelDocument{
		Id:           channel.Id,
		TeamId:       channel.TeamId,
		Type:         string(channel.Type),
		Name:         channel.Name,
		DisplayName:  channel.DisplayName,
		Purpose:      channel.Purpose,
		DeleteAt:     channel.DeleteAt,
		CreateAt:     channel.CreateAt,
		UserIds:      []string{},
		Discoverable: channel.Discoverable,
	}
	if channel.Type == model.ChannelTypePrivate && userIDs != nil {
		doc.UserIds = userIDs
	}
	return doc
}

func newUserDocument(user *model.User, teamIDs, channelIDs []string) *userDocument {
	doc := &userDocument{
		Id:         user.Id,
		Username:   user.Username,
		Nickname:   user.Nickname,
		FirstName:  user.FirstName,
		LastName:   user.LastName,
		Email:      user.Email,
		Roles:      strings.Fields(user.Roles),
		RolesRaw:   strings.TrimSpace(user.Roles),
		DeleteAt:   user.DeleteAt,
		CreateAt:   user.CreateAt,
		TeamIds:    nonNil(teamIDs),
		ChannelIds: nonNil(channelIDs),
	}
	return doc
}

// ShouldIndexFile mirrors model.FileForIndexing.ShouldIndex for plain file
// infos.
func ShouldIndexFile(file *model.FileInfo) bool {
	return file != nil && file.DeleteAt == 0 && (file.PostId != "" || file.CreatorId == model.BookmarkFileOwner)
}

func newFileDocument(file *model.FileInfo, channelID string) *fileDocument {
	return &fileDocument{
		Id:        file.Id,
		ChannelId: channelID,
		PostId:    file.PostId,
		UserId:    file.CreatorId,
		CreateAt:  file.CreateAt,
		Name:      file.Name,
		Extension: strings.ToLower(strings.TrimPrefix(file.Extension, ".")),
		Content:   file.Content,
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
