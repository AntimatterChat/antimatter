// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/mattermost/mattermost/server/public/model"
	emailmocks "github.com/mattermost/mattermost/server/v8/channels/app/email/mocks"
	clustermocks "github.com/mattermost/mattermost/server/v8/einterfaces/mocks"
)

func TestRevertHostedPushNotificationServer(t *testing.T) {
	// Not parallel: subtests mutate the shared config and environment.
	th := Setup(t)

	tests := []struct {
		name           string
		initialServer  string
		expectedServer string
	}{
		{
			name:           "reverts Global to TPNS",
			initialServer:  model.MHPNSGlobal,
			expectedServer: model.GenericNotificationServer,
		},
		{
			name:           "reverts regional endpoint (MHPNSUS) to TPNS",
			initialServer:  model.MHPNSUS,
			expectedServer: model.GenericNotificationServer,
		},
		{
			name:           "reverts regional endpoint (MHPNSEU) to TPNS",
			initialServer:  model.MHPNSEU,
			expectedServer: model.GenericNotificationServer,
		},
		{
			name:           "reverts legacy endpoint (MHPNSLegacyDE) to TPNS",
			initialServer:  model.MHPNSLegacyDE,
			expectedServer: model.GenericNotificationServer,
		},
		{
			name:           "leaves TPNS untouched",
			initialServer:  model.GenericNotificationServer,
			expectedServer: model.GenericNotificationServer,
		},
		{
			name:           "leaves custom endpoint untouched",
			initialServer:  "https://push.example.com",
			expectedServer: "https://push.example.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			th.App.UpdateConfig(func(cfg *model.Config) {
				*cfg.EmailSettings.PushNotificationServer = tc.initialServer
				*cfg.EmailSettings.SendPushNotifications = true
			})

			th.Server.revertHostedPushNotificationServer()

			cfg := th.App.Config()
			assert.Equal(t, tc.expectedServer, *cfg.EmailSettings.PushNotificationServer)
			assert.True(t, *cfg.EmailSettings.SendPushNotifications, "SendPushNotifications must never be modified")
		})
	}

	t.Run("environment override leaves setting untouched", func(t *testing.T) {
		t.Setenv("MM_EMAILSETTINGS_PUSHNOTIFICATIONSERVER", model.MHPNSGlobal)

		// The config value alone can't prove the env-override guard fired: a save of this
		// config would be a no-op anyway, because the store re-applies env overrides and
		// skips config listeners when the effective config is unchanged. The one side
		// effect a futile save cannot avoid is cluster propagation — SaveConfig calls
		// ConfigChanged on the cluster interface unconditionally — so probe that to prove
		// the guard returned before saving.
		clusterMock := &clustermocks.ClusterInterface{}
		clusterMock.On("IsLeader").Return(true).Maybe()
		clusterMock.On("GetClusterId").Return("").Maybe()
		clusterMock.On("SendClusterMessage", mock.Anything).Return().Maybe()
		clusterMock.On("RegisterClusterMessageHandler", mock.Anything, mock.Anything).Return().Maybe()
		clusterMock.On("StopInterNodeCommunication").Return().Maybe()
		clusterMock.On("Shutdown").Return().Maybe()
		clusterMock.On("ConfigChanged", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		// The subtest needs its own harness so the mock is installed before the platform
		// starts; swapping the cluster interface mid-test races with platform goroutines
		// that read it. Setup also isolates the env var set above.
		envTh := SetupWithClusterMock(t, clusterMock)

		envTh.Server.revertHostedPushNotificationServer()

		clusterMock.AssertNotCalled(t, "ConfigChanged", mock.Anything, mock.Anything, mock.Anything)
		assert.Equal(t, model.MHPNSGlobal, *envTh.App.Config().EmailSettings.PushNotificationServer)
	})

	t.Run("reverting hosted endpoint does not re-init email batching", func(t *testing.T) {
		originalEmailService := th.App.Srv().EmailService
		t.Cleanup(func() {
			th.App.Srv().EmailService = originalEmailService
		})

		emailServiceMock := emailmocks.ServiceInterface{}
		th.App.Srv().EmailService = &emailServiceMock

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.EmailSettings.PushNotificationServer = model.MHPNSGlobal
		})
		th.Server.revertHostedPushNotificationServer()

		emailServiceMock.AssertNotCalled(t, "InitEmailBatching")
		assert.Equal(t, model.GenericNotificationServer, *th.App.Config().EmailSettings.PushNotificationServer)
	})
}
