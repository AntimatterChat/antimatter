// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package metrics

import (
	"slices"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

const (
	// legacyMetricPrefix is the prefix every server metric is registered with. It is kept
	// so that existing dashboards and alerting rules written for Mattermost keep working.
	legacyMetricPrefix = namespace + "_"

	// MetricPrefix is the prefix under which every legacy metric is also exposed.
	MetricPrefix = "antimatter_"
)

// WithAntimatterNames returns families plus, for every family whose name starts with
// "mattermost_", a copy named "antimatter_<rest>" carrying the same help, type, unit and
// samples. Collectors are only registered (and updated) once: the copies are created at
// scrape time and share their samples with the original family, so they must be treated
// as read-only. A copy is skipped when a family with that name already exists.
//
// The result is sorted by name, as expected from a prometheus.Gatherer.
func WithAntimatterNames(families []*dto.MetricFamily) []*dto.MetricFamily {
	names := make(map[string]struct{}, len(families))
	for _, f := range families {
		names[f.GetName()] = struct{}{}
	}

	out := make([]*dto.MetricFamily, len(families), 2*len(families))
	copy(out, families)
	for _, f := range families {
		rest, ok := strings.CutPrefix(f.GetName(), legacyMetricPrefix)
		if !ok {
			continue
		}
		name := MetricPrefix + rest
		if _, exists := names[name]; exists {
			continue
		}
		names[name] = struct{}{}
		out = append(out, &dto.MetricFamily{
			Name:   new(name),
			Help:   f.Help,
			Type:   f.Type,
			Unit:   f.Unit,
			Metric: f.Metric,
		})
	}

	if len(out) != len(families) {
		slices.SortFunc(out, func(a, b *dto.MetricFamily) int {
			return strings.Compare(a.GetName(), b.GetName())
		})
	}
	return out
}

// antimatterNamesGatherer exposes every "mattermost_" family of the wrapped gatherer under
// its "antimatter_" name as well. See WithAntimatterNames.
type antimatterNamesGatherer struct {
	prometheus.Gatherer
}

func (g antimatterNamesGatherer) Gather() ([]*dto.MetricFamily, error) {
	// On error the gatherer may still return the families it could collect; keep
	// them, the handler decides what to do with the error.
	families, err := g.Gatherer.Gather()
	return WithAntimatterNames(families), err
}
