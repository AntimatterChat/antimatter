// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package healthcheck

import (
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuleAppliesTo(t *testing.T) {
	t.Parallel()

	cloudRule := validRule("CLOUD")
	cloudRule.AppliesToCloud = true

	onPremRule := validRule("ONPREM")
	onPremRule.AppliesToCloud = false

	require.True(t, cloudRule.AppliesTo(Deployment{IsCloud: true}))
	require.False(t, onPremRule.AppliesTo(Deployment{IsCloud: true}))
	require.True(t, onPremRule.AppliesTo(Deployment{IsCloud: false}))
	require.True(t, cloudRule.AppliesTo(Deployment{IsCloud: false}))
}

func validRule(code string) Rule {
	return Rule{
		Code:       code,
		Area:       model.AreaPlatform,
		Severity:   SeverityWarning,
		RuleText:   model.RuleText{TitleID: "health.rule." + strings.ToLower(code) + ".title", RemediationID: "health.rule." + strings.ToLower(code) + ".remediation"},
		Surface:    SurfaceProduct,
		Volatility: VolatilityStable,
		Subject:    "ServiceSettings.SiteURL",
		Eval:       func(*Snapshot) []Result { return []Result{Resolved()} },
	}
}

func TestRuleIsNodeScoped(t *testing.T) {
	t.Parallel()

	require.False(t, validRule("WORKSPACE").IsNodeScoped())

	nodeRule := validRule("NODE")
	nodeRule.EvalNode = func(*Snapshot, *NodeSnapshot) []Result { return nil }
	require.True(t, nodeRule.IsNodeScoped())
}

func TestGate(t *testing.T) {
	t.Parallel()

	firing := Firing("message")
	check := func(*Snapshot) Result { return firing }
	nodeCheck := func(*Snapshot, *NodeSnapshot) Result { return firing }

	testCases := []struct {
		name     string
		enabled  bool
		ok       bool
		expected Result
	}{
		{name: "gate unknown", ok: false, expected: Unknown(ReasonConfigUnavailable)},
		{name: "gate off", enabled: false, ok: true, expected: Resolved()},
		{name: "gate on", enabled: true, ok: true, expected: firing},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gate := func(*Snapshot) (bool, bool) { return tc.enabled, tc.ok }
			assert.Equal(t, []Result{tc.expected}, Gate(gate, ReasonConfigUnavailable, check)(&Snapshot{}))
			assert.Equal(t, []Result{tc.expected}, GateNode(gate, ReasonConfigUnavailable, nodeCheck)(&Snapshot{}, &NodeSnapshot{}))
		})
	}
}
