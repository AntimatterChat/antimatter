// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/fips"
)

// GenerateClientConfig renders the given configuration for a client.
func GenerateClientConfig(c *model.Config, telemetryID string) map[string]string {
	props := GenerateLimitedClientConfig(c, telemetryID)

	props["EnableCustomUserStatuses"] = strconv.FormatBool(*c.TeamSettings.EnableCustomUserStatuses)
	props["EnableLastActiveTime"] = strconv.FormatBool(*c.TeamSettings.EnableLastActiveTime)
	props["EnableUserDeactivation"] = strconv.FormatBool(*c.TeamSettings.EnableUserDeactivation)
	props["RestrictDirectMessage"] = *c.TeamSettings.RestrictDirectMessage
	props["TeammateNameDisplay"] = *c.TeamSettings.TeammateNameDisplay
	props["LockTeammateNameDisplay"] = strconv.FormatBool(*c.TeamSettings.LockTeammateNameDisplay)
	props["ExperimentalPrimaryTeam"] = *c.TeamSettings.ExperimentalPrimaryTeam
	props["EnableJoinLeaveMessageByDefault"] = strconv.FormatBool(*c.TeamSettings.EnableJoinLeaveMessageByDefault)
	props["EnableChannelCategorySorting"] = strconv.FormatBool(*c.TeamSettings.EnableChannelCategorySorting)

	props["EnableBotAccountCreation"] = strconv.FormatBool(*c.ServiceSettings.EnableBotAccountCreation)
	props["EnableDesktopLandingPage"] = strconv.FormatBool(*c.ServiceSettings.EnableDesktopLandingPage)
	props["DefaultWebUI"] = DefaultWebUI(c)
	props["AllowUserWebUISelection"] = strconv.FormatBool(UserWebUISelectionAllowed(c))
	props["EnableOAuthServiceProvider"] = strconv.FormatBool(*c.ServiceSettings.EnableOAuthServiceProvider)
	props["GoogleDeveloperKey"] = *c.ServiceSettings.GoogleDeveloperKey
	props["EnableIncomingWebhooks"] = strconv.FormatBool(*c.ServiceSettings.EnableIncomingWebhooks)
	props["EnableOutgoingWebhooks"] = strconv.FormatBool(*c.ServiceSettings.EnableOutgoingWebhooks)
	props["EnableOutgoingOAuthConnections"] = strconv.FormatBool(*c.ServiceSettings.EnableOutgoingOAuthConnections)
	props["EnableCommands"] = strconv.FormatBool(*c.ServiceSettings.EnableCommands)
	props["EnablePostUsernameOverride"] = strconv.FormatBool(*c.ServiceSettings.EnablePostUsernameOverride)
	props["EnablePostIconOverride"] = strconv.FormatBool(*c.ServiceSettings.EnablePostIconOverride)
	props["EnableUserAccessTokens"] = strconv.FormatBool(*c.ServiceSettings.EnableUserAccessTokens)
	props["MaximumPersonalAccessTokenLifetimeDays"] = strconv.Itoa(*c.ServiceSettings.MaximumPersonalAccessTokenLifetimeDays)
	props["EnableLinkPreviews"] = strconv.FormatBool(*c.ServiceSettings.EnableLinkPreviews)
	props["EnablePermalinkPreviews"] = strconv.FormatBool(*c.ServiceSettings.EnablePermalinkPreviews)
	props["EnableInlineReplies"] = strconv.FormatBool(*c.ServiceSettings.EnableInlineReplies)
	props["EnableTesting"] = strconv.FormatBool(*c.ServiceSettings.EnableTesting)
	props["EnableDeveloper"] = strconv.FormatBool(*c.ServiceSettings.EnableDeveloper)
	props["EnableClientPerformanceDebugging"] = strconv.FormatBool(*c.ServiceSettings.EnableClientPerformanceDebugging)
	props["PostEditTimeLimit"] = strconv.Itoa(*c.ServiceSettings.PostEditTimeLimit)
	props["MinimumHashtagLength"] = strconv.Itoa(*c.ServiceSettings.MinimumHashtagLength)
	props["EnableTutorial"] = strconv.FormatBool(*c.ServiceSettings.EnableTutorial)
	props["EnableOnboardingFlow"] = strconv.FormatBool(*c.ServiceSettings.EnableOnboardingFlow)
	props["ExperimentalEnableDefaultChannelLeaveJoinMessages"] = strconv.FormatBool(*c.ServiceSettings.ExperimentalEnableDefaultChannelLeaveJoinMessages)
	props["ExperimentalGroupUnreadChannels"] = *c.ServiceSettings.ExperimentalGroupUnreadChannels
	props["EnableSVGs"] = strconv.FormatBool(*c.ServiceSettings.EnableSVGs)
	props["EnableMarketplace"] = strconv.FormatBool(*c.PluginSettings.EnableMarketplace)
	props["EnableLatex"] = strconv.FormatBool(*c.ServiceSettings.EnableLatex)
	props["EnableInlineLatex"] = strconv.FormatBool(*c.ServiceSettings.EnableInlineLatex)
	props["ExtendSessionLengthWithActivity"] = strconv.FormatBool(*c.ServiceSettings.ExtendSessionLengthWithActivity)
	props["ManagedResourcePaths"] = *c.ServiceSettings.ManagedResourcePaths
	props["DeleteAccountLink"] = *c.ServiceSettings.DeleteAccountLink

	// This setting is only temporary, so keep using the old setting name for the mobile and web apps
	props["ExperimentalEnablePostMetadata"] = "true"

	props["DisableAppBar"] = strconv.FormatBool(*c.ExperimentalSettings.DisableAppBar)

	props["ExperimentalEnableAutomaticReplies"] = strconv.FormatBool(*c.TeamSettings.ExperimentalEnableAutomaticReplies)
	props["ExperimentalTimezone"] = "true"

	props["SendEmailNotifications"] = strconv.FormatBool(*c.EmailSettings.SendEmailNotifications)
	props["SendPushNotifications"] = strconv.FormatBool(*c.EmailSettings.SendPushNotifications)
	props["RequireEmailVerification"] = strconv.FormatBool(*c.EmailSettings.RequireEmailVerification)
	props["EnableEmailBatching"] = strconv.FormatBool(*c.EmailSettings.EnableEmailBatching)
	props["EnablePreviewModeBanner"] = strconv.FormatBool(*c.EmailSettings.EnablePreviewModeBanner)
	props["EmailNotificationContentsType"] = *c.EmailSettings.EmailNotificationContentsType

	props["ShowEmailAddress"] = strconv.FormatBool(*c.PrivacySettings.ShowEmailAddress)
	props["ShowFullName"] = strconv.FormatBool(*c.PrivacySettings.ShowFullName)
	props["UseAnonymousURLs"] = strconv.FormatBool(*c.PrivacySettings.UseAnonymousURLs)

	props["EnableFileAttachments"] = strconv.FormatBool(*c.FileSettings.EnableFileAttachments)
	props["EnablePublicLink"] = strconv.FormatBool(*c.FileSettings.EnablePublicLink)

	props["AvailableLocales"] = *c.LocalizationSettings.AvailableLocales

	props["SQLDriverName"] = *c.SqlSettings.DriverName

	props["EnableEmojiPicker"] = strconv.FormatBool(*c.ServiceSettings.EnableEmojiPicker)
	props["EnableGifPicker"] = strconv.FormatBool(*c.ServiceSettings.EnableGifPicker)
	props["GiphySdkKey"] = getGiphySdkKey(c.ServiceSettings)
	props["MaxFileSize"] = strconv.FormatInt(*c.FileSettings.MaxFileSize, 10)

	props["MaxNotificationsPerChannel"] = strconv.FormatInt(*c.TeamSettings.MaxNotificationsPerChannel, 10)
	props["EnableConfirmNotificationsToChannel"] = strconv.FormatBool(*c.TeamSettings.EnableConfirmNotificationsToChannel)
	props["TimeBetweenUserTypingUpdatesMilliseconds"] = strconv.FormatInt(*c.ServiceSettings.TimeBetweenUserTypingUpdatesMilliseconds, 10)
	props["EnableUserTypingMessages"] = strconv.FormatBool(*c.ServiceSettings.EnableUserTypingMessages)
	props["EnableChannelViewedMessages"] = strconv.FormatBool(*c.ServiceSettings.EnableChannelViewedMessages)

	props["RunJobs"] = strconv.FormatBool(*c.JobSettings.RunJobs)

	props["EnableEmailInvitations"] = strconv.FormatBool(*c.ServiceSettings.EnableEmailInvitations)

	props["CWSURL"] = *c.CloudSettings.CWSURL
	props["CWSMock"] = model.MockCWS

	props["DisableRefetchingOnBrowserFocus"] = strconv.FormatBool(*c.ExperimentalSettings.DisableRefetchingOnBrowserFocus)
	props["DisableWakeUpReconnectHandler"] = strconv.FormatBool(*c.ExperimentalSettings.DisableWakeUpReconnectHandler)
	props["UsersStatusAndProfileFetchingPollIntervalMilliseconds"] = strconv.FormatInt(*c.ExperimentalSettings.UsersStatusAndProfileFetchingPollIntervalMilliseconds, 10)
	// Here we set the new option, but we also send the old FeatureFlag property for backwards compatibility on mobile < 2.27
	props["EnableCrossTeamSearch"] = strconv.FormatBool(*c.ServiceSettings.EnableCrossTeamSearch)
	props["FeatureFlagExperimentalCrossTeamSearch"] = strconv.FormatBool(*c.ServiceSettings.EnableCrossTeamSearch)

	props["ExperimentalEnableAuthenticationTransfer"] = strconv.FormatBool(*c.ServiceSettings.ExperimentalEnableAuthenticationTransfer)

	props["LdapNicknameAttributeSet"] = strconv.FormatBool(*c.LdapSettings.NicknameAttribute != "")
	props["LdapFirstNameAttributeSet"] = strconv.FormatBool(*c.LdapSettings.FirstNameAttribute != "")
	props["LdapLastNameAttributeSet"] = strconv.FormatBool(*c.LdapSettings.LastNameAttribute != "")
	props["LdapPictureAttributeSet"] = strconv.FormatBool(*c.LdapSettings.PictureAttribute != "")
	props["LdapPositionAttributeSet"] = strconv.FormatBool(*c.LdapSettings.PositionAttribute != "")

	props["EnableCompliance"] = strconv.FormatBool(*c.ComplianceSettings.Enable)
	props["EnableMobileFileDownload"] = strconv.FormatBool(*c.FileSettings.EnableMobileDownload)
	props["EnableMobileFileUpload"] = strconv.FormatBool(*c.FileSettings.EnableMobileUpload)

	props["SamlFirstNameAttributeSet"] = strconv.FormatBool(*c.SamlSettings.FirstNameAttribute != "")
	props["SamlLastNameAttributeSet"] = strconv.FormatBool(*c.SamlSettings.LastNameAttribute != "")
	props["SamlNicknameAttributeSet"] = strconv.FormatBool(*c.SamlSettings.NicknameAttribute != "")
	props["SamlPositionAttributeSet"] = strconv.FormatBool(*c.SamlSettings.PositionAttribute != "")

	props["EnableCluster"] = strconv.FormatBool(*c.ClusterSettings.Enable)

	props["EnableMetrics"] = strconv.FormatBool(*c.MetricsSettings.Enable)
	props["EnableClientMetrics"] = strconv.FormatBool(*c.MetricsSettings.Enable && *c.MetricsSettings.EnableClientMetrics)
	props["EnableNotificationMetrics"] = strconv.FormatBool(*c.MetricsSettings.EnableNotificationMetrics)

	props["EnableBanner"] = strconv.FormatBool(*c.AnnouncementSettings.EnableBanner)
	props["BannerText"] = *c.AnnouncementSettings.BannerText
	props["BannerColor"] = *c.AnnouncementSettings.BannerColor
	props["BannerTextColor"] = *c.AnnouncementSettings.BannerTextColor
	props["AllowBannerDismissal"] = strconv.FormatBool(*c.AnnouncementSettings.AllowBannerDismissal)

	props["EnableThemeSelection"] = strconv.FormatBool(*c.ThemeSettings.EnableThemeSelection)
	props["DefaultTheme"] = *c.ThemeSettings.DefaultTheme
	props["AllowCustomThemes"] = strconv.FormatBool(*c.ThemeSettings.AllowCustomThemes)
	props["AllowedThemes"] = strings.Join(c.ThemeSettings.AllowedThemes, ",")

	props["DataRetentionEnableMessageDeletion"] = strconv.FormatBool(*c.DataRetentionSettings.EnableMessageDeletion)
	props["DataRetentionMessageRetentionHours"] = strconv.FormatInt(int64(c.DataRetentionSettings.GetMessageRetentionHours()), 10)
	props["DataRetentionEnableFileDeletion"] = strconv.FormatBool(*c.DataRetentionSettings.EnableFileDeletion)
	props["DataRetentionFileRetentionHours"] = strconv.FormatInt(int64(c.DataRetentionSettings.GetFileRetentionHours()), 10)

	props["ExperimentalSharedChannels"] = strconv.FormatBool(*c.ConnectedWorkspacesSettings.EnableSharedChannels)
	props["ExperimentalRemoteClusterService"] = strconv.FormatBool(*c.ConnectedWorkspacesSettings.EnableRemoteClusterService)

	props["EnableCustomGroups"] = strconv.FormatBool(*c.ServiceSettings.EnableCustomGroups)
	props["PostAcknowledgements"] = "true"
	props["ScheduledPosts"] = strconv.FormatBool(*c.ServiceSettings.ScheduledPosts)

	props["LockProfileFieldsForEmailUsers"] = *c.TeamSettings.LockProfileFieldsForEmailUsers

	props["MobileEnableSecureFilePreview"] = strconv.FormatBool(*c.NativeAppSettings.MobileEnableSecureFilePreview)
	props["MobileAllowPdfLinkNavigation"] = strconv.FormatBool(*c.NativeAppSettings.MobileAllowPdfLinkNavigation)

	props["ContentFlaggingEnabled"] = strconv.FormatBool(c.FeatureFlags.ContentFlagging && *c.ContentFlaggingSettings.EnableContentFlagging)
	props["EnableAutoTranslation"] = strconv.FormatBool(c.FeatureFlags.AutoTranslation && *c.AutoTranslationSettings.Enable)
	if c.FeatureFlags.AutoTranslation {
		props["AutoTranslationLanguages"] = strings.Join(*c.AutoTranslationSettings.TargetLanguages, ",")
	} else {
		props["AutoTranslationLanguages"] = ""
	}
	props["RestrictDMAndGMAutotranslation"] = strconv.FormatBool(*c.AutoTranslationSettings.RestrictDMAndGM)

	if c.FeatureFlags.MobileEphemeralMode {
		ephemeralEnabled := c.MobileEphemeralModeSettings.Enable != nil && *c.MobileEphemeralModeSettings.Enable
		props["MobileEphemeralModeEnabled"] = strconv.FormatBool(ephemeralEnabled)
		if c.MobileEphemeralModeSettings.DisconnectionTimeoutSeconds != nil {
			props["MobileEphemeralModeDisconnectionTimeoutSeconds"] = strconv.Itoa(*c.MobileEphemeralModeSettings.DisconnectionTimeoutSeconds)
		}
		if c.MobileEphemeralModeSettings.OfflinePersistenceTimerHours != nil {
			props["MobileEphemeralModeOfflinePersistenceTimerHours"] = strconv.Itoa(*c.MobileEphemeralModeSettings.OfflinePersistenceTimerHours)
		}
		if c.MobileEphemeralModeSettings.AutoCacheCleanupDays != nil {
			props["MobileEphemeralModeAutoCacheCleanupDays"] = strconv.Itoa(*c.MobileEphemeralModeSettings.AutoCacheCleanupDays)
		}
	}

	props["CustomUrlSchemes"] = strings.Join(c.DisplaySettings.CustomURLSchemes, ",")
	props["MaxMarkdownNodes"] = strconv.FormatInt(int64(*c.DisplaySettings.MaxMarkdownNodes), 10)
	props["IsDefaultMarketplace"] = strconv.FormatBool(*c.PluginSettings.MarketplaceURL == model.PluginSettingsDefaultMarketplaceURL)
	props["CollapsedThreads"] = *c.ServiceSettings.CollapsedThreads
	props["PostPriority"] = strconv.FormatBool(*c.ServiceSettings.PostPriority)
	props["AllowPersistentNotifications"] = strconv.FormatBool(*c.ServiceSettings.AllowPersistentNotifications)
	props["AllowPersistentNotificationsForGuests"] = strconv.FormatBool(*c.ServiceSettings.AllowPersistentNotificationsForGuests)
	props["PersistentNotificationMaxCount"] = strconv.FormatInt(int64(*c.ServiceSettings.PersistentNotificationMaxCount), 10)
	props["PersistentNotificationIntervalMinutes"] = strconv.FormatInt(int64(*c.ServiceSettings.PersistentNotificationIntervalMinutes), 10)
	props["PersistentNotificationMaxRecipients"] = strconv.FormatInt(int64(*c.ServiceSettings.PersistentNotificationMaxRecipients), 10)
	props["EnableBurnOnRead"] = strconv.FormatBool(*c.ServiceSettings.EnableBurnOnRead)
	props["BurnOnReadDurationSeconds"] = strconv.Itoa(*c.ServiceSettings.BurnOnReadDurationSeconds)
	props["BurnOnReadMaximumTimeToLiveSeconds"] = strconv.Itoa(*c.ServiceSettings.BurnOnReadMaximumTimeToLiveSeconds)
	props["AllowSyncedDrafts"] = strconv.FormatBool(*c.ServiceSettings.AllowSyncedDrafts)
	props["DelayChannelAutocomplete"] = strconv.FormatBool(*c.ExperimentalSettings.DelayChannelAutocomplete)
	props["YoutubeReferrerPolicy"] = strconv.FormatBool(*c.ExperimentalSettings.YoutubeReferrerPolicy)
	props["UniqueEmojiReactionLimitPerPost"] = strconv.FormatInt(int64(*c.ServiceSettings.UniqueEmojiReactionLimitPerPost), 10)

	props["EnableAttributeBasedAccessControl"] = strconv.FormatBool(*c.AccessControlSettings.EnableAttributeBasedAccessControl)
	props["EnableUserManagedAttributes"] = strconv.FormatBool(*c.AccessControlSettings.EnableUserManagedAttributes)
	props["EnableAccessControlAuditLogging"] = strconv.FormatBool(*c.AccessControlSettings.EnableAccessControlAuditLogging)
	props["AuditLoggingActive"] = strconv.FormatBool(IsAuditLoggingActive(c.ExperimentalAuditSettings))
	props["EnableChannelPolicyIndicators"] = strconv.FormatBool(*c.AccessControlSettings.EnableChannelPolicyIndicators)

	props["WranglerPermittedWranglerRoles"] = strings.Join(c.WranglerSettings.PermittedWranglerRoles, ",")
	props["WranglerAllowedEmailDomain"] = strings.Join(c.WranglerSettings.AllowedEmailDomain, ",")
	props["WranglerMoveThreadMaxCount"] = strconv.FormatInt(*c.WranglerSettings.MoveThreadMaxCount, 10)
	props["WranglerMoveThreadToAnotherTeamEnable"] = strconv.FormatBool(*c.WranglerSettings.MoveThreadToAnotherTeamEnable)
	props["WranglerMoveThreadFromPrivateChannelEnable"] = strconv.FormatBool(*c.WranglerSettings.MoveThreadFromPrivateChannelEnable)
	props["WranglerMoveThreadFromDirectMessageChannelEnable"] = strconv.FormatBool(*c.WranglerSettings.MoveThreadFromDirectMessageChannelEnable)
	props["WranglerMoveThreadFromGroupMessageChannelEnable"] = strconv.FormatBool(*c.WranglerSettings.MoveThreadFromGroupMessageChannelEnable)

	return props
}

// GenerateLimitedClientConfig renders the given configuration for an untrusted client.
func GenerateLimitedClientConfig(c *model.Config, telemetryID string) map[string]string {
	props := make(map[string]string)

	props["Version"] = model.CurrentVersion
	props["BuildNumber"] = model.BuildNumber
	props["BuildDate"] = model.BuildDate
	props["BuildHash"] = model.BuildHash
	props["ServiceEnvironment"] = model.GetServiceEnvironment()
	props["IsFipsEnabled"] = strconv.FormatBool(fips.IsEnabled)

	props["EnableBotAccountCreation"] = strconv.FormatBool(*c.ServiceSettings.EnableBotAccountCreation)
	props["EnableDesktopLandingPage"] = strconv.FormatBool(*c.ServiceSettings.EnableDesktopLandingPage)
	props["EnableFile"] = strconv.FormatBool(*c.LogSettings.EnableFile)
	props["FileLevel"] = *c.LogSettings.FileLevel

	props["SiteURL"] = strings.TrimRight(*c.ServiceSettings.SiteURL, "/")
	props["SiteName"] = *c.TeamSettings.SiteName
	props["WebsocketURL"] = strings.TrimRight(*c.ServiceSettings.WebsocketURL, "/")
	props["WebsocketPort"] = strconv.Itoa(*c.ServiceSettings.WebsocketPort)
	props["WebsocketSecurePort"] = strconv.Itoa(*c.ServiceSettings.WebsocketSecurePort)
	props["EnableUserCreation"] = strconv.FormatBool(*c.TeamSettings.EnableUserCreation)
	props["EnableOpenServer"] = strconv.FormatBool(*c.TeamSettings.EnableOpenServer)
	props["EnableJoinLeaveMessageByDefault"] = strconv.FormatBool(*c.TeamSettings.EnableJoinLeaveMessageByDefault)

	props["AndroidLatestVersion"] = c.ClientRequirements.AndroidLatestVersion
	props["AndroidMinVersion"] = c.ClientRequirements.AndroidMinVersion
	props["IosLatestVersion"] = c.ClientRequirements.IosLatestVersion
	props["IosMinVersion"] = c.ClientRequirements.IosMinVersion

	props["EnableDiagnostics"] = strconv.FormatBool(*c.LogSettings.EnableDiagnostics)
	props["EnableClientMetrics"] = strconv.FormatBool(*c.MetricsSettings.EnableClientMetrics)

	props["EnableComplianceExport"] = strconv.FormatBool(*c.MessageExportSettings.EnableExport)

	props["EnableSignUpWithEmail"] = strconv.FormatBool(*c.EmailSettings.EnableSignUpWithEmail)
	props["EnableSignInWithEmail"] = strconv.FormatBool(*c.EmailSettings.EnableSignInWithEmail)
	props["EnableSignInWithUsername"] = strconv.FormatBool(*c.EmailSettings.EnableSignInWithUsername)

	props["TermsOfServiceLink"] = *c.SupportSettings.TermsOfServiceLink
	props["PrivacyPolicyLink"] = *c.SupportSettings.PrivacyPolicyLink
	props["AboutLink"] = *c.SupportSettings.AboutLink
	props["HelpLink"] = *c.SupportSettings.HelpLink
	props["ReportAProblemType"] = *c.SupportSettings.ReportAProblemType
	props["ReportAProblemLink"] = *c.SupportSettings.ReportAProblemLink
	props["ReportAProblemMail"] = *c.SupportSettings.ReportAProblemMail
	props["AllowDownloadLogs"] = strconv.FormatBool(*c.SupportSettings.AllowDownloadLogs)
	props["ForgotPasswordLink"] = *c.SupportSettings.ForgotPasswordLink
	props["SupportEmail"] = *c.SupportSettings.SupportEmail
	props["EnableAskCommunityLink"] = strconv.FormatBool(*c.SupportSettings.EnableAskCommunityLink)

	props["DefaultClientLocale"] = *c.LocalizationSettings.DefaultClientLocale

	props["EnableCustomEmoji"] = strconv.FormatBool(*c.ServiceSettings.EnableCustomEmoji)
	props["EnableUserStatuses"] = strconv.FormatBool(*c.ServiceSettings.EnableUserStatuses)
	props["AppDownloadLink"] = *c.NativeAppSettings.AppDownloadLink
	props["AndroidAppDownloadLink"] = *c.NativeAppSettings.AndroidAppDownloadLink
	props["IosAppDownloadLink"] = *c.NativeAppSettings.IosAppDownloadLink
	props["MobileExternalBrowser"] = strconv.FormatBool(*c.NativeAppSettings.MobileExternalBrowser)

	props["DiagnosticId"] = telemetryID
	props["TelemetryId"] = telemetryID
	props["DiagnosticsEnabled"] = strconv.FormatBool(*c.LogSettings.EnableDiagnostics)

	props["HasImageProxy"] = strconv.FormatBool(*c.ImageProxySettings.Enable)

	props["PluginsEnabled"] = strconv.FormatBool(*c.PluginSettings.Enable)
	props["AppsPluginEnabled"] = strconv.FormatBool(*c.PluginSettings.Enable && c.PluginSettings.PluginStates[model.PluginIdApps] != nil && c.PluginSettings.PluginStates[model.PluginIdApps].Enable)

	props["PasswordMinimumLength"] = strconv.Itoa(*c.PasswordSettings.MinimumLength)
	props["PasswordRequireLowercase"] = strconv.FormatBool(*c.PasswordSettings.Lowercase)
	props["PasswordRequireUppercase"] = strconv.FormatBool(*c.PasswordSettings.Uppercase)
	props["PasswordRequireNumber"] = strconv.FormatBool(*c.PasswordSettings.Number)
	props["PasswordRequireSymbol"] = strconv.FormatBool(*c.PasswordSettings.Symbol)
	props["PasswordEnableForgotLink"] = strconv.FormatBool(*c.PasswordSettings.EnableForgotLink)

	props["CWSURL"] = ""
	props["EnableCustomBrand"] = strconv.FormatBool(*c.TeamSettings.EnableCustomBrand)
	props["CustomBrandText"] = *c.TeamSettings.CustomBrandText
	props["CustomDescriptionText"] = *c.TeamSettings.CustomDescriptionText
	props["EnableMultifactorAuthentication"] = strconv.FormatBool(*c.ServiceSettings.EnableMultifactorAuthentication)
	props["EnforceMultifactorAuthentication"] = strconv.FormatBool(*c.ServiceSettings.EnforceMultifactorAuthentication)
	props["EnableGuestAccounts"] = strconv.FormatBool(*c.GuestAccountsSettings.Enable)
	props["HideGuestTags"] = strconv.FormatBool(*c.GuestAccountsSettings.HideTags)
	props["GuestAccountsEnforceMultifactorAuthentication"] = strconv.FormatBool(*c.GuestAccountsSettings.EnforceMultifactorAuthentication)
	props["EnableGuestMagicLink"] = strconv.FormatBool(*c.GuestAccountsSettings.EnableGuestMagicLink)

	props["EnableLdap"] = strconv.FormatBool(*c.LdapSettings.Enable)
	props["LdapLoginFieldName"] = *c.LdapSettings.LoginFieldName

	props["EnableSaml"] = strconv.FormatBool(*c.SamlSettings.Enable)
	props["SamlLoginButtonText"] = *c.SamlSettings.LoginButtonText

	props["EnableCustomTermsOfService"] = strconv.FormatBool(*c.SupportSettings.CustomTermsOfServiceEnabled)
	props["CustomTermsOfServiceReAcceptancePeriod"] = strconv.FormatInt(int64(*c.SupportSettings.CustomTermsOfServiceReAcceptancePeriod), 10)

	props["EnableSignUpWithGoogle"] = strconv.FormatBool(*c.GoogleSettings.Enable)
	props["EnableSignUpWithOffice365"] = strconv.FormatBool(*c.Office365Settings.Enable)

	props["EnableSignUpWithOpenId"] = strconv.FormatBool(*c.OpenIdSettings.Enable)
	props["OpenIdButtonColor"] = *c.OpenIdSettings.ButtonColor
	props["OpenIdButtonText"] = *c.OpenIdSettings.ButtonText
	props["EnableSignUpWithGitLab"] = strconv.FormatBool(*c.GitLabSettings.Enable)
	props["GitLabButtonColor"] = *c.GitLabSettings.ButtonColor
	props["GitLabButtonText"] = *c.GitLabSettings.ButtonText

	props["MobileEnableBiometrics"] = strconv.FormatBool(*c.NativeAppSettings.MobileEnableBiometrics)
	props["MobilePreventScreenCapture"] = strconv.FormatBool(*c.NativeAppSettings.MobilePreventScreenCapture)
	props["MobileJailbreakProtection"] = strconv.FormatBool(*c.NativeAppSettings.MobileJailbreakProtection)
	props["ExperimentalEnableWatermark"] = strconv.FormatBool(*c.ExperimentalSettings.EnableWatermark)

	// Check IntuneSettings configuration
	intuneEnabled := (c.IntuneSettings.Enable != nil && *c.IntuneSettings.Enable && c.IntuneSettings.IsValid() == nil)

	props["IntuneMAMEnabled"] = strconv.FormatBool(intuneEnabled)

	if intuneEnabled {
		// Use IntuneSettings for scope
		props["IntuneScope"] = fmt.Sprintf("api://%s/login.mattermost", *c.IntuneSettings.ClientId)

		// Expose AuthService if set
		if c.IntuneSettings.AuthService != nil && *c.IntuneSettings.AuthService != "" {
			props["IntuneAuthService"] = *c.IntuneSettings.AuthService
		}
	}

	for key, value := range c.FeatureFlags.ToMap() {
		props["FeatureFlag"+key] = value
	}

	return props
}

func getGiphySdkKey(ss model.ServiceSettings) string {
	if model.GetServiceEnvironment() == model.ServiceEnvironmentProduction {
		if *ss.GiphySdkKey != "" {
			return *ss.GiphySdkKey
		}

		return model.MattermostGiphySdkKey
	} else if model.GetServiceEnvironment() == model.ServiceEnvironmentDev || model.GetServiceEnvironment() == model.ServiceEnvironmentTest {
		return model.ServiceSettingsDefaultGiphySdkKeyTest
	}

	return ""
}
