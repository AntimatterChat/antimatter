// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"github.com/mattermost/mattermost/server/v8/einterfaces"
	ejobs "github.com/mattermost/mattermost/server/v8/einterfaces/jobs"
)

var cloudInterface func(*Server) einterfaces.CloudInterface

func RegisterCloudInterface(f func(*Server) einterfaces.CloudInterface) {
	cloudInterface = f
}

var notificationInterface func(*App) einterfaces.NotificationInterface

func RegisterNotificationInterface(f func(*App) einterfaces.NotificationInterface) {
	notificationInterface = f
}

var accessControlServiceInterface func(*App) einterfaces.AccessControlServiceInterface

func RegisterAccessControlServiceInterface(f func(*App) einterfaces.AccessControlServiceInterface) {
	accessControlServiceInterface = f
}

var jobsAccessControlSyncJobInterface func(*Server) ejobs.AccessControlSyncJobInterface

func RegisterJobsAccessControlSyncJobInterface(f func(*Server) ejobs.AccessControlSyncJobInterface) {
	jobsAccessControlSyncJobInterface = f
}

var jobsAccessControlTeamSyncJobInterface func(*Server) ejobs.AccessControlSyncJobInterface

func RegisterJobsAccessControlTeamSyncJobInterface(f func(*Server) ejobs.AccessControlSyncJobInterface) {
	jobsAccessControlTeamSyncJobInterface = f
}

var pushProxyInterface func(*App) einterfaces.PushProxyInterface

func RegisterPushProxyInterface(f func(*App) einterfaces.PushProxyInterface) {
	pushProxyInterface = f
}

var autoTranslationInterface func(*Server) einterfaces.AutoTranslationInterface

func RegisterAutoTranslationInterface(f func(*Server) einterfaces.AutoTranslationInterface) {
	autoTranslationInterface = f
}

func (s *Server) initEnterprise() {
	if cloudInterface != nil {
		s.Cloud = cloudInterface(s)
	}
	if autoTranslationInterface != nil {
		s.AutoTranslation = autoTranslationInterface(s)
	}
}
