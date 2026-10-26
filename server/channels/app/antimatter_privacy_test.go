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

// TestPushNeverReachesMattermost: unless the admin allowed it (EmailSettings.AllowMattermostPushService, off by
// default), no notification or acknowledgement is sent to a push service of Mattermost, Inc., whatever the push
// notification server says; the admin's own push proxy gets them.
func TestPushNeverReachesMattermost(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t)
	require.False(t, *th.App.Config().EmailSettings.AllowMattermostPushService, "off by default")

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

// roundTripFunc stands in for the network: TestPushToMattermostOnlyWhenAllowed must never reach mattermost.com.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestPushToMattermostOnlyWhenAllowed: with EmailSettings.AllowMattermostPushService on, notifications and
// acknowledgements go to the Mattermost push service the admin set (so the official mobile apps get them); turned
// off again, nothing is sent there.
func TestPushToMattermostOnlyWhenAllowed(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t)

	var requested []string
	client := th.App.Srv().pushNotificationClient
	th.App.Srv().pushNotificationClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requested = append(requested, r.Method+" "+r.URL.String())
		rec := httptest.NewRecorder()
		_, _ = rec.WriteString(`{"status":"OK"}`)
		return rec.Result(), nil
	})}
	t.Cleanup(func() { th.App.Srv().pushNotificationClient = client })

	send := func() (sendErr, ackErr error) {
		_, sendErr = th.App.rawSendToPushProxy(&model.PushNotification{Platform: model.PushNotifyApple, DeviceId: "device"})
		ackErr = th.App.SendAckToPushProxy(th.Context, &model.PushNotificationAck{Id: model.NewId(), NotificationType: model.PushTypeMessage})
		return sendErr, ackErr
	}

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.EmailSettings.SendPushNotifications = true
		*cfg.EmailSettings.PushNotificationServer = model.MHPNSGlobal
		*cfg.EmailSettings.AllowMattermostPushService = true
	})
	assert.True(t, th.App.canSendPushNotifications())
	sendErr, ackErr := send()
	require.NoError(t, sendErr)
	require.NoError(t, ackErr)
	assert.Equal(t, []string{
		"POST " + model.MHPNSGlobal + model.APIURLSuffixV1 + "/send_push",
		"POST " + model.MHPNSGlobal + model.APIURLSuffixV1 + "/ack",
	}, requested)

	requested = nil
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.EmailSettings.AllowMattermostPushService = false
	})
	assert.False(t, th.App.canSendPushNotifications())
	sendErr, ackErr = send()
	require.Error(t, sendErr)
	require.Error(t, ackErr)
	assert.Empty(t, requested, "nothing reaches Mattermost once the opt-in is off")
}
