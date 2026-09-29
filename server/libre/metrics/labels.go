// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package metrics

import (
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	// otherLabelValue is used in place of label values that are either invalid or
	// would push a metric over its cardinality budget.
	otherLabelValue = "other"

	// maxLabelValueLength is the maximum length (in bytes) of a label value coming from
	// an untrusted source (e.g. client performance reports).
	maxLabelValueLength = 64
)

// boundedLabel keeps track of the distinct values seen for a label whose values come from
// an untrusted source (e.g. labels provided by clients in performance reports) and caps the
// number of distinct values. Once the budget is exhausted, new values are reported as
// "other". This keeps the cardinality of the exposed metrics bounded no matter what
// clients send.
type boundedLabel struct {
	mut  sync.RWMutex
	max  int
	seen map[string]struct{}
}

func newBoundedLabel(maxValues int) *boundedLabel {
	return &boundedLabel{
		max:  maxValues,
		seen: make(map[string]struct{}, maxValues),
	}
}

// value returns the label value to use for v.
func (b *boundedLabel) value(v string) string {
	v = sanitizeLabelValue(v)
	if v == "" || v == otherLabelValue {
		return v
	}

	b.mut.RLock()
	_, ok := b.seen[v]
	b.mut.RUnlock()
	if ok {
		return v
	}

	b.mut.Lock()
	defer b.mut.Unlock()
	if _, ok = b.seen[v]; ok {
		return v
	}
	if len(b.seen) >= b.max {
		return otherLabelValue
	}
	b.seen[v] = struct{}{}
	return v
}

// reset forgets every value seen so far.
func (b *boundedLabel) reset() {
	b.mut.Lock()
	defer b.mut.Unlock()
	b.seen = make(map[string]struct{}, b.max)
}

// sanitizeLabelValue trims a label value, truncates it to a sane length and makes sure it is
// valid UTF-8 (which Prometheus requires).
func sanitizeLabelValue(v string) string {
	v = strings.TrimSpace(v)
	if !utf8.ValidString(v) {
		v = strings.ToValidUTF8(v, "")
	}
	if len(v) > maxLabelValueLength {
		v = v[:maxLabelValueLength]
		// Don't leave a partial rune behind.
		for !utf8.ValidString(v) {
			v = v[:len(v)-1]
		}
	}
	return v
}

// allowedLabel returns v if it's one of the accepted values, otherwise "other".
func allowedLabel(v string, accepted map[string]any) string {
	if _, ok := accepted[v]; ok {
		return v
	}
	// Accept case-insensitive matches but always report the canonical form.
	for k := range accepted {
		if strings.EqualFold(k, v) {
			return k
		}
	}
	return otherLabelValue
}

func boolLabel(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
