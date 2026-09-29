// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package accesscontrol

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"golang.org/x/mod/semver"
)

// CEL variables available to policy expressions.
const (
	celVarUser     = "user"
	celVarResource = "resource"
)

// Custom CEL functions understood by the engine, in addition to the standard
// library (startsWith, endsWith, contains, in, ==, ...).
const (
	fnHasAnyOf         = "hasAnyOf"
	fnHasAllOf         = "hasAllOf"
	fnCoversAll        = "coversAll"
	fnCoversAny        = "coversAny"
	fnWithinAll        = "withinAll"
	fnWithinAny        = "withinAny"
	fnYoungerThanDays  = "youngerThanDays"
	fnInCIDR           = "inCIDR"
	fnVersionEQ        = "versionEQ"
	fnVersionGT        = "versionGT"
	fnVersionGTE       = "versionGTE"
	fnVersionLT        = "versionLT"
	fnVersionLTE       = "versionLTE"
	millisecondsPerDay = int64(24 * time.Hour / time.Millisecond)
)

var graphFunctions = []string{fnCoversAll, fnCoversAny, fnWithinAll, fnWithinAny}

var versionFunctions = map[string]func(cmp int) bool{
	fnVersionEQ:  func(c int) bool { return c == 0 },
	fnVersionGT:  func(c int) bool { return c > 0 },
	fnVersionGTE: func(c int) bool { return c >= 0 },
	fnVersionLT:  func(c int) bool { return c < 0 },
	fnVersionLTE: func(c int) bool { return c <= 0 },
}

// graphResolver answers the hierarchy questions graph predicates ask.
type graphResolver interface {
	optionIDsByName(field *fieldInfo, names []string) (map[string]string, error)
	optionNamesByID(field *fieldInfo, ids []string) (map[string]string, error)
	ancestorsOrSelf(field *fieldInfo, ids []string) (map[string][]string, error)
	descendantsOrSelf(field *fieldInfo, ids []string) (map[string][]string, error)
}

// newCELEnv builds the CEL environment used to parse, check and evaluate
// every policy expression. `now` is injectable for tests.
func newCELEnv(now func() time.Time) (*cel.Env, error) {
	if now == nil {
		now = time.Now
	}

	opts := []cel.EnvOption{
		cel.Variable(celVarUser, cel.DynType),
		cel.Variable(celVarResource, cel.DynType),
		cel.EnableMacroCallTracking(),
		cel.CrossTypeNumericComparisons(true),

		cel.Function(fnHasAnyOf, cel.MemberOverload("mm_dyn_hasAnyOf_dyn",
			[]*cel.Type{cel.DynType, cel.DynType}, cel.BoolType,
			cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val { return listIntersects(lhs, rhs, false) }))),
		cel.Function(fnHasAllOf, cel.MemberOverload("mm_dyn_hasAllOf_dyn",
			[]*cel.Type{cel.DynType, cel.DynType}, cel.BoolType,
			cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val { return listIntersects(lhs, rhs, true) }))),

		cel.Function(fnYoungerThanDays, cel.MemberOverload("mm_int_youngerThanDays_int",
			[]*cel.Type{cel.IntType, cel.IntType}, cel.BoolType,
			cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
				createAt, ok1 := lhs.(types.Int)
				days, ok2 := rhs.(types.Int)
				if !ok1 || !ok2 {
					return types.NewErr("youngerThanDays expects integer operands")
				}
				if days < 0 {
					return types.NewErr("youngerThanDays expects a non-negative number of days")
				}
				if createAt <= 0 {
					return types.False
				}
				threshold := now().UnixMilli() - int64(days)*millisecondsPerDay
				return types.Bool(int64(createAt) > threshold)
			}))),

		cel.Function(fnInCIDR, cel.MemberOverload("mm_string_inCIDR_string",
			[]*cel.Type{cel.StringType, cel.StringType}, cel.BoolType,
			cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
				ip, ok1 := lhs.(types.String)
				cidr, ok2 := rhs.(types.String)
				if !ok1 || !ok2 {
					return types.NewErr("inCIDR expects string operands")
				}
				return inCIDR(string(ip), string(cidr))
			}))),
	}

	for _, name := range graphFunctions {
		opts = append(opts, cel.Function(name, cel.MemberOverload("mm_dyn_"+name+"_dyn",
			[]*cel.Type{cel.DynType, cel.DynType}, cel.BoolType,
			cel.BinaryBinding(graphPredicate(name)))))
	}

	for name, accept := range versionFunctions {
		opts = append(opts, cel.Function(name, cel.MemberOverload("mm_string_"+name+"_string",
			[]*cel.Type{cel.StringType, cel.StringType}, cel.BoolType,
			cel.BinaryBinding(versionComparison(name, accept)))))
	}

	return cel.NewEnv(opts...)
}

// toStringSlice converts a CEL list (or a graph value, whose elements are
// option IDs) into a string slice.
func toStringSlice(val ref.Val) ([]string, error) {
	switch v := val.(type) {
	case *graphValue:
		return v.ids, nil
	case traits.Lister:
		size, ok := v.Size().(types.Int)
		if !ok {
			return nil, fmt.Errorf("invalid list size")
		}
		out := make([]string, 0, int(size))
		for i := types.Int(0); i < size; i++ {
			elem := v.Get(i)
			switch e := elem.(type) {
			case types.String:
				out = append(out, string(e))
			case *rankValue:
				out = append(out, e.name)
			default:
				if types.IsError(elem) {
					return nil, fmt.Errorf("%v", elem)
				}
				out = append(out, fmt.Sprint(elem.Value()))
			}
		}
		return out, nil
	case types.String:
		return []string{string(v)}, nil
	}
	return nil, fmt.Errorf("expected a list, got %s", val.Type().TypeName())
}

// listIntersects implements hasAnyOf (all=false) and hasAllOf (all=true).
func listIntersects(lhs, rhs ref.Val, all bool) ref.Val {
	if types.IsError(lhs) {
		return lhs
	}
	if types.IsError(rhs) {
		return rhs
	}
	if _, isGraph := lhs.(*graphValue); isGraph {
		return types.NewErr("hasAnyOf/hasAllOf are not supported on graph attributes; use coversAll/coversAny/withinAll/withinAny or `in`")
	}
	held, err := toStringSlice(lhs)
	if err != nil {
		return types.NewErr("%s", err.Error())
	}
	wanted, err := toStringSlice(rhs)
	if err != nil {
		return types.NewErr("%s", err.Error())
	}
	if len(wanted) == 0 {
		// An empty target list is never a grant: it is what a fully masked
		// condition looks like, and must not widen access.
		return types.False
	}
	heldSet := make(map[string]struct{}, len(held))
	for _, h := range held {
		heldSet[h] = struct{}{}
	}
	for _, w := range wanted {
		_, ok := heldSet[w]
		if all && !ok {
			return types.False
		}
		if !all && ok {
			return types.True
		}
	}
	return types.Bool(all)
}

// graphPredicate implements coversAll/coversAny (the holder is at or above a
// target) and withinAll/withinAny (the holder is at or below a target). The
// argument is a list of option names or another graph value (e.g. the accessed
// channel's attribute), whose elements are option IDs.
func graphPredicate(name string) func(lhs, rhs ref.Val) ref.Val {
	up := name == fnCoversAll || name == fnCoversAny
	all := name == fnCoversAll || name == fnWithinAll

	return func(lhs, rhs ref.Val) ref.Val {
		if types.IsError(lhs) {
			return lhs
		}
		if types.IsError(rhs) {
			return rhs
		}
		g, ok := lhs.(*graphValue)
		if !ok {
			return types.NewErr("%s requires a graph attribute", name)
		}
		if g.graph == nil {
			return types.NewErr("graph attribute is not resolvable")
		}

		var targetIDs []string
		missing := false
		switch r := rhs.(type) {
		case *graphValue:
			targetIDs = r.ids
		default:
			names, err := toStringSlice(rhs)
			if err != nil {
				return types.NewErr("%s: %s", name, err.Error())
			}
			idsByName, err := g.graph.optionIDsByName(g.field, names)
			if err != nil {
				return types.WrapErr(err)
			}
			for _, n := range names {
				id, found := idsByName[n]
				if !found {
					missing = true
					continue
				}
				targetIDs = append(targetIDs, id)
			}
		}

		if len(targetIDs) == 0 {
			return types.False
		}
		if all && missing {
			// A target that is not an option of the field is covered by nothing.
			return types.False
		}

		var closure map[string][]string
		var err error
		if up {
			closure, err = g.graph.ancestorsOrSelf(g.field, targetIDs)
		} else {
			closure, err = g.graph.descendantsOrSelf(g.field, targetIDs)
		}
		if err != nil {
			return types.WrapErr(err)
		}

		held := make(map[string]struct{}, len(g.ids))
		for _, id := range g.ids {
			held[id] = struct{}{}
		}

		for _, t := range targetIDs {
			satisfied := false
			for _, candidate := range closure[t] {
				if _, ok := held[candidate]; ok {
					satisfied = true
					break
				}
			}
			if all && !satisfied {
				return types.False
			}
			if !all && satisfied {
				return types.True
			}
		}
		return types.Bool(all)
	}
}

func inCIDR(ipStr, cidrStr string) ref.Val {
	ipStr = strings.TrimSpace(ipStr)
	if host, _, err := net.SplitHostPort(ipStr); err == nil {
		ipStr = host
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return types.NewErr("invalid IP address %q", ipStr)
	}
	_, network, err := net.ParseCIDR(strings.TrimSpace(cidrStr))
	if err != nil {
		// Accept a bare address as a single-host range.
		single := net.ParseIP(strings.TrimSpace(cidrStr))
		if single == nil {
			return types.NewErr("invalid CIDR range %q", cidrStr)
		}
		return types.Bool(single.Equal(ip))
	}
	return types.Bool(network.Contains(ip))
}

func canonicalSemver(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	if !semver.IsValid(v) {
		return ""
	}
	return v
}

func versionComparison(name string, accept func(int) bool) func(lhs, rhs ref.Val) ref.Val {
	return func(lhs, rhs ref.Val) ref.Val {
		l, ok1 := lhs.(types.String)
		r, ok2 := rhs.(types.String)
		if !ok1 || !ok2 {
			return types.NewErr("%s expects string operands", name)
		}
		lv, rv := canonicalSemver(string(l)), canonicalSemver(string(r))
		if lv == "" {
			return types.NewErr("invalid version %q", string(l))
		}
		if rv == "" {
			return types.NewErr("invalid version %q", string(r))
		}
		return types.Bool(accept(semver.Compare(lv, rv)))
	}
}
