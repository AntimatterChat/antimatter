// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBurnOnReadDurationMillis(t *testing.T) {
	const defaultSeconds = 600
	const maxSeconds = 4 * 60 * 60

	for name, tc := range map[string]struct {
		requested any
		max       int
		want      int64
		ok        bool
	}{
		"none asked: the default":                     {requested: nil, want: 600_000, ok: true},
		"a step of the picker, as JSON gives it":      {requested: float64(15 * 60 * 1000), want: 900_000, ok: true},
		"as an int":                                   {requested: 30 * 60 * 1000, want: 1_800_000, ok: true},
		"as an int64":                                 {requested: int64(60 * 60 * 1000), want: 3_600_000, ok: true},
		"as a json.Number":                            {requested: json.Number("60000"), want: 60_000, ok: true},
		"rounded to the second":                       {requested: float64(90_400), want: 90_000, ok: true},
		"the maximum itself":                          {requested: float64(maxSeconds * 1000), want: maxSeconds * 1000, ok: true},
		"above the maximum: clamped":                  {requested: float64(24 * 60 * 60 * 1000), want: maxSeconds * 1000, ok: true},
		"far above the maximum: clamped, no wrap":     {requested: 1e300, want: maxSeconds * 1000, ok: true},
		"the largest int64: clamped":                  {requested: int64(math.MaxInt64), want: maxSeconds * 1000, ok: true},
		"under a minute: a minute":                    {requested: float64(5_000), want: 60_000, ok: true},
		"a maximum under a minute wins":               {requested: float64(50_000), max: 30, want: 30_000, ok: true},
		"the default is kept under the maximum":       {requested: nil, max: 300, want: 300_000, ok: true},
		"no maximum configured: a day":                {requested: float64(2 * 24 * 60 * 60 * 1000), max: -1, want: BurnOnReadDefaultMaximumDurationSeconds * 1000, ok: true},
		"zero isn't a duration":                       {requested: float64(0), ok: false},
		"negative isn't a duration":                   {requested: float64(-60_000), ok: false},
		"text isn't a duration":                       {requested: "86400000", ok: false},
		"a list isn't a duration":                     {requested: []any{1}, ok: false},
		"an object isn't a duration":                  {requested: map[string]any{"ms": 1}, ok: false},
		"a boolean isn't a duration":                  {requested: true, ok: false},
		"NaN isn't a duration":                        {requested: math.NaN(), ok: false},
		"infinity isn't a duration":                   {requested: math.Inf(1), ok: false},
		"a json.Number that isn't a number":           {requested: json.Number("soon"), ok: false},
		"a negative json.Number isn't a duration":     {requested: json.Number("-1"), ok: false},
		"one millisecond: still at least a minute":    {requested: float64(1), want: 60_000, ok: true},
		"a fraction of a millisecond: still a minute": {requested: 0.5, want: 60_000, ok: true},
	} {
		t.Run(name, func(t *testing.T) {
			limit := maxSeconds
			if tc.max != 0 {
				limit = tc.max
			}
			got, ok := BurnOnReadDurationMillis(tc.requested, defaultSeconds, limit)
			assert.Equal(t, tc.ok, ok)
			if tc.ok {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestBurnOnReadMaximumDurationSetting(t *testing.T) {
	t.Run("defaults to a day, and keeps the default duration", func(t *testing.T) {
		s := ServiceSettings{}
		s.SetDefaults(false)
		assert.Equal(t, BurnOnReadDefaultMaximumDurationSeconds, *s.BurnOnReadMaximumDurationSeconds)
		assert.Equal(t, 600, *s.BurnOnReadDurationSeconds)
	})

	t.Run("an administrator's value is kept", func(t *testing.T) {
		s := ServiceSettings{BurnOnReadMaximumDurationSeconds: new(3600)}
		s.SetDefaults(true)
		assert.Equal(t, 3600, *s.BurnOnReadMaximumDurationSeconds)
	})
}
