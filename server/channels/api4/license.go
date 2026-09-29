// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

func (api *API) InitLicense() {
	api.BaseRoutes.License.Handle("/client", api.APIHandler(getClientLicense)).Methods(http.MethodGet)
}

// getClientLicense serves the static feature map read by clients that predate
// license removal, such as the official mobile and desktop apps.
func getClientLicense(c *Context, w http.ResponseWriter, r *http.Request) {
	if _, err := w.Write([]byte(model.MapToJSON(c.App.Srv().ClientLicense()))); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}
