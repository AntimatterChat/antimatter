// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func Test_SendNotifyAdminPosts(t *testing.T) {
	mainHelper.Parallel(t)
	t.Run("no error sending non trial upgrade post when no notifications are available", func(t *testing.T) {
		th := Setup(t).InitBasic(t)

		err := th.App.SendNotifyAdminPosts(th.Context, "", "", false)
		require.Nil(t, err)
	})

	t.Run("no error sending trial upgrade post when no notifications are available", func(t *testing.T) {
		th := Setup(t).InitBasic(t)

		err := th.App.SendNotifyAdminPosts(th.Context, "", "", true)
		require.Nil(t, err)
	})

	t.Run("error when trying to send upgrade post before end of cool off period", func(t *testing.T) {
		th := Setup(t).InitBasic(t)

		// some notifications
		_, appErr := th.App.SaveAdminNotifyData(&model.NotifyAdminData{
			UserId:          th.BasicUser.Id,
			RequiredPlan:    model.LicenseShortSkuProfessional,
			RequiredFeature: model.PaidFeatureAllProfessionalfeatures,
		})
		require.Nil(t, appErr)

		appErr = th.App.SendNotifyAdminPosts(th.Context, "", "", false)
		require.Nil(t, appErr)

		// add some more notifications while in cool off
		_, appErr = th.App.SaveAdminNotifyData(&model.NotifyAdminData{
			UserId:          th.BasicUser.Id,
			RequiredPlan:    model.LicenseShortSkuProfessional,
			RequiredFeature: model.PaidFeatureCustomUsergroups,
		})
		require.Nil(t, appErr)

		// second time trying to notify is forbidden
		appErr = th.App.SendNotifyAdminPosts(th.Context, "", "", false)
		require.NotNil(t, appErr)
		require.Equal(t, appErr.Error(), "SendNotifyAdminPosts: Unable to send notification post., Cannot notify yet")
	})

	t.Run("can send upgrade post at the end of cool off period", func(t *testing.T) {
		th := Setup(t).InitBasic(t)

		th.App.Srv().SetNotifyAdminCoolOffDaysOverride("0.00003472222222") // set to 3 seconds
		t.Cleanup(func() { th.App.Srv().SetNotifyAdminCoolOffDaysOverride("") })

		// some notifications
		_, appErr := th.App.SaveAdminNotifyData(&model.NotifyAdminData{
			UserId:          th.BasicUser.Id,
			RequiredPlan:    model.LicenseShortSkuProfessional,
			RequiredFeature: model.PaidFeatureAllProfessionalfeatures,
		})
		require.Nil(t, appErr)

		appErr = th.App.SendNotifyAdminPosts(th.Context, "", "", false)
		require.Nil(t, appErr)

		// add some more notifications while in cool off
		_, appErr = th.App.SaveAdminNotifyData(&model.NotifyAdminData{
			UserId:          th.BasicUser.Id,
			RequiredPlan:    model.LicenseShortSkuProfessional,
			RequiredFeature: model.PaidFeatureCustomUsergroups,
		})
		require.Nil(t, appErr)

		time.Sleep(5 * time.Second)

		// no error sending second time
		appErr = th.App.SendNotifyAdminPosts(th.Context, "", "", false)
		require.Nil(t, appErr)
	})
}
