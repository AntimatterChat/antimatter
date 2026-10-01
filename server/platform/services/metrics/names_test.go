// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package metrics

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func familyNames(families []*dto.MetricFamily) []string {
	names := make([]string, 0, len(families))
	for _, f := range families {
		names = append(names, f.GetName())
	}
	return names
}

func TestWithAntimatterNames(t *testing.T) {
	t.Run("copies mattermost families only, sorted", func(t *testing.T) {
		in := []*dto.MetricFamily{
			{Name: new("go_goroutines"), Type: dto.MetricType_GAUGE.Enum()},
			{Name: new("mattermost_post_total"), Help: new("Posts"), Type: dto.MetricType_COUNTER.Enum(), Metric: []*dto.Metric{
				{Counter: &dto.Counter{Value: new(3.0)}},
			}},
			{Name: new("mattermostish_total")},
		}
		out := WithAntimatterNames(in)

		assert.Equal(t, []string{"antimatter_post_total", "go_goroutines", "mattermost_post_total", "mattermostish_total"}, familyNames(out))
		alias := out[0]
		assert.Equal(t, "Posts", alias.GetHelp())
		assert.Equal(t, dto.MetricType_COUNTER, alias.GetType())
		require.Len(t, alias.GetMetric(), 1)
		assert.Equal(t, 3.0, alias.GetMetric()[0].GetCounter().GetValue())

		// The input is left untouched.
		assert.Equal(t, []string{"go_goroutines", "mattermost_post_total", "mattermostish_total"}, familyNames(in))
	})

	t.Run("an existing antimatter family wins over the copy", func(t *testing.T) {
		in := []*dto.MetricFamily{
			{Name: new("antimatter_x"), Help: new("native")},
			{Name: new("mattermost_x"), Help: new("legacy")},
		}
		out := WithAntimatterNames(in)
		require.Len(t, out, 2)
		assert.Equal(t, "native", out[0].GetHelp())
	})

	t.Run("no families", func(t *testing.T) {
		assert.Empty(t, WithAntimatterNames(nil))
	})
}

type failingGatherer struct {
	families []*dto.MetricFamily
}

func (g failingGatherer) Gather() ([]*dto.MetricFamily, error) {
	return g.families, errors.New("collector failed")
}

func TestAntimatterNamesGatherer(t *testing.T) {
	t.Run("registry", func(t *testing.T) {
		reg := prometheus.NewRegistry()
		vec := prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: namespace, Subsystem: "api", Name: "calls_total", Help: "Calls."}, []string{"handler"})
		reg.MustRegister(vec)
		vec.WithLabelValues("a").Add(2)
		vec.WithLabelValues("b").Inc()

		families, err := antimatterNamesGatherer{Gatherer: reg}.Gather()
		require.NoError(t, err)
		require.Equal(t, []string{"antimatter_api_calls_total", "mattermost_api_calls_total"}, familyNames(families))
		assert.Equal(t, families[1].GetMetric(), families[0].GetMetric())
		assert.Equal(t, "Calls.", families[0].GetHelp())
	})

	t.Run("keeps partial results on error", func(t *testing.T) {
		families, err := antimatterNamesGatherer{Gatherer: failingGatherer{families: []*dto.MetricFamily{
			{Name: new("mattermost_a")},
		}}}.Gather()
		require.Error(t, err)
		assert.Equal(t, []string{"antimatter_a", "mattermost_a"}, familyNames(families))
	})
}

func TestHandlerServesBothPrefixes(t *testing.T) {
	m := newTestMetrics(t, Options{})
	m.IncrementPostCreate()
	m.IncrementPostCreate()
	m.ObserveAPIEndpointDuration("getPosts", "GET", "200", "web", "", 0.1)

	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	for _, prefix := range []string{"mattermost_", "antimatter_"} {
		assert.Contains(t, body, "# HELP "+prefix+"post_total The total number of posts created.")
		assert.Contains(t, body, "# TYPE "+prefix+"post_total counter")
		assert.Contains(t, body, prefix+"post_total 2")
		assert.Contains(t, body, prefix+`api_time_count{handler="getPosts"`)
	}
	assert.Contains(t, body, "go_goroutines")
	assert.NotContains(t, body, "antimatter_go_goroutines")

	// Only the legacy names are registered: the copies don't add collectors.
	for name := range gatherFamilies(t, m) {
		assert.NotContains(t, name, MetricPrefix)
	}
}
