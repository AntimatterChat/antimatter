// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package ipfiltering

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

func allow(cidr string) model.AllowedIPRange {
	return model.AllowedIPRange{CIDRBlock: cidr, Enabled: true, Action: model.IPFilterActionAllow}
}

func deny(cidr string) model.AllowedIPRange {
	return model.AllowedIPRange{CIDRBlock: cidr, Enabled: true, Action: model.IPFilterActionDeny}
}

func TestRulesAllows(t *testing.T) {
	cases := map[string]struct {
		ranges  model.AllowedIPRanges
		allowed []string
		refused []string
	}{
		"no rules lets everyone through": {
			allowed: []string{"192.0.2.1", "2001:db8::1", "not-an-ip"},
		},
		"disabled rules are ignored": {
			ranges:  model.AllowedIPRanges{{CIDRBlock: "192.0.2.0/24", Enabled: false, Action: model.IPFilterActionDeny}},
			allowed: []string{"192.0.2.1"},
		},
		"allow rules form an allowlist": {
			ranges:  model.AllowedIPRanges{allow("192.0.2.0/24"), allow("2001:db8::/32")},
			allowed: []string{"192.0.2.1", "192.0.2.254", "2001:db8::1"},
			refused: []string{"198.51.100.1", "2001:db9::1", "not-an-ip", ""},
		},
		"a rule without an action is an allow rule": {
			ranges:  model.AllowedIPRanges{{CIDRBlock: "192.0.2.0/24", Enabled: true}},
			allowed: []string{"192.0.2.1"},
			refused: []string{"198.51.100.1"},
		},
		"deny rules alone form a blocklist": {
			ranges:  model.AllowedIPRanges{deny("192.0.2.0/24")},
			allowed: []string{"198.51.100.1", "2001:db8::1"},
			refused: []string{"192.0.2.1"},
		},
		"deny wins over allow": {
			ranges:  model.AllowedIPRanges{allow("10.0.0.0/8"), deny("10.1.0.0/16")},
			allowed: []string{"10.2.0.1"},
			refused: []string{"10.1.2.3", "192.0.2.1"},
		},
		"a bare address is a single host": {
			ranges:  model.AllowedIPRanges{deny("192.0.2.7")},
			allowed: []string{"192.0.2.8"},
			refused: []string{"192.0.2.7"},
		},
		"IPv4-mapped IPv6 clients match IPv4 rules": {
			ranges:  model.AllowedIPRanges{allow("192.0.2.0/24")},
			allowed: []string{"::ffff:192.0.2.1"},
			refused: []string{"::ffff:198.51.100.1"},
		},
		"IPv4-mapped IPv6 rules match IPv4 clients": {
			ranges:  model.AllowedIPRanges{deny("::ffff:192.0.2.0/120")},
			allowed: []string{"192.0.3.1"},
			refused: []string{"192.0.2.1"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rules, err := Compile(tc.ranges)
			require.NoError(t, err)
			for _, ip := range tc.allowed {
				assert.True(t, rules.AllowsString(ip), "%s should be allowed", ip)
			}
			for _, ip := range tc.refused {
				assert.False(t, rules.AllowsString(ip), "%s should be refused", ip)
			}
		})
	}
}

func TestCompileRejectsInvalidRanges(t *testing.T) {
	_, err := Compile(model.AllowedIPRanges{allow("192.0.2.0/33")})
	require.Error(t, err)

	// Invalid disabled rules are ignored; config validation rejects them separately.
	_, err = Compile(model.AllowedIPRanges{{CIDRBlock: "nope", Enabled: false}})
	require.NoError(t, err)
}

func TestMiddleware(t *testing.T) {
	var f Filter
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	clientIP := func(r *http.Request) string { return r.Header.Get("X-Test-IP") }
	handler := f.Middleware(next, clientIP, mlog.CreateConsoleTestLogger(t))

	serve := func(ip string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/v4/system/ping", nil)
		req.Header.Set("X-Test-IP", ip)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	// No rules set yet.
	assert.Equal(t, http.StatusTeapot, serve("192.0.2.1"))

	rules, err := Compile(model.AllowedIPRanges{deny("192.0.2.0/24")})
	require.NoError(t, err)
	f.Set(rules)

	assert.Equal(t, http.StatusForbidden, serve("192.0.2.1"))
	assert.Equal(t, http.StatusTeapot, serve("198.51.100.1"))

	// Rules are swapped live.
	rules, err = Compile(nil)
	require.NoError(t, err)
	f.Set(rules)
	assert.Equal(t, http.StatusTeapot, serve("192.0.2.1"))
}
