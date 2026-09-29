// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllowedIPRangePrefix(t *testing.T) {
	cases := map[string]string{
		"192.0.2.0/24":         "192.0.2.0/24",
		"192.0.2.77/24":        "192.0.2.0/24",
		" 192.0.2.7 ":          "192.0.2.7/32",
		"2001:db8::1":          "2001:db8::1/128",
		"2001:db8::/32":        "2001:db8::/32",
		"::ffff:192.0.2.0/120": "192.0.2.0/24",
		"::ffff:192.0.2.7":     "192.0.2.7/32",
		"0.0.0.0/0":            "0.0.0.0/0",
	}
	for in, want := range cases {
		r := AllowedIPRange{CIDRBlock: in}
		p, err := r.Prefix()
		require.NoError(t, err, in)
		assert.Equal(t, want, p.String(), in)
	}

	for _, in := range []string{"", "not-an-ip", "192.0.2.0/33", "fe80::1%eth0", "::ffff:0.0.0.0/64"} {
		r := AllowedIPRange{CIDRBlock: in}
		_, err := r.Prefix()
		assert.Error(t, err, in)
	}
}

func TestAllowedIPRangesIsValid(t *testing.T) {
	valid := AllowedIPRanges{
		{CIDRBlock: "192.0.2.0/24", Enabled: true},
		{CIDRBlock: "198.51.100.0/24", Enabled: true, Action: IPFilterActionAllow},
		{CIDRBlock: "203.0.113.9", Enabled: false, Action: IPFilterActionDeny},
	}
	require.Nil(t, valid.IsValid())

	for name, r := range map[string]AllowedIPRange{
		"bad range":        {CIDRBlock: "nope"},
		"bad action":       {CIDRBlock: "192.0.2.0/24", Action: "block"},
		"long description": {CIDRBlock: "192.0.2.0/24", Description: strings.Repeat("x", IPFilterDescriptionMaxRunes+1)},
	} {
		appErr := AllowedIPRanges{r}.IsValid()
		assert.NotNil(t, appErr, name)
	}

	cfg := &Config{}
	cfg.SetDefaults()
	assert.NotNil(t, cfg.IPFilteringSettings.Rules, "rules default to an empty list")
	cfg.IPFilteringSettings.Rules = AllowedIPRanges{{CIDRBlock: "nope", Enabled: true}}
	assert.NotNil(t, cfg.IsValid(), "config validation checks the rules")
}
