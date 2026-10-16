// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Antimatter never contacts Mattermost, Inc. (UPSTREAM_MERGE_RULES.md, section B). These tests keep its defaults and
// its guard honest; CI's repository-wide outbound-host check covers the rest of the code.

func TestIsMattermostServiceURL(t *testing.T) {
	for _, tc := range []struct {
		url  string
		want bool
	}{
		{"https://mattermost.com", true},
		{"https://push.mattermost.com", true},
		{"https://push-test.mattermost.com/", true},
		{"https://hpns-global.mattermost.com", true},
		{"https://api.integrations.mattermost.com", true},
		{"https://notices.mattermost.com/notices.json", true},
		{"https://PUSH.MatterMost.COM:443/api/v1/send_push", true},
		{"  https://push.mattermost.com  ", true},
		{"http://a.b.c.mattermost.com/x?y=z", true},

		{"", false},
		{"https://push.example.com", false},
		{"https://antimatter.chat", false},
		{"https://notmattermost.com", false},
		{"https://mattermost.com.evil.example", false},
		{"https://example.com/?next=https://push.mattermost.com", false},
		{"https://mattermost.example.com", false},
		{"not a url at all", false},
	} {
		t.Run(tc.url, func(t *testing.T) {
			assert.Equal(t, tc.want, IsMattermostServiceURL(tc.url))
		})
	}

	t.Run("every push service Mattermost runs is recognised", func(t *testing.T) {
		for _, u := range []string{GenericNotificationServer, MHPNSLegacyUS, MHPNSLegacyDE, MHPNSGlobal, MHPNSUS, MHPNSEU, MHPNSAP, MHPNS} {
			assert.True(t, IsMattermostPushServer(u), u)
		}
	})
}

// mattermostHost matches hosts of Mattermost, Inc. and of the telemetry and error reporting services upstream used.
var mattermostHost = regexp.MustCompile(`(?i)(^|[^a-z0-9-])([a-z0-9-]+\.)*(mattermost\.(com|cloud|org|io)|matterlytics|sentry\.io|rudderstack\.com|rudderlabs\.com|segment\.(io|com))\b|stun\.global\.calls`)

// defaultsToFix are default settings that still point at a Mattermost site and should not. They are links shown to
// users, not addresses the server calls: the "download the apps" links send people to Mattermost's apps, not
// Antimatter's. Point them at Antimatter's own download pages (or leave them empty) and remove them from here; the test
// fails until you do, so this list only shrinks.
var defaultsToFix = map[string]bool{
	"Config.NativeAppSettings.AppDownloadLink":        true,
	"Config.NativeAppSettings.AndroidAppDownloadLink": true,
	"Config.NativeAppSettings.IosAppDownloadLink":     true,
}

// TestDefaultConfigContactsNoMattermostService walks every string of the default configuration (fields, slices, map
// keys and values) and fails on any Mattermost, telemetry or error reporting host.
func TestDefaultConfigContactsNoMattermostService(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	var visited int
	var walk func(path string, v reflect.Value)
	walk = func(path string, v reflect.Value) {
		switch v.Kind() {
		case reflect.Pointer, reflect.Interface:
			if !v.IsNil() {
				walk(path, v.Elem())
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() {
					walk(path+"."+v.Type().Field(i).Name, v.Field(i))
				}
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				walk(fmt.Sprintf("%s[%d]", path, i), v.Index(i))
			}
		case reflect.Map:
			iter := v.MapRange()
			for iter.Next() {
				walk(path+"{key}", iter.Key())
				walk(fmt.Sprintf("%s[%v]", path, iter.Key()), iter.Value())
			}
		case reflect.String:
			visited++
			if defaultsToFix[path] {
				assert.True(t, mattermostHost.MatchString(v.String()), "%s is fixed: remove it from defaultsToFix", path)
				return
			}
			assert.False(t, mattermostHost.MatchString(v.String()), "%s defaults to %q, a Mattermost or telemetry host", path, v.String())
		}
	}
	walk("Config", reflect.ValueOf(cfg))
	require.Greater(t, visited, 250, "the walk missed most of the configuration")
}

// TestDefaultConfigPhonesNoHome checks the switches and addresses that would make a fresh server reach out by itself.
func TestDefaultConfigPhonesNoHome(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	assert.Empty(t, *cfg.EmailSettings.PushNotificationServer, "no push service until the admin sets their own push proxy")
	assert.Empty(t, *cfg.PluginSettings.MarketplaceURL, "no app marketplace by default")
	assert.Empty(t, *cfg.AnnouncementSettings.NoticesURL, "no product notices")
	assert.False(t, *cfg.AnnouncementSettings.AdminNoticesEnabled)
	assert.False(t, *cfg.AnnouncementSettings.UserNoticesEnabled)
	assert.False(t, *cfg.ServiceSettings.EnableSecurityFixAlert, "no security update check")
	assert.False(t, *cfg.LogSettings.EnableDiagnostics, "no telemetry")
	assert.False(t, *cfg.LogSettings.EnableSentry, "no error reporting")
	if state, ok := cfg.PluginSettings.PluginStates[PluginIdNPS]; ok {
		assert.False(t, state.Enable, "the NPS survey plugin stays off")
	}

	t.Run("an old configuration naming Mattermost's services gets none", func(t *testing.T) {
		old := &Config{}
		old.EmailSettings.PushNotificationServer = NewPointer(MHPNSGlobal)
		old.PluginSettings.MarketplaceURL = NewPointer("https://api.integrations.mattermost.com")
		old.AnnouncementSettings.NoticesURL = NewPointer("https://notices.mattermost.com/")
		old.SetDefaults()
		assert.Empty(t, *old.PluginSettings.MarketplaceURL)
		assert.Empty(t, *old.AnnouncementSettings.NoticesURL)
		// The push server is kept as the admin set it (the System Console shows a warning), but nothing is sent to it.
		assert.True(t, IsMattermostPushServer(*old.EmailSettings.PushNotificationServer))
	})
}

// TestAntimatterSettingDefaults pins the defaults of Antimatter's own settings that bear on security and privacy.
func TestAntimatterSettingDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	// No proxy header is trusted until the admin names one: otherwise anyone could forge their address.
	assert.Empty(t, cfg.ServiceSettings.TrustedProxyIPHeader)
	assert.Empty(t, cfg.ServiceSettings.TrustedProxyIPs)
	assert.True(t, *cfg.PrivacySettings.EnableUserBlocking, "people can block and ignore others")
	assert.Equal(t, WebUIFusion, *cfg.ServiceSettings.DefaultWebUI)
	assert.True(t, *cfg.ServiceSettings.AllowUserWebUISelection)
	assert.True(t, *cfg.TeamSettings.AllChannelsMembershipForNewTeams)
	assert.False(t, *cfg.TeamSettings.RestrictChannelExportToAdmins)
	assert.True(t, *cfg.TeamSettings.MemberListRoleGroups)
	assert.False(t, *cfg.TextToSpeechSettings.Enable, "no text to speech service until the admin sets one")
	assert.Empty(t, *cfg.TextToSpeechSettings.URL)
	assert.Empty(t, *cfg.AutoTranslationSettings.OpenAI.URL, "no translation service until the admin sets one")
}

// secretField names settings that hold a secret.
var secretField = regexp.MustCompile(`(?i)(secret|password|passwd|apikey|api_key|token|privatekey|salt|signingkey|accesskey$|sdkkey|developerkey|splitkey|datasource)`)

// notSecret are settings whose name looks like a secret's but that hold none.
var notSecret = []string{
	"Config.ServiceSettings.EnableUserAccessTokens",
	"Config.ServiceSettings.TLSKeyFile",
	"Config.ServiceSettings.TrustedProxyIPHeader",
	"Config.GitLabSettings.TokenEndpoint",
	"Config.GoogleSettings.TokenEndpoint",
	"Config.Office365Settings.TokenEndpoint",
	"Config.OpenIdSettings.TokenEndpoint",
	"Config.SamlSettings.PrivateKeyFile",
	"Config.SamlSettings.PublicCertificateFile",
	"Config.ElasticsearchSettings.ClientKey",
	"Config.SupportSettings.ForgotPasswordLink",
	"Config.LdapSettings.PrivateKeyFile",
}

// TestSanitizeHidesEverySecret fills every setting whose name says it holds a secret, Antimatter's included (the text
// to speech and translation API keys), and checks the configuration the System Console and support packets get shows
// none of them.
func TestSanitizeHidesEverySecret(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	*cfg.SqlSettings.DriverName = DatabaseDriverPostgres

	const secret = "s3cr3t-value"
	var filled []string
	var walk func(path string, v reflect.Value, fill bool)
	walk = func(path string, v reflect.Value, fill bool) {
		switch v.Kind() {
		case reflect.Pointer:
			if !v.IsNil() {
				walk(path, v.Elem(), fill)
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() {
					walk(path+"."+v.Type().Field(i).Name, v.Field(i), fill)
				}
			}
		case reflect.Slice:
			if v.Type().Elem().Kind() == reflect.String && secretField.MatchString(path) && !slices.Contains(notSecret, path) {
				if fill {
					v.Set(reflect.ValueOf([]string{secret}))
					filled = append(filled, path)
					return
				}
				for i := 0; i < v.Len(); i++ {
					assert.NotEqual(t, secret, v.Index(i).String(), "%s isn't sanitized", path)
				}
			}
		case reflect.String:
			if !secretField.MatchString(path) || slices.Contains(notSecret, path) {
				return
			}
			if fill {
				v.SetString(secret)
				filled = append(filled, path)
				return
			}
			assert.NotContains(t, v.String(), secret, "%s isn't sanitized", path)
		}
	}
	walk("Config", reflect.ValueOf(cfg), true)
	require.Contains(t, filled, "Config.TextToSpeechSettings.APIKey")
	require.Contains(t, filled, "Config.AutoTranslationSettings.OpenAI.APIKey")
	require.Contains(t, filled, "Config.SqlSettings.DataSource")

	t.Logf("secrets checked: %v", filled)

	cfg.Sanitize(nil, nil)
	walk("Config", reflect.ValueOf(cfg), false)
}
