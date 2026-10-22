// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package healthcheck

import "github.com/mattermost/mattermost/server/public/model"

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

type State string

const (
	StateFiring   State = "firing"
	StateResolved State = "resolved"
	StateUnknown  State = "unknown"
)

type Volatility string

const (
	VolatilityStable    Volatility = "stable"
	VolatilityProbe     Volatility = "probe"
	VolatilityTopology  Volatility = "topology"
	VolatilityThreshold Volatility = "threshold"
	VolatilityFeed      Volatility = "feed"
)

type Surface string

const (
	SurfaceProduct  Surface = "product"
	SurfaceInternal Surface = "internal"
)

type Rule struct {
	Code     string           `json:"code"`
	Area     model.HealthArea `json:"area"`
	Severity Severity         `json:"severity"`
	model.RuleText

	Surface        Surface    `json:"surface"`
	Volatility     Volatility `json:"volatility"`
	AppliesToCloud bool       `json:"applies_to_cloud"`
	Subject        string     `json:"subject,omitempty"`

	Eval     func(*Snapshot) []Result                `json:"-"`
	EvalNode func(*Snapshot, *NodeSnapshot) []Result `json:"-"`
}

func (r Rule) IsNodeScoped() bool {
	return r.EvalNode != nil
}

func (r Rule) AppliesTo(d Deployment) bool {
	if d.IsCloud {
		return r.AppliesToCloud
	}

	return true
}

// Single adapts a check that yields exactly one result to Rule.Eval.
func Single(check func(*Snapshot) Result) func(*Snapshot) []Result {
	return func(s *Snapshot) []Result {
		return []Result{check(s)}
	}
}

// SingleNode adapts a node check that yields exactly one result to Rule.EvalNode.
func SingleNode(check func(*Snapshot, *NodeSnapshot) Result) func(*Snapshot, *NodeSnapshot) []Result {
	return func(s *Snapshot, n *NodeSnapshot) []Result {
		return []Result{check(s, n)}
	}
}

// Gate resolves the rule while gate reports its feature off, so settings that cannot take effect never fire.
// unknownReason is reported when gate cannot tell.
func Gate(gate func(*Snapshot) (enabled, ok bool), unknownReason string, check func(*Snapshot) Result) func(*Snapshot) []Result {
	return func(s *Snapshot) []Result {
		if r, closed := gateClosed(s, gate, unknownReason); closed {
			return []Result{r}
		}
		return []Result{check(s)}
	}
}

// GateNode is Gate for node-scoped rules.
func GateNode(gate func(*Snapshot) (enabled, ok bool), unknownReason string, check func(*Snapshot, *NodeSnapshot) Result) func(*Snapshot, *NodeSnapshot) []Result {
	return func(s *Snapshot, n *NodeSnapshot) []Result {
		if r, closed := gateClosed(s, gate, unknownReason); closed {
			return []Result{r}
		}
		return []Result{check(s, n)}
	}
}

func gateClosed(s *Snapshot, gate func(*Snapshot) (bool, bool), unknownReason string) (Result, bool) {
	enabled, ok := gate(s)
	switch {
	case !ok:
		return Unknown(unknownReason), true
	case !enabled:
		return Resolved(), true
	default:
		return Result{}, false
	}
}
