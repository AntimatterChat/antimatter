// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

const (
	// ChannelExportFormatCSV is the format of Antimatter's channel export, the only one so far.
	ChannelExportFormatCSV = "csv"

	// ChannelExportMaxPosts is the most messages a channel export holds: past that, a shorter date range must be
	// asked for.
	ChannelExportMaxPosts = 50000
)
