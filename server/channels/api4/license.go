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
	api.BaseRoutes.License.Handle("/load_metric", api.APISessionRequired(getLicenseLoadMetric)).Methods(http.MethodGet)
}

// getClientLicense serves the static feature map read by clients that predate
// license removal, such as the official mobile and desktop apps.
func getClientLicense(c *Context, w http.ResponseWriter, r *http.Request) {
	if _, err := w.Write([]byte(model.MapToJSON(c.App.Srv().ClientLicense()))); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

// getLicenseLoadMetric answers Mattermost's mobile app, which asks for the licence load (monthly active users per
// thousand licensed users) on its About screen. There is no licence and no user limit: the load is always 0, which
// the app doesn't show.
func getLicenseLoadMetric(c *Context, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(`{"load":0}`)); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}
