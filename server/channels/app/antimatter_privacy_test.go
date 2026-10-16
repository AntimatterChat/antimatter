// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// TestPushNeverReachesMattermost: whatever the configuration says, no notification or acknowledgement is sent to a
// push service of Mattermost, Inc.; the admin's own push proxy gets them.
func TestPushNeverReachesMattermost(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t)

	var hits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"status":"OK"}`))
	}))
	defer proxy.Close()

	for _, server := range []string{
		model.GenericNotificationServer, model.MHPNSGlobal, model.MHPNSUS, model.MHPNSEU, model.MHPNSAP,
		model.MHPNSLegacyUS, model.MHPNSLegacyDE, "https://PUSH.mattermost.com/",
	} {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.EmailSettings.SendPushNotifications = true
			*cfg.EmailSettings.PushNotificationServer = server
		})
		_, err := th.App.rawSendToPushProxy(&model.PushNotification{Platform: model.PushNotifyApple, DeviceId: "device"})
		require.Error(t, err, server)
		assert.Contains(t, err.Error(), "Mattermost service", server)

		err = th.App.SendAckToPushProxy(th.Context, &model.PushNotificationAck{Id: model.NewId(), NotificationType: model.PushTypeMessage})
		require.Error(t, err, server)
		assert.Contains(t, err.Error(), "Mattermost service", server)
	}

	// The admin's own push proxy is used: the guard isn't simply refusing everything.
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.EmailSettings.PushNotificationServer = proxy.URL
		*cfg.ServiceSettings.AllowedUntrustedInternalConnections = "127.0.0.1"
	})
	_, err := th.App.rawSendToPushProxy(&model.PushNotification{Platform: model.PushNotifyApple, DeviceId: "device"})
	require.NoError(t, err)
	assert.Equal(t, int32(1), hits.Load())
}
