// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package metrics

import (
	"github.com/mattermost/mattermost/server/public/model"
)

// Client side metrics are reported by the web, desktop and mobile apps through performance
// reports. The platform and agent labels are already normalized by
// model.PerformanceReport.ProcessLabels; the other labels coming from clients are either
// validated against the accepted values or bounded by a cardinality budget.
//
// The userID parameters are deliberately not used as labels: doing so would make the
// cardinality of the metrics grow with the number of users.

func (m *MetricsInterfaceImpl) ObserveClientTimeToFirstByte(platform, agent, _ string, elapsed float64) {
	m.ClientTimeToFirstByte.WithLabelValues(platform, agent).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObserveClientTimeToLastByte(platform, agent, _ string, elapsed float64) {
	m.ClientTimeToLastByte.WithLabelValues(platform, agent).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObserveClientTimeToDomInteractive(platform, agent, _ string, elapsed float64) {
	m.ClientTimeToDomInteractive.WithLabelValues(platform, agent).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObserveClientSplashScreenEnd(platform, agent, pageType, _ string, elapsed float64) {
	m.ClientSplashScreenEnd.WithLabelValues(platform, agent, allowedLabel(pageType, model.AcceptedSplashScreenOrigins)).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObserveClientFirstContentfulPaint(platform, agent, _ string, elapsed float64) {
	m.ClientFirstContentfulPaint.WithLabelValues(platform, agent).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObserveClientLargestContentfulPaint(platform, agent, region, _ string, elapsed float64) {
	m.ClientLargestContentfulPaint.WithLabelValues(platform, agent, allowedLabel(region, model.AcceptedLCPRegions)).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObserveClientInteractionToNextPaint(platform, agent, interaction, _ string, elapsed float64) {
	m.ClientInteractionToNextPaint.WithLabelValues(platform, agent, allowedLabel(interaction, model.AcceptedInteractions)).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObserveClientCumulativeLayoutShift(platform, agent, _ string, elapsed float64) {
	m.ClientCumulativeLayoutShift.WithLabelValues(platform, agent).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) IncrementClientLongTasks(platform, agent, _ string, inc float64) {
	if inc <= 0 {
		return
	}
	m.ClientLongTasks.WithLabelValues(platform, agent).Add(inc)
}

func (m *MetricsInterfaceImpl) ObserveClientPageLoadDuration(platform, agent, _ string, elapsed float64) {
	m.ClientPageLoadDuration.WithLabelValues(platform, agent).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObserveClientChannelSwitchDuration(platform, agent, fresh, _ string, elapsed float64) {
	m.ClientChannelSwitchDuration.WithLabelValues(platform, agent, freshLabel(fresh)).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObserveClientTeamSwitchDuration(platform, agent, fresh, _ string, elapsed float64) {
	m.ClientTeamSwitchDuration.WithLabelValues(platform, agent, freshLabel(fresh)).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObserveClientRHSLoadDuration(platform, agent, _ string, elapsed float64) {
	m.ClientRHSLoadDuration.WithLabelValues(platform, agent).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObserveGlobalThreadsLoadDuration(platform, agent, _ string, elapsed float64) {
	m.ClientGlobalThreadsLoad.WithLabelValues(platform, agent).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObservePluginWebappPerf(platform, agent, pluginID, pluginMetricLabel string, elapsed float64) {
	m.ClientPluginWebappPerf.WithLabelValues(
		platform,
		agent,
		m.pluginIDLabel.value(pluginID),
		m.pluginMetricLabel.value(pluginMetricLabel),
	).Observe(elapsed)
}

// Mobile

func (m *MetricsInterfaceImpl) ObserveMobileClientLoadDuration(platform string, elapsed float64) {
	m.MobileClientLoadDuration.WithLabelValues(platform).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObserveMobileClientChannelSwitchDuration(platform string, elapsed float64) {
	m.MobileClientChannelSwitchDuration.WithLabelValues(platform).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObserveMobileClientTeamSwitchDuration(platform string, elapsed float64) {
	m.MobileClientTeamSwitchDuration.WithLabelValues(platform).Observe(elapsed)
}

func networkRequestGroupLabel(group string) string {
	return allowedLabel(group, model.AcceptedNetworkRequestGroups)
}

func (m *MetricsInterfaceImpl) ObserveMobileClientNetworkRequestsAverageSpeed(platform, agent, networkRequestGroup string, speed float64) {
	m.MobileClientNetworkAverageSpeed.WithLabelValues(platform, agent, networkRequestGroupLabel(networkRequestGroup)).Observe(speed)
}

func (m *MetricsInterfaceImpl) ObserveMobileClientNetworkRequestsEffectiveLatency(platform, agent, networkRequestGroup string, latency float64) {
	m.MobileClientNetworkEffectiveLatency.WithLabelValues(platform, agent, networkRequestGroupLabel(networkRequestGroup)).Observe(latency)
}

func (m *MetricsInterfaceImpl) ObserveMobileClientNetworkRequestsElapsedTime(platform, agent, networkRequestGroup string, elapsedTime float64) {
	m.MobileClientNetworkElapsedTime.WithLabelValues(platform, agent, networkRequestGroupLabel(networkRequestGroup)).Observe(elapsedTime)
}

func (m *MetricsInterfaceImpl) ObserveMobileClientNetworkRequestsLatency(platform, agent, networkRequestGroup string, latency float64) {
	m.MobileClientNetworkLatency.WithLabelValues(platform, agent, networkRequestGroupLabel(networkRequestGroup)).Observe(latency)
}

func (m *MetricsInterfaceImpl) ObserveMobileClientNetworkRequestsTotalCompressedSize(platform, agent, networkRequestGroup string, size float64) {
	m.MobileClientNetworkTotalCompressedSize.WithLabelValues(platform, agent, networkRequestGroupLabel(networkRequestGroup)).Observe(size)
}

func (m *MetricsInterfaceImpl) ObserveMobileClientNetworkRequestsTotalParallelRequests(platform, agent, networkRequestGroup string, count float64) {
	m.MobileClientNetworkTotalParallelReqs.WithLabelValues(platform, agent, networkRequestGroupLabel(networkRequestGroup)).Observe(count)
}

func (m *MetricsInterfaceImpl) ObserveMobileClientNetworkRequestsTotalRequests(platform, agent, networkRequestGroup string, count float64) {
	m.MobileClientNetworkTotalRequests.WithLabelValues(platform, agent, networkRequestGroupLabel(networkRequestGroup)).Observe(count)
}

func (m *MetricsInterfaceImpl) ObserveMobileClientNetworkRequestsTotalSequentialRequests(platform, agent, networkRequestGroup string, count float64) {
	m.MobileClientNetworkTotalSequentialReqs.WithLabelValues(platform, agent, networkRequestGroupLabel(networkRequestGroup)).Observe(count)
}

func (m *MetricsInterfaceImpl) ObserveMobileClientNetworkRequestsTotalSize(platform, agent, networkRequestGroup string, size float64) {
	m.MobileClientNetworkTotalSize.WithLabelValues(platform, agent, networkRequestGroupLabel(networkRequestGroup)).Observe(size)
}

// ClearMobileClientSessionMetadata removes every series of the mobile session metadata gauge.
// The mobile session metadata job calls it before publishing a fresh snapshot.
func (m *MetricsInterfaceImpl) ClearMobileClientSessionMetadata() {
	m.mobileClientSessionMetadataMut.Lock()
	defer m.mobileClientSessionMetadataMut.Unlock()

	m.MobileClientSessionMetadata.Reset()
	m.mobileVersionLabel.reset()
	m.mobilePlatformLabel.reset()
}

func (m *MetricsInterfaceImpl) ObserveMobileClientSessionMetadata(version string, platform string, value float64, notificationDisabled string) {
	m.mobileClientSessionMetadataMut.Lock()
	defer m.mobileClientSessionMetadataMut.Unlock()

	m.MobileClientSessionMetadata.WithLabelValues(
		m.mobileVersionLabel.value(version),
		m.mobilePlatformLabel.value(platform),
		sanitizeLabelValue(notificationDisabled),
	).Add(value)
}

// Desktop

func (m *MetricsInterfaceImpl) ObserveDesktopCpuUsage(platform, version, process string, usage float64) {
	m.DesktopClientCPUUsage.WithLabelValues(platform, m.desktopVersionLabel.value(version), m.desktopProcessLabel.value(process)).Observe(usage)
}

func (m *MetricsInterfaceImpl) ObserveDesktopMemoryUsage(platform, version, process string, usage float64) {
	m.DesktopClientMemoryUsage.WithLabelValues(platform, m.desktopVersionLabel.value(version), m.desktopProcessLabel.value(process)).Observe(usage)
}

func freshLabel(fresh string) string {
	if fresh == "" {
		return ""
	}
	return allowedLabel(fresh, model.AcceptedTrueFalseLabels)
}
