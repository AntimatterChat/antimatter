// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

const (
	// ChannelExportFormatCSV is the format of Antimatter's channel export, the only one so far.
	ChannelExportFormatCSV = "csv"

	// ChannelExportMaxPosts is the most messages a channel export holds: past that, a shorter date range must be
	// asked for.
	ChannelExportMaxPosts = 50000

	// ChannelExportMaxAttachmentsSize is the largest message attachments (props.attachments, as JSON) the channel
	// export reads for a post, in bytes: larger ones are left out of the file, which keeps the memory an export
	// takes small.
	ChannelExportMaxAttachmentsSize = 64 * 1024
)

// ChannelExportFilter selects the posts of Antimatter's channel export: the messages of a channel created in
// [SinceCreateAt, UntilCreateAt] (0 leaves a bound open), as the channel shows them now. System messages, deleted
// messages, the previous versions of edited ones and burn-on-read messages are left out.
type ChannelExportFilter struct {
	ChannelId     string
	SinceCreateAt int64
	UntilCreateAt int64
}

// ChannelExportCursor is where a page of the channel export starts: after the post created at AfterCreateAt with
// the id AfterId. The zero cursor starts at the beginning.
type ChannelExportCursor struct {
	AfterCreateAt int64
	AfterId       string
}

// ChannelExportPost is a post as the channel export reads it: only what the file shows.
type ChannelExportPost struct {
	PostId       string
	PostCreateAt int64
	PostEditAt   int64
	PostRootId   string
	PostType     string
	PostMessage  string
	PostFileIds  StringArray

	UserId    string
	Username  string
	UserEmail string
	IsBot     bool

	// The post's from_webhook and override_username props.
	FromWebhook      string
	OverrideUsername string

	// Attachments holds the post's message attachments (props.attachments) as JSON, or is empty when there are
	// none or they are larger than ChannelExportMaxAttachmentsSize.
	Attachments string
}
