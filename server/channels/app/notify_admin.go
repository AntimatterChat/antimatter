// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

const lastTrialNotificationTimeStamp = "LAST_TRIAL_NOTIFICATION_TIMESTAMP"
const lastUpgradeNotificationTimeStamp = "LAST_UPGRADE_NOTIFICATION_TIMESTAMP"
const defaultNotifyAdminCoolOffDays = 14

func (a *App) SaveAdminNotification(userId string, notifyData *model.NotifyAdminToUpgradeRequest) *model.AppError {
	requiredFeature := notifyData.RequiredFeature
	requiredPlan := notifyData.RequiredPlan
	trial := notifyData.TrialNotification

	isUserAlreadyNotified := a.UserAlreadyNotifiedOnRequiredFeature(userId, requiredFeature)
	if isUserAlreadyNotified {
		return model.NewAppError("app.SaveAdminNotification", "api.cloud.notify_admin_to_upgrade_error.already_notified", nil, "", http.StatusForbidden)
	}

	_, appErr := a.SaveAdminNotifyData(&model.NotifyAdminData{
		UserId:          userId,
		RequiredPlan:    requiredPlan,
		RequiredFeature: requiredFeature,
		Trial:           trial,
	})

	if appErr != nil {
		return appErr
	}

	return nil
}

func (a *App) DoCheckForAdminNotifications(trial bool) *model.AppError {
	ctx := request.EmptyContext(a.Srv().Log())
	return a.SendNotifyAdminPosts(ctx, "", "", trial)
}

func (a *App) SaveAdminNotifyData(data *model.NotifyAdminData) (*model.NotifyAdminData, *model.AppError) {
	d, err := a.Srv().Store().NotifyAdmin().Save(data)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("SaveAdminNotifyData", "app.notify_admin.save.app_error", nil, "", http.StatusNotFound).Wrap(nfErr)
		default:
			return nil, model.NewAppError("SaveAdminNotifyData", "app.notify_admin.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
	}

	return d, nil
}

// SendNotifyAdminPosts processes the pending notify-admin requests. Upgrade and trial requests
// no longer produce a post to the system admins (every feature is available); the pending
// requests are only marked as handled, and plugin install requests keep their SentAt updated.
// TODO: the workspace name and current SKU parameters are unused; drop them together with the
// api4 caller.
func (a *App) SendNotifyAdminPosts(rctx request.CTX, _ string, _ string, trial bool) *model.AppError {
	if !a.CanNotifyAdmin(rctx, trial) {
		return model.NewAppError("SendNotifyAdminPosts", "app.notify_admin.send_notification_post.app_error", nil, "Cannot notify yet", http.StatusForbidden)
	}

	now := model.GetMillis()

	data, err := a.Srv().Store().NotifyAdmin().Get(trial)
	if err != nil {
		return model.NewAppError("SendNotifyAdminPosts", "app.notify_admin.send_notification_post.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	if len(data) == 0 {
		rctx.Logger().Warn("No notification data available")
		return nil
	}

	pluginBasedData := a.groupNotifyAdminByPlugin(data)

	a.FinishSendAdminNotifyPost(rctx, trial, now, pluginBasedData)
	return nil
}

func (a *App) UserAlreadyNotifiedOnRequiredFeature(user string, feature model.MattermostFeature) bool {
	data, err := a.Srv().Store().NotifyAdmin().GetDataByUserIdAndFeature(user, feature)
	if err != nil {
		return false
	}
	if len(data) > 0 {
		return true // if we find data, it means this user already notified on the need for this feature
	}

	return false
}

func (a *App) CanNotifyAdmin(rctx request.CTX, trial bool) bool {
	systemVarName := lastUpgradeNotificationTimeStamp
	if trial {
		systemVarName = lastTrialNotificationTimeStamp
	}

	sysVal, sysValErr := a.Srv().Store().System().GetByName(systemVarName)
	if sysValErr != nil {
		var nfErr *store.ErrNotFound
		if errors.As(sysValErr, &nfErr) { // if no timestamps have been recorded before, system is free to notify
			return true
		}
		rctx.Logger().Error("Cannot notify", mlog.Err(sysValErr))
		return false
	}

	lastNotificationTimestamp, err := strconv.ParseFloat(sysVal.Value, 64)
	if err != nil {
		rctx.Logger().Error("Cannot notify", mlog.Err(err))
		return false
	}

	coolOffPeriodDaysEnv := a.Srv().notifyAdminCoolOffDaysOverride
	if coolOffPeriodDaysEnv == "" {
		coolOffPeriodDaysEnv = os.Getenv("MM_NOTIFY_ADMIN_COOL_OFF_DAYS")
	}
	coolOffPeriodDays, parseError := strconv.ParseFloat(coolOffPeriodDaysEnv, 64)
	if parseError != nil {
		coolOffPeriodDays = defaultNotifyAdminCoolOffDays
	}
	daysToMillis := coolOffPeriodDays * 24 * 60 * 60 * 1000
	timeDiff := model.GetMillis() - int64(lastNotificationTimestamp)
	return timeDiff >= int64(daysToMillis)
}

func (a *App) FinishSendAdminNotifyPost(rctx request.CTX, trial bool, now int64, pluginBasedData map[string][]*model.NotifyAdminData) {
	systemVarName := lastUpgradeNotificationTimeStamp
	if trial {
		systemVarName = lastTrialNotificationTimeStamp
	}

	val := strconv.FormatInt(model.GetMillis(), 10)
	sysVar := &model.System{Name: systemVarName, Value: val}
	if err := a.Srv().Store().System().SaveOrUpdate(sysVar); err != nil {
		rctx.Logger().Error("Unable to finish send admin notify post job", mlog.Err(err))
	}

	// All the requested features notifications are now sent in a post and can safely be removed except
	// the plugin notify admin. We keep it as we do not want the same user to send the notification for the same plugin.
	// We update the NotifyAdmin SentAt to keep track of it.
	for pluginId := range pluginBasedData {
		notifications := pluginBasedData[pluginId]
		for _, notification := range notifications {
			requiredFeature := notification.RequiredFeature
			requiredPlan := notification.RequiredPlan
			userId := notification.UserId
			if err := a.Srv().Store().NotifyAdmin().Update(userId, requiredPlan, requiredFeature, now); err != nil {
				rctx.Logger().Error("Unable to update SentAt for work template feature", mlog.Err(err))
			}
		}
	}

	if err := a.Srv().Store().NotifyAdmin().DeleteBefore(trial, now); err != nil {
		rctx.Logger().Error("Unable to finish send admin notify post job", mlog.Err(err))
	}
}

func (a *App) groupNotifyAdminByPlugin(data []*model.NotifyAdminData) map[string][]*model.NotifyAdminData {
	myMap := make(map[string][]*model.NotifyAdminData)
	for _, d := range data {
		if strings.HasPrefix(string(d.RequiredFeature), string(model.PluginFeature)) {
			plugins := strings.SplitSeq(d.RequiredPlan, ",")
			for plugin := range plugins {
				myMap[plugin] = append(myMap[plugin], d)
			}
		}
	}
	return myMap
}
