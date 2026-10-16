// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package accesscontrol

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

// rankCELType is the runtime type of a ranked attribute value. It declares the
// comparer trait so the standard ordering operators (<, <=, >, >=) dispatch to
// rankValue.Compare.
var rankCELType = types.NewObjectType("mattermost.accesscontrol.Rank", traits.ComparerType)

// graphCELType is the runtime type of a graph (hierarchical) attribute value.
// It declares the container trait so `"Option" in user.attributes.graphField`
// tests exact membership by option name.
var graphCELType = types.NewObjectType("mattermost.accesscontrol.Graph", traits.ContainerType)

// rankValue is the CEL value of a `rank` attribute: an option name together
// with its rank. Equality compares option names, ordering compares ranks. A
// string operand is interpreted as an option name of the same field and
// resolved to its rank.
type rankValue struct {
	name    string
	rank    int64
	hasRank bool
	ranks   map[string]int64
}

func (r *rankValue) ConvertToNative(typeDesc reflect.Type) (any, error) {
	switch typeDesc.Kind() {
	case reflect.String:
		return r.name, nil
	case reflect.Int, reflect.Int64:
		return r.rank, nil
	case reflect.Interface:
		return r.name, nil
	}
	return nil, fmt.Errorf("type conversion error from rank to %v", typeDesc)
}

func (r *rankValue) ConvertToType(typeVal ref.Type) ref.Val {
	switch typeVal {
	case types.StringType:
		return types.String(r.name)
	case types.IntType:
		return types.Int(r.rank)
	case types.TypeType:
		return rankCELType
	}
	return types.NewErr("type conversion error from rank to '%s'", typeVal)
}

func (r *rankValue) Equal(other ref.Val) ref.Val {
	switch o := other.(type) {
	case types.String:
		return types.Bool(r.name == string(o))
	case *rankValue:
		return types.Bool(r.name == o.name)
	}
	return types.False
}

func (r *rankValue) Type() ref.Type {
	return rankCELType
}

func (r *rankValue) Value() any {
	return r.name
}

// Compare implements traits.Comparer.
func (r *rankValue) Compare(other ref.Val) ref.Val {
	if !r.hasRank {
		return types.NewErr("option %q has no rank", r.name)
	}
	var otherRank int64
	switch o := other.(type) {
	case types.String:
		rank, ok := r.ranks[string(o)]
		if !ok {
			return types.NewErr("unknown ranked option %q", string(o))
		}
		otherRank = rank
	case *rankValue:
		if !o.hasRank {
			return types.NewErr("option %q has no rank", o.name)
		}
		otherRank = o.rank
	case types.Int:
		otherRank = int64(o)
	default:
		return types.MaybeNoSuchOverloadErr(other)
	}
	switch {
	case r.rank < otherRank:
		return types.IntNegOne
	case r.rank > otherRank:
		return types.IntOne
	}
	return types.IntZero
}

// graphValue is the CEL value of a `graph` attribute: the set of option IDs
// the object holds, plus what is needed to resolve option names and walk the
// field's hierarchy.
type graphValue struct {
	ids   []string
	field *fieldInfo
	graph graphResolver
}

func (g *graphValue) ConvertToNative(typeDesc reflect.Type) (any, error) {
	if typeDesc.Kind() == reflect.Slice || typeDesc.Kind() == reflect.Interface {
		return slices.Clone(g.ids), nil
	}
	return nil, fmt.Errorf("type conversion error from graph to %v", typeDesc)
}

func (g *graphValue) ConvertToType(typeVal ref.Type) ref.Val {
	if typeVal == types.TypeType {
		return graphCELType
	}
	return types.NewErr("type conversion error from graph to '%s'", typeVal)
}

func (g *graphValue) Equal(other ref.Val) ref.Val {
	o, ok := other.(*graphValue)
	if !ok {
		return types.False
	}
	if len(o.ids) != len(g.ids) {
		return types.False
	}
	for _, id := range g.ids {
		if !slices.Contains(o.ids, id) {
			return types.False
		}
	}
	return types.True
}

func (g *graphValue) Type() ref.Type {
	return graphCELType
}

func (g *graphValue) Value() any {
	return g.ids
}

// Contains implements traits.Container: exact, hierarchy-blind membership of
// an option given by name.
func (g *graphValue) Contains(value ref.Val) ref.Val {
	name, ok := value.(types.String)
	if !ok {
		return types.MaybeNoSuchOverloadErr(value)
	}
	if g.graph == nil || g.field == nil {
		return types.NewErr("graph attribute is not resolvable")
	}
	idsByName, err := g.graph.optionIDsByName(g.field, []string{string(name)})
	if err != nil {
		return types.WrapErr(err)
	}
	id, found := idsByName[string(name)]
	if !found {
		return types.False
	}
	return types.Bool(slices.Contains(g.ids, id))
}
