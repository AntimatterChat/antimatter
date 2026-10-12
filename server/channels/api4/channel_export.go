// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

// exportChannel downloads the messages of a channel as a CSV file (Antimatter's channel export), for those who can
// read it: POST /channels/{channel_id}/export?format=csv&since=&until=, the creation time range in milliseconds.
// It's a POST, not a GET, so that a link or an image in a message can't make its readers' browsers export (the
// session cookie alone doesn't do for a POST, which needs the CSRF header). The app checks the permissions and
// audit-logs the export. The file is streamed: once it's under way, an error can only be logged.
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

	started := false
	_, appErr := c.App.ExportChannel(c.AppContext, c.AppContext.Session().UserId, c.Params.ChannelId, opts, func(file *app.ChannelExportFile) (io.Writer, error) {
		started = true
		w.Header().Set("Content-Type", file.ContentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Name))
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		return w, nil
	})
	if appErr != nil {
		if started {
			c.Logger.Warn("The channel export was cut short", mlog.String("channel_id", c.Params.ChannelId), mlog.Err(appErr))
			return
		}
		c.Err = appErr
	}
}
