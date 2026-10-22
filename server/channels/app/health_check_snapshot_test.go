// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest/mocks"
	emocks "github.com/mattermost/mattermost/server/v8/einterfaces/mocks"
	"github.com/mattermost/mattermost/server/v8/platform/shared/healthcheck"
	"github.com/mattermost/mattermost/server/v8/platform/shared/healthcheck/packet"
)

func TestBuildHealthSnapshotStandaloneLeaderDiagnostics(t *testing.T) {
	th := Setup(t)

	snapshot, buildErr := th.App.BuildHealthSnapshot(th.Context)
	require.NoError(t, buildErr)
	require.NotNil(t, snapshot)

	nodes := snapshot.Nodes()
	require.Len(t, nodes, 1)
	require.True(t, nodes[0].IsLeader)
	require.Nil(t, nodes[0].ClusterInfo)
	require.NotNil(t, nodes[0].Diagnostics)
	require.NotNil(t, nodes[0].Diagnostics.Diagnostics)
	for _, section := range model.AllNodeSections() {
		assert.True(t, nodes[0].Has(section), "section %q", section)
	}

	hostname, err := os.Hostname()
	require.NoError(t, err)
	assert.Equal(t, hostname, nodes[0].Hostname)
}

// threeNodeCluster reports this node as id-2 of three.
type threeNodeCluster struct {
	captureClusterMock
}

func (c *threeNodeCluster) GetClusterId() string { return "id-2" }

func (c *threeNodeCluster) GetClusterInfos() ([]*model.ClusterInfo, error) {
	return []*model.ClusterInfo{
		{Id: "id-1", Hostname: "node-1"},
		{Id: "id-2", Hostname: "node-2"},
		{Id: "id-3", Hostname: "node-3"},
	}, nil
}

func TestBuildHealthSnapshotOnlyLeaderHasDiagnostics(t *testing.T) {
	th := SetupWithClusterMock(t, &threeNodeCluster{})
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ClusterSettings.Enable = true
	})

	snapshot, buildErr := th.App.BuildHealthSnapshot(th.Context)
	require.NoError(t, buildErr)
	require.NotNil(t, snapshot)

	nodes := snapshot.Nodes()
	require.Len(t, nodes, 3)
	for _, node := range nodes {
		if node.IsLeader {
			assert.Equal(t, "id-2", node.ClusterInfo.Id)
			require.NotNil(t, node.Diagnostics)
			require.NotNil(t, node.Diagnostics.Diagnostics.Cluster.NumberOfNodes)
			assert.Equal(t, 3, *node.Diagnostics.Diagnostics.Cluster.NumberOfNodes)
		} else {
			assert.Nil(t, node.Diagnostics, "follower %q", node.ClusterInfo.Id)
		}
	}
	leader, ok := snapshot.Leader()
	require.True(t, ok)
	assert.Equal(t, "id-2", leader.ClusterInfo.Id)
}

func TestBuildHealthSnapshotSectionFailureIsolationWithPartialStats(t *testing.T) {
	th := Setup(t)

	storeMock := mocks.Store{}
	channelStore := &mocks.ChannelStore{}
	channelStore.On("AnalyticsTypeCount", "", model.ChannelTypeOpen).Return(int64(2), nil)
	channelStore.On("AnalyticsTypeCount", "", model.ChannelTypePrivate).Return(int64(0), errors.New("private query failed"))

	originalStore := th.App.Srv().Store()
	t.Cleanup(func() {
		th.App.Srv().SetStore(originalStore)
	})

	storeMock.On("User").Return(originalStore.User())
	storeMock.On("Post").Return(originalStore.Post())
	storeMock.On("Channel").Return(channelStore)
	storeMock.On("Team").Return(originalStore.Team())
	storeMock.On("Command").Return(originalStore.Command())
	storeMock.On("Webhook").Return(originalStore.Webhook())
	storeMock.On("Job").Return(originalStore.Job())
	storeMock.On("GetDBSchemaVersion").Return(originalStore.GetDBSchemaVersion())
	storeMock.On("GetDbVersion", false).Return(originalStore.GetDbVersion(false))
	storeMock.On("TotalMasterDbConnections").Return(originalStore.TotalMasterDbConnections())
	storeMock.On("TotalReadDbConnections").Return(originalStore.TotalReadDbConnections())
	storeMock.On("TotalSearchDbConnections").Return(originalStore.TotalSearchDbConnections())
	storeMock.On("GetDiagnostics", mock.Anything).Return(originalStore.GetDiagnostics(th.Context))
	storeMock.On("ClearCaches")
	storeMock.On("Close").Return(nil)

	th.App.Srv().SetStore(&storeMock)

	snapshot, buildErr := th.App.BuildHealthSnapshot(th.Context)
	require.NoError(t, buildErr)
	require.NotNil(t, snapshot)

	ok, sectionErr := snapshot.SectionErr(model.SectionStats)
	require.True(t, ok)
	require.ErrorContains(t, sectionErr, "failed to get channel count")

	require.NotNil(t, snapshot.Stats)
	assert.Nil(t, snapshot.Stats.Channels)
	assert.NotNil(t, snapshot.Stats.Posts)

	require.NotNil(t, snapshot.Config)
	require.NotNil(t, snapshot.Jobs)
	require.NotNil(t, snapshot.Plugins)
	ok, sectionErr = snapshot.SectionErr(model.SectionConfig)
	require.True(t, ok)
	require.NoError(t, sectionErr)
	ok, sectionErr = snapshot.SectionErr(model.SectionJobs)
	require.True(t, ok)
	require.NoError(t, sectionErr)
	ok, sectionErr = snapshot.SectionErr(model.SectionPlugins)
	require.True(t, ok)
	require.NoError(t, sectionErr)
}

// TestBuildHealthSnapshotFetchesNoLatestRelease: the snapshot carries the running version only and
// records exactly the workspace sections it collects; there is no latest-release lookup.
func TestBuildHealthSnapshotFetchesNoLatestRelease(t *testing.T) {
	th := Setup(t)

	snapshot, err := th.App.BuildHealthSnapshot(th.Context)
	require.NoError(t, err)

	assert.Equal(t, model.CurrentVersion, snapshot.Version.Current)
	sections := make([]model.WorkspaceSection, 0, len(snapshot.Sections))
	for section := range snapshot.Sections {
		sections = append(sections, section)
	}
	assert.ElementsMatch(t, model.AllWorkspaceSections(), sections)
}

func TestHealthSnapshotLiveOfflineParity(t *testing.T) {
	th := Setup(t)
	th.App.UpdateConfig(func(cfg *model.Config) {
		cfg.ServiceSettings.SiteURL = model.NewPointer("http://chat.example.com")
		cfg.EmailSettings.SendPushNotifications = model.NewPointer(true)
		cfg.EmailSettings.PushNotificationServer = model.NewPointer("http://push.example.com")
		// The live and packet collections each probe SMTP and could disagree.
		cfg.EmailSettings.SendEmailNotifications = model.NewPointer(false)
		cfg.EmailSettings.EnableSignInWithEmail = model.NewPointer(false)
		cfg.EmailSettings.EnableSignInWithUsername = model.NewPointer(false)
		cfg.SqlSettings.QueryTimeout = model.NewPointer(60)
		cfg.ComplianceSettings.Enable = model.NewPointer(true)
		cfg.ComplianceSettings.Directory = model.NewPointer("")
		cfg.MetricsSettings.Enable = model.NewPointer(true)
	})

	// Without it the permissions collector fails and the packet carries a warning.txt.
	err := th.App.Srv().Store().System().Save(&model.System{Name: model.MigrationKeyAdvancedPermissionsPhase2, Value: "true"})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, deleteErr := th.App.Srv().Store().System().PermanentDeleteByName(model.MigrationKeyAdvancedPermissionsPhase2)
		require.NoError(t, deleteErr)
	})

	// Half an hour off the hour boundary, so the reported idle hours match although the packet is generated later.
	stale := time.Now().Add(-30*time.Hour - 30*time.Minute).UnixMilli()
	for _, job := range []*model.Job{
		{Id: model.NewId(), Type: model.JobTypeDataRetention, CreateAt: stale - 1000, LastActivityAt: stale - 1000, Status: model.JobStatusError, Data: model.StringMap{"error": "driver: bad connection"}},
		{Id: model.NewId(), Type: model.JobTypeDataRetention, CreateAt: stale, StartAt: stale, LastActivityAt: stale, Status: model.JobStatusInProgress},
	} {
		_, err = th.App.Srv().Store().Job().Save(job)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, deleteErr := th.App.Srv().Store().Job().Delete(job.Id)
			require.NoError(t, deleteErr)
		})
	}

	live, err := th.App.BuildHealthSnapshot(th.Context)
	require.NoError(t, err)

	var zipped bytes.Buffer
	var noCPUProfile time.Duration
	files := th.App.GenerateSupportPacket(th.Context, &model.SupportPacketOptions{CPUProfileDuration: &noCPUProfile})
	require.NoError(t, th.App.WriteZipFile(&zipped, files))
	offline, err := packet.Read(bytes.NewReader(zipped.Bytes()), int64(zipped.Len()))
	require.NoError(t, err)
	assert.Empty(t, offline.Warnings)

	evaluatedAt := time.Now()
	engine := healthcheck.NewEngine(healthcheck.EngineOpts{
		Registry: healthcheck.Builtin(),
		Now:      func() time.Time { return evaluatedAt },
	})
	liveEvaluations := engine.Evaluate(live)
	offlineEvaluations := engine.Evaluate(offline.Snapshot)

	// Each collection samples the open file descriptor count separately.
	for _, evaluations := range [][]healthcheck.Evaluation{liveEvaluations, offlineEvaluations} {
		for i := range evaluations {
			if evaluations[i].Code == "NODE_FD_EXHAUSTION" {
				require.NotNil(t, evaluations[i].Result.Value)
				evaluations[i].Result.Value = nil
			}
		}
	}

	require.NotEmpty(t, liveEvaluations)
	assert.Equal(t, liveEvaluations, offlineEvaluations)

	firing := map[string]bool{}
	for _, evaluation := range liveEvaluations {
		if evaluation.Result.State == healthcheck.StateFiring {
			firing[evaluation.Code] = true
		}
	}
	assert.Equal(t, map[string]bool{
		"COMPLIANCE_NO_DIR":     true,
		"DB_QUERY_TIMEOUT_HIGH": true,
		"JOB_FAILED":            true,
		"JOB_STUCK":             true,
		"PUSH_BAD_SCHEME":       true,
		"PUSH_ID_ONLY":          true,
		"SITE_URL_HTTP":         true,
	}, firing)
}

func TestClusterNodes(t *testing.T) {
	t.Parallel()

	t.Run("nil cluster creates standalone leader", func(t *testing.T) {
		nodes, err := clusterNodes(nil)
		require.NoError(t, err)
		require.Len(t, nodes, 1)
		require.True(t, nodes[0].IsLeader)
		require.Nil(t, nodes[0].ClusterInfo)
		require.Nil(t, nodes[0].Diagnostics)
	})

	t.Run("empty cluster infos creates standalone leader", func(t *testing.T) {
		cluster := emocks.NewClusterInterface(t)
		cluster.On("GetClusterInfos").Return([]*model.ClusterInfo{}, nil)

		nodes, err := clusterNodes(cluster)
		require.NoError(t, err)
		require.Len(t, nodes, 1)
		require.True(t, nodes[0].IsLeader)
		require.Nil(t, nodes[0].ClusterInfo)
		require.Nil(t, nodes[0].Diagnostics)
	})

	t.Run("leader selection and db-only entry", func(t *testing.T) {
		cluster := emocks.NewClusterInterface(t)
		cluster.On("GetClusterId").Return("id-2")
		cluster.On("GetClusterInfos").Return([]*model.ClusterInfo{
			{Id: "id-1", Hostname: "node-1", Version: "10.0.0"},
			{Id: "id-2", Hostname: "node-2", Version: "10.0.0"},
			{Id: "id-3", Hostname: "node-3"},
		}, nil)

		nodes, err := clusterNodes(cluster)
		require.NoError(t, err)
		require.Len(t, nodes, 3)
		require.Nil(t, nodes[0].Diagnostics)
		require.Nil(t, nodes[1].Diagnostics)
		require.Nil(t, nodes[2].Diagnostics)

		leaders := 0
		var dbOnlyNode *model.ClusterInfo
		for _, node := range nodes {
			if node.IsLeader {
				leaders++
				assert.Equal(t, "id-2", node.ClusterInfo.Id)
			}
			if node.ClusterInfo != nil && node.ClusterInfo.Id == "id-3" {
				dbOnlyNode = node.ClusterInfo
				_, ok := node.NodeVersion()
				require.False(t, ok)
			}
		}
		require.Equal(t, 1, leaders)
		require.NotNil(t, dbOnlyNode)
	})

	t.Run("cluster info failure returns error", func(t *testing.T) {
		cluster := emocks.NewClusterInterface(t)
		cluster.On("GetClusterInfos").Return(nil, assert.AnError)

		nodes, err := clusterNodes(cluster)
		require.Error(t, err)
		require.Nil(t, nodes)
	})
}
