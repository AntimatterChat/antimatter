// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/app/accesscontrol"
	"github.com/mattermost/mattermost/server/v8/channels/app/accountmigration"
	"github.com/mattermost/mattermost/server/v8/channels/app/autotranslation"
	"github.com/mattermost/mattermost/server/v8/channels/app/intune"
	"github.com/mattermost/mattermost/server/v8/channels/app/ldap"
	"github.com/mattermost/mattermost/server/v8/channels/app/outgoingoauth"
	"github.com/mattermost/mattermost/server/v8/channels/app/pushproxy"
	"github.com/mattermost/mattermost/server/v8/channels/app/saml"
	"github.com/mattermost/mattermost/server/v8/channels/jobs/compliance"
	"github.com/mattermost/mattermost/server/v8/channels/jobs/dataretention"
	"github.com/mattermost/mattermost/server/v8/channels/jobs/messageexport"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

// TestAntimatterServicesWired checks that a server builds every Antimatter
// implementation of the einterfaces services: they are no longer registered
// from init(), so a constructor call dropped from NewServer, NewChannels,
// initJobs or the platform would leave the feature silently missing.
func TestAntimatterServicesWired(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t)
	srv := th.App.Srv()
	ch := th.App.Channels()

	t.Run("app services", func(t *testing.T) {
		assert.IsType(t, &ldap.Ldap{}, ch.Ldap)
		assert.IsType(t, &saml.Service{}, ch.Saml)
		assert.IsType(t, &intune.Intune{}, ch.Intune)
		assert.IsType(t, &accountmigration.AccountMigration{}, ch.AccountMigration)
		assert.IsType(t, &compliance.Compliance{}, ch.Compliance)
		assert.IsType(t, &dataretention.DataRetention{}, ch.DataRetention)
		assert.IsType(t, &messageexport.MessageExport{}, ch.MessageExport)
		assert.IsType(t, &accesscontrol.Service{}, ch.AccessControl)
		assert.IsType(t, &idLoadedNotification{}, ch.Notification)

		assert.IsType(t, &outgoingoauth.Service{}, srv.OutgoingOAuthConnection)
		assert.IsType(t, &pushproxy.PushProxy{}, srv.PushProxy)
		assert.IsType(t, &autotranslation.Service{}, srv.AutoTranslation)

		// No implementation: stays nil, as before.
		assert.Nil(t, srv.Cloud)
	})

	t.Run("channel exporter", func(t *testing.T) {
		require.NotNil(t, srv.channelExporter)
		assert.Equal(t, reflect.ValueOf(messageexport.ExportChannelCSV).Pointer(), reflect.ValueOf(srv.channelExporter).Pointer())
	})

	t.Run("OAuth providers", func(t *testing.T) {
		for _, service := range []string{model.ServiceGoogle, model.ServiceOffice365, model.ServiceOpenid} {
			assert.NotNil(t, einterfaces.GetOAuthProvider(service), service)
		}
	})

	t.Run("platform services", func(t *testing.T) {
		// The search engine and the access control decision point are checked
		// by the platform's TestPlatformServicesWired: the test helper replaces
		// the server's search engine broker with the shared one of its store.
		ps := srv.Platform()
		assert.IsType(t, &ldap.Diagnostic{}, ps.LdapDiagnostic())
		assert.IsType(t, &saml.Diagnostic{}, ps.SamlDiagnostic())
	})

	t.Run("jobs", func(t *testing.T) {
		require.NotNil(t, srv.Jobs)
		for jobType, wantScheduler := range map[string]bool{
			model.JobTypeLdapSync:                     true,
			model.JobTypeDataRetention:                true,
			model.JobTypeMessageExport:                true,
			model.JobTypeElasticsearchPostIndexing:    false,
			model.JobTypeElasticsearchPostAggregation: true,
			model.JobTypeAccessControlSync:            true,
			model.JobTypeAccessControlTeamSync:        true,
			model.JobTypePushProxyAuth:                true,
			model.JobTypeAutoTranslationRecovery:      true,
		} {
			worker, scheduler := srv.Jobs.JobType(jobType)
			assert.NotNil(t, worker, "worker of %s", jobType)
			if wantScheduler {
				assert.NotNil(t, scheduler, "scheduler of %s", jobType)
			}
		}
	})

	t.Run("error ids shared with the app layer", func(t *testing.T) {
		assert.Equal(t, MissingAuthAccountError, saml.MissingAuthAccountErrorID)
		assert.Equal(t, MissingAccountError, saml.MissingAccountErrorID)
		assert.Equal(t, MissingAuthAccountError, intune.MissingAuthAccountErrorID)
	})
}
