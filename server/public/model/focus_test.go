// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFocusRequestResolveEnd(t *testing.T) {
	paris, err := time.LoadLocation("Europe/Paris")
	require.NoError(t, err)
	// 14:10 in Paris (CEST, UTC+2).
	now := time.Date(2026, 10, 21, 12, 10, 0, 0, time.UTC)

	t.Run("the durations of the dialog", func(t *testing.T) {
		for duration, want := range map[string]time.Duration{
			"15m": 15 * time.Minute, "30m": 30 * time.Minute, "1h": time.Hour, "2h": 2 * time.Hour, "3h": 3 * time.Hour,
			"4h": 4 * time.Hour, "5h": 5 * time.Hour, "6h": 6 * time.Hour, "1h30m": 90 * time.Minute,
		} {
			r := &FocusRequest{Duration: duration}
			end, appErr := r.ResolveEnd(now, paris)
			require.Nil(t, appErr, duration)
			assert.Equal(t, now.Add(want), end, duration)
		}
	})

	t.Run("until tomorrow is 09:00 the next day in the user's timezone", func(t *testing.T) {
		r := &FocusRequest{Duration: FocusDurationTomorrow}
		end, appErr := r.ResolveEnd(now, paris)
		require.Nil(t, appErr)
		assert.Equal(t, time.Date(2026, 10, 22, 9, 0, 0, 0, paris), end)
		assert.Equal(t, time.Date(2026, 10, 22, 7, 0, 0, 0, time.UTC), end.UTC())

		// Just after midnight, tomorrow is still the next calendar day.
		late := time.Date(2026, 10, 21, 22, 30, 0, 0, time.UTC) // 00:30 on the 22nd in Paris
		end, appErr = r.ResolveEnd(late, paris)
		require.Nil(t, appErr)
		assert.Equal(t, time.Date(2026, 10, 23, 9, 0, 0, 0, paris), end)

		// Across the change to winter time (25 October 2026 in Paris), still 09:00 local.
		saturday := time.Date(2026, 10, 24, 18, 0, 0, 0, time.UTC)
		end, appErr = r.ResolveEnd(saturday, paris)
		require.Nil(t, appErr)
		assert.Equal(t, 9, end.In(paris).Hour())
		assert.Equal(t, 25, end.In(paris).Day())

		// Without a timezone, UTC.
		end, appErr = r.ResolveEnd(now, nil)
		require.Nil(t, appErr)
		assert.Equal(t, time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC), end)
	})

	t.Run("a custom end", func(t *testing.T) {
		at := now.Add(3*time.Hour + 20*time.Minute)
		r := &FocusRequest{EndAt: at.UnixMilli()}
		end, appErr := r.ResolveEnd(now, paris)
		require.Nil(t, appErr)
		assert.Equal(t, at.UnixMilli(), end.UnixMilli())
	})

	t.Run("bounds", func(t *testing.T) {
		for name, r := range map[string]*FocusRequest{
			"in the past":          {EndAt: now.Add(-time.Hour).UnixMilli()},
			"under a minute":       {EndAt: now.Add(30 * time.Second).UnixMilli()},
			"over a week":          {EndAt: now.Add(8 * 24 * time.Hour).UnixMilli()},
			"a duration too long":  {Duration: "200h"},
			"a negative duration":  {Duration: "-1h"},
			"a duration too short": {Duration: "10s"},
			"not a duration":       {Duration: "soon"},
			"neither":              {},
			"both":                 {Duration: "1h", EndAt: now.Add(time.Hour).UnixMilli()},
		} {
			_, appErr := r.ResolveEnd(now, paris)
			assert.NotNil(t, appErr, name)
		}

		_, appErr := (&FocusRequest{Duration: "1m"}).ResolveEnd(now, paris)
		assert.Nil(t, appErr, "a minute is the shortest")
		_, appErr = (&FocusRequest{EndAt: now.Add(FocusMaxDuration).UnixMilli()}).ResolveEnd(now, paris)
		assert.Nil(t, appErr, "a week is the longest")
	})

	t.Run("the reason fits a custom status", func(t *testing.T) {
		r := &FocusRequest{Duration: "1h", Reason: "  Working on the new release  "}
		_, appErr := r.ResolveEnd(now, paris)
		require.Nil(t, appErr)
		assert.Equal(t, "Working on the new release", r.Reason, "trimmed")

		r = &FocusRequest{Duration: "1h", Reason: strings.Repeat("é", CustomStatusTextMaxRunes)}
		_, appErr = r.ResolveEnd(now, paris)
		assert.Nil(t, appErr, "the limit is in characters, not bytes")

		r = &FocusRequest{Duration: "1h", Reason: strings.Repeat("a", CustomStatusTextMaxRunes+1)}
		_, appErr = r.ResolveEnd(now, paris)
		assert.NotNil(t, appErr)

		r = &FocusRequest{Duration: "1h"}
		_, appErr = r.ResolveEnd(now, paris)
		assert.Nil(t, appErr, "the reason is optional")
	})
}

func TestUserFocusState(t *testing.T) {
	var none *UserFocus
	assert.False(t, none.IsActive())
	assert.Equal(t, &FocusState{}, none.ToState())

	f := &UserFocus{UserId: NewId(), StartAt: 10, EndAt: 20, Reason: "r", PrevStatus: StatusAway, PrevCustomStatus: `{"emoji":"calendar"}`}
	assert.True(t, f.IsActive())
	assert.Equal(t, &FocusState{Active: true, StartAt: 10, EndAt: 20, Reason: "r"}, f.ToState())

	f.EndedAt = 15
	assert.False(t, f.IsActive())
	assert.Equal(t, &FocusState{StartAt: 10, EndAt: 20, Reason: "r", EndedAt: 15}, f.ToState())

	js, err := json.Marshal(f)
	require.NoError(t, err)
	assert.NotContains(t, string(js), "calendar", "the previous custom status never leaves the server")
	assert.NotContains(t, string(js), StatusAway)
}

func TestFocusWindowIsValid(t *testing.T) {
	now := GetMillis()
	hour := int64(time.Hour / time.Millisecond)
	assert.Nil(t, (&FocusWindow{Since: now - hour, Until: now}).IsValid(now))
	assert.Nil(t, (&FocusWindow{Since: now - FocusMaxDuration.Milliseconds(), Until: now}).IsValid(now), "the longest focus")
	assert.NotNil(t, (&FocusWindow{Since: 0, Until: now}).IsValid(now))
	assert.NotNil(t, (&FocusWindow{Since: now, Until: now - hour}).IsValid(now), "in order")
	assert.NotNil(t, (&FocusWindow{Since: now - hour, Until: now + hour}).IsValid(now), "not in the future")
	assert.NotNil(t, (&FocusWindow{Since: now - 30*24*hour, Until: now}).IsValid(now), "bounded")
}
