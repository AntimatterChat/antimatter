// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"math"
)

const (
	// BurnOnReadMinimumDurationSeconds is the shortest a burn-on-read message lives once read, whatever its sender
	// asks (an administrator's maximum below it still wins).
	BurnOnReadMinimumDurationSeconds = 60

	// BurnOnReadDefaultMaximumDurationSeconds is the default of ServiceSettings.BurnOnReadMaximumDurationSeconds: a
	// day, the longest step of the clients' duration picker.
	BurnOnReadDefaultMaximumDurationSeconds = 24 * 60 * 60
)

// BurnOnReadDurationMillis is how long a burn-on-read message lives once read, in milliseconds as the post's
// PostPropsReadDurationSeconds prop stores it, from what its sender asked in that prop (requested, in milliseconds):
// the default when they asked nothing, else what they asked, to the second, kept between
// BurnOnReadMinimumDurationSeconds and maxSeconds. The default is kept under maxSeconds too. ok is false when
// requested isn't a duration: not a positive number.
func BurnOnReadDurationMillis(requested any, defaultSeconds, maxSeconds int) (millis int64, ok bool) {
	if maxSeconds <= 0 {
		maxSeconds = BurnOnReadDefaultMaximumDurationSeconds
	}
	if requested == nil {
		seconds := min(defaultSeconds, maxSeconds)
		if seconds <= 0 {
			seconds = min(BurnOnReadMinimumDurationSeconds, maxSeconds)
		}
		return int64(seconds) * 1000, true
	}

	var ms float64
	switch v := requested.(type) {
	case float64:
		ms = v
	case float32:
		ms = float64(v)
	case int:
		ms = float64(v)
	case int64:
		ms = float64(v)
	case int32:
		ms = float64(v)
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return 0, false
		}
		ms = f
	default:
		return 0, false
	}
	if math.IsNaN(ms) || math.IsInf(ms, 0) || ms <= 0 {
		return 0, false
	}

	// Compared as floats first, so that a huge number can't overflow on its way to an integer.
	lowest := min(BurnOnReadMinimumDurationSeconds, maxSeconds)
	seconds := math.Round(ms / 1000)
	switch {
	case seconds >= float64(maxSeconds):
		return int64(maxSeconds) * 1000, true
	case seconds <= float64(lowest):
		return int64(lowest) * 1000, true
	}
	return int64(seconds) * 1000, true
}
