// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

// exportChannel downloads the messages of a channel as a CSV file (Antimatter's channel export), for those who can
// read it: GET /channels/{channel_id}/export?format=csv&since=&until=, the creation time range in milliseconds.
// The app checks the permissions and audit-logs the export.
func exportChannel(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelId()
	if c.Err != nil {
		return
	}

	query := r.URL.Query()
	opts := app.ChannelExportOptions{Format: query.Get("format")}
	for name, value := range map[string]*int64{"since": &opts.Since, "until": &opts.Until} {
		if s := query.Get(name); s != "" {
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				c.SetInvalidURLParam(name)
				return
			}
			*value = n
		}
	}

	file, appErr := c.App.ExportChannel(c.AppContext, c.AppContext.Session().UserId, c.Params.ChannelId, opts)
	if appErr != nil {
		c.Err = appErr
		return
	}

	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Name))
	w.Header().Set("Cache-Control", "no-store")
	if _, err := w.Write(file.Data); err != nil {
		c.Logger.Warn("Failed to write the channel export", mlog.String("channel_id", c.Params.ChannelId), mlog.Err(err))
	}
}
