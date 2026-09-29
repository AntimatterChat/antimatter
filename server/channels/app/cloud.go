// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
)

func (a *App) AdjustInProductLimits(limits *model.ProductLimits, subscription *model.Subscription) *model.AppError {
	if limits.Teams != nil && limits.Teams.Active != nil && *limits.Teams.Active > 0 {
		err := a.AdjustTeamsFromProductLimits(limits.Teams)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetPreviewModalData fetches modal content data from the configured S3 bucket
func (a *App) GetPreviewModalData() ([]model.PreviewModalContentData, *model.AppError) {
	bucketURL := a.Config().CloudSettings.PreviewModalBucketURL
	if bucketURL == nil || *bucketURL == "" {
		return nil, model.NewAppError("GetPreviewModalData", "app.cloud.preview_modal_bucket_url_not_configured", nil, "", http.StatusNotFound)
	}

	// Construct the full URL to the modal_content.json file
	fileURL := *bucketURL + "/modal_content.json"

	// Make HTTP request to S3
	resp, err := http.Get(fileURL)
	if err != nil {
		return nil, model.NewAppError("GetPreviewModalData", "app.cloud.preview_modal_data_fetch_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, model.NewAppError("GetPreviewModalData", "app.cloud.preview_modal_data_fetch_error", nil, "", resp.StatusCode)
	}

	// Parse the JSON response
	var modalData []model.PreviewModalContentData
	if err := json.NewDecoder(resp.Body).Decode(&modalData); err != nil {
		return nil, model.NewAppError("GetPreviewModalData", "app.cloud.preview_modal_data_parse_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	return modalData, nil
}
