// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package platform

import (
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/markdown"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest"
	"github.com/mattermost/mattermost/server/v8/config"
	"github.com/mattermost/mattermost/server/v8/einterfaces/mocks"
)

func TestReadReplicas(t *testing.T) {
	mainHelper.Parallel(t)
	cfg := model.Config{}
	cfg.SetDefaults()
	driverName := os.Getenv("MM_SQLSETTINGS_DRIVERNAME")
	if driverName == "" {
		driverName = model.DatabaseDriverPostgres
	}
	cfg.SqlSettings = *storetest.MakeSqlSettings(driverName)
	cfg.SqlSettings.DataSourceReplicas = []string{*cfg.SqlSettings.DataSource}
	cfg.SqlSettings.DataSourceSearchReplicas = []string{*cfg.SqlSettings.DataSource}

	t.Run("Read Replicas", func(t *testing.T) {
		configStore := config.NewTestMemoryStore()
		_, _, err := configStore.Set(&cfg)
		require.NoError(t, err)
		ps, err := New(
			ServiceConfig{},
			ConfigStore(configStore),
		)
		require.NoError(t, err)
		require.NotSame(t, ps.sqlStore.GetMaster(), ps.sqlStore.GetReplica())
		require.Len(t, ps.Config().SqlSettings.DataSourceReplicas, 1)
	})

	t.Run("Search Replicas", func(t *testing.T) {
		configStore := config.NewTestMemoryStore()
		_, _, err := configStore.Set(&cfg)
		require.NoError(t, err)
		ps, err := New(
			ServiceConfig{},
			ConfigStore(configStore),
		)
		require.NoError(t, err)
		require.NotSame(t, ps.sqlStore.GetMaster(), ps.sqlStore.GetSearchReplicaX())
		require.Len(t, ps.Config().SqlSettings.DataSourceSearchReplicas, 1)
	})
}

func TestMetrics(t *testing.T) {
	mainHelper.Parallel(t)
	t.Run("ensure the metrics server is not started by default", func(t *testing.T) {
		mainHelper.Parallel(t)
		th := Setup(t)

		require.Nil(t, th.Service.metrics)
	})

	t.Run("ensure the metrics server is started", func(t *testing.T) {
		mainHelper.Parallel(t)
		th := Setup(t, StartMetrics())

		// there is no config listener for the metrics
		// we handle it on config save step
		cfg := th.Service.Config().Clone()
		cfg.MetricsSettings.Enable = new(true)
		_, _, appErr := th.Service.SaveConfig(cfg, false)
		require.Nil(t, appErr)

		require.NotNil(t, th.Service.metrics)
		metricsAddr := strings.Replace(th.Service.metrics.listenAddr, "[::]", "http://localhost", 1)
		metricsAddr = strings.Replace(metricsAddr, "127.0.0.1", "http://localhost", 1)

		resp, err := http.Get(metricsAddr)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		cfg.MetricsSettings.Enable = new(false)
		_, _, appErr = th.Service.SaveConfig(cfg, false)
		require.Nil(t, appErr)

		_, err = http.Get(metricsAddr)
		require.Error(t, err)
	})

	t.Run("ensure the metrics server is started with advanced metrics", func(t *testing.T) {
		mainHelper.Parallel(t)
		th := Setup(t, StartMetrics())

		mockMetricsImpl := &mocks.MetricsInterface{}
		mockMetricsImpl.On("Register").Return()

		th.Service.metricsIFace = mockMetricsImpl
		err := th.Service.resetMetrics()
		require.NoError(t, err)

		mockMetricsImpl.AssertExpectations(t)
	})

	t.Run("ensure advanced metrics have database metrics", func(t *testing.T) {
		mainHelper.Parallel(t)
		mockMetricsImpl := &mocks.MetricsInterface{}
		mockMetricsImpl.On("Register").Return()
		mockMetricsImpl.On("ObserveStoreMethodDuration", mock.Anything, mock.Anything, mock.Anything).Return()
		mockMetricsImpl.On("RegisterDBCollector", mock.AnythingOfType("*sql.DB"), "master")

		th := Setup(t, StartMetrics(), func(ps *PlatformService) error {
			ps.metricsIFace = mockMetricsImpl
			return nil
		})

		_ = th.CreateUserOrGuest(t, false)

		th.Shutdown(t)
		mockMetricsImpl.AssertExpectations(t)
	})
}

func TestShutdown(t *testing.T) {
	mainHelper.Parallel(t)
	t.Run("should shutdown gracefully", func(t *testing.T) {
		th := Setup(t)

		// we create plenty of go routines to make sure we wait for all of them
		// to finish before shutting down
		for range 1000 {
			th.Service.Go(func() {
				time.Sleep(time.Millisecond * time.Duration(rand.IntN(20)))
			})
		}

		th.Shutdown(t)

		// assert that there are no more go routines running
		require.Zero(t, atomic.LoadInt32(&th.Service.goroutineCount))
	})
}

func TestSetTelemetryId(t *testing.T) {
	mainHelper.Parallel(t)
	t.Run("ensure client config is regenerated after setting the telemetry id", func(t *testing.T) {
		th := Setup(t)

		clientConfig := th.Service.LimitedClientConfig()
		require.Empty(t, clientConfig["DiagnosticId"])

		id := model.NewId()
		th.Service.SetTelemetryId(id)

		clientConfig = th.Service.LimitedClientConfig()
		require.Equal(t, clientConfig["DiagnosticId"], id)
	})
}

func TestDatabaseTypeAndMattermostVersion(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t)

	databaseType, schemaVersion, err := th.Service.DatabaseTypeAndSchemaVersion()
	require.NoError(t, err)
	assert.Equal(t, "postgres", databaseType)

	// It's hard to check whether the schema version is correct or not.
	// So, we just check if it's greater than 1.
	assert.GreaterOrEqual(t, schemaVersion, strconv.Itoa(1))
}

func TestNewSyncsMarkdownMaxLenWithMaxPostSize(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t)

	// markdown.SetMaxPostRunes is package-global; parallel tests (including
	// markdown's own tests) can overwrite it between Setup and this check.
	require.Eventually(t, func() bool {
		maxPostSize := th.Service.MaxPostSize()
		return markdown.MaxLen() == 4*maxPostSize
	}, 5*time.Second, 10*time.Millisecond)
}
