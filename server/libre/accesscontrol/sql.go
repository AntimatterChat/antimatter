// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package accesscontrol

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	celast "github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/operators"

	"github.com/mattermost/mattermost/server/public/model"
)

// The SQL produced here is consumed by the attribute store's SearchUsers,
// GetChannelMembersToRemove and GetTeamMembersToRemove, which join Users with
// UserAttributeView (one JSONB object of attribute name → value per user) and
// number their own parameters after ours, so placeholders are PostgreSQL
// positional parameters ($1, $2, ...).
//
// Three-valued SQL logic mirrors CEL's error semantics closely enough: a
// missing attribute yields NULL, which never matches, just as a CEL error
// never grants access.

const attributesColumn = "UserAttributeView.Attributes"

// errUnsupportedSQL marks expressions that are valid CEL but cannot be turned
// into a database query.
type errUnsupportedSQL struct{ reason string }

func (e *errUnsupportedSQL) Error() string {
	return "expression cannot be evaluated against the database: " + e.reason
}

func unsupported(format string, args ...any) error {
	return &errUnsupportedSQL{reason: fmt.Sprintf(format, args...)}
}

type sqlBuilder struct {
	cat   *catalog
	graph graphResolver
	args  []any
	now   time.Time

	// resource holds the accessed channel's raw attribute values, keyed by
	// name. hasResource is false when no resource is in scope.
	resource    map[string]any
	hasResource bool

	excludeNative bool
}

// operand is one side of a test.
type operand struct {
	literal   any
	isLiteral bool

	ref   attrRef
	isRef bool
	field *fieldInfo

	// resolved resource.attributes.* value (constant for the query)
	resValue   any
	isResource bool
	resMissing bool
}

func (b *sqlBuilder) param(v any, cast string) string {
	b.args = append(b.args, v)
	return fmt.Sprintf("$%d::%s", len(b.args), cast)
}

func (b *sqlBuilder) operandOf(e celast.Expr) (operand, error) {
	if v, ok := literalOf(e); ok {
		return operand{literal: v, isLiteral: true}, nil
	}
	if list, ok := stringListOf(e); ok {
		vals := make([]any, len(list))
		for i, s := range list {
			vals[i] = s
		}
		return operand{literal: vals, isLiteral: true}, nil
	}
	r, ok := refOf(e)
	if !ok {
		return operand{}, unsupported("unsupported operand")
	}
	switch r.scope {
	case scopeSession:
		return operand{}, unsupported("session attributes (%s) are only known at request time", r.path())
	case scopeResource:
		if !b.hasResource {
			return operand{}, unsupported("the expression references channel attributes but no channel is in scope")
		}
		v, found := b.resource[r.name]
		return operand{isResource: true, resValue: v, resMissing: !found || v == nil, ref: r}, nil
	case scopeUser:
		return operand{isRef: true, ref: r, field: b.cat.lookupScope(scopeUser, r.name)}, nil
	case scopeNative:
		return operand{isRef: true, ref: r}, nil
	}
	return operand{}, unsupported("unsupported operand")
}

func (b *sqlBuilder) build(e celast.Expr) (string, error) {
	switch e.Kind() {
	case celast.LiteralKind:
		v, _ := literalOf(e)
		if bv, ok := v.(bool); ok {
			if bv {
				return "TRUE", nil
			}
			return "FALSE", nil
		}
		return "", unsupported("non-boolean constant")
	case celast.SelectKind:
		return b.boolRef(e)
	case celast.CallKind:
		call := e.AsCall()
		fn := call.FunctionName()
		if call.IsMemberFunction() {
			return b.method(fn, call.Target(), call.Args())
		}
		args := call.Args()
		switch fn {
		case operators.LogicalAnd, operators.LogicalOr:
			parts := make([]string, 0, len(args))
			for _, a := range flatten(e, fn) {
				p, err := b.build(a)
				if err != nil {
					return "", err
				}
				parts = append(parts, p)
			}
			joiner := " AND "
			if fn == operators.LogicalOr {
				joiner = " OR "
			}
			return "(" + strings.Join(parts, joiner) + ")", nil
		case operators.LogicalNot:
			inner, err := b.build(args[0])
			if err != nil {
				return "", err
			}
			return "(NOT " + inner + ")", nil
		case operators.In:
			return b.in(args[0], args[1])
		}
		if op, ok := comparisonOperators[fn]; ok && len(args) == 2 {
			return b.compare(op, args[0], args[1])
		}
		return "", unsupported("function %s", fn)
	}
	return "", unsupported("expression kind %v", e.Kind())
}

func (b *sqlBuilder) keyParam(name string) string {
	return b.param(name, "text")
}

func (b *sqlBuilder) jsonAttr(name string) string {
	return "(" + attributesColumn + " -> " + b.keyParam(name) + ")"
}

func (b *sqlBuilder) textAttr(o operand) string {
	if o.field != nil && o.field.Type == model.PropertyFieldTypeRank {
		return "((" + attributesColumn + " -> " + b.keyParam(o.ref.name) + ") ->> 'name')"
	}
	return "(" + attributesColumn + " ->> " + b.keyParam(o.ref.name) + ")"
}

func nativeColumn(name string) string {
	switch name {
	case nativeID:
		return "Users.Id"
	case nativeEmail:
		return "Users.Email"
	case nativeVerified:
		return "Users.EmailVerified"
	case nativeIsBot:
		return "(EXISTS (SELECT 1 FROM Bots WHERE Bots.UserId = Users.Id))"
	case nativeCreateAt:
		return "Users.CreateAt"
	}
	return ""
}

func (b *sqlBuilder) boolRef(e celast.Expr) (string, error) {
	r, ok := refOf(e)
	if !ok {
		return "", unsupported("unsupported selector")
	}
	switch r.scope {
	case scopeNative:
		if b.excludeNative {
			return "TRUE", nil
		}
		if r.name == nativeVerified || r.name == nativeIsBot {
			return nativeColumn(r.name), nil
		}
	case scopeUser:
		return "(" + b.jsonAttr(r.name) + " = 'true'::jsonb)", nil
	case scopeResource:
		o, err := b.operandOf(e)
		if err != nil {
			return "", err
		}
		if bv, ok := o.resValue.(bool); ok && bv {
			return "TRUE", nil
		}
		return "FALSE", nil
	}
	return "", unsupported("%s is not a boolean", r.path())
}

// constantValue returns the literal value of a literal or resource operand.
// ok is false when the resource value is missing.
func constantValue(o operand) (any, bool) {
	if o.isLiteral {
		return o.literal, true
	}
	if o.isResource {
		if o.resMissing {
			return nil, false
		}
		return o.resValue, true
	}
	return nil, false
}

const sqlUnknown = "(NULL::boolean)"

func (b *sqlBuilder) compare(op string, left, right celast.Expr) (string, error) {
	l, err := b.operandOf(left)
	if err != nil {
		return "", err
	}
	r, err := b.operandOf(right)
	if err != nil {
		return "", err
	}
	if !l.isRef && r.isRef {
		l, r = r, l
		op = flippedComparison[op]
	}
	if !l.isRef {
		// Both sides constant (literal or resource value).
		lv, lok := constantValue(l)
		rv, rok := constantValue(r)
		if !lok || !rok {
			return sqlUnknown, nil
		}
		return constantCompare(op, lv, rv)
	}
	if r.isRef {
		return "", unsupported("comparing two user attributes")
	}
	if l.ref.scope == scopeNative && b.excludeNative {
		return "TRUE", nil
	}

	value, ok := constantValue(r)
	if !ok {
		return sqlUnknown, nil
	}

	sqlOp := op
	if op == "==" {
		sqlOp = "="
	} else if op == "!=" {
		sqlOp = "<>"
	}

	if l.ref.scope == scopeNative {
		col := nativeColumn(l.ref.name)
		switch l.ref.name {
		case nativeVerified, nativeIsBot:
			bv, isBool := value.(bool)
			if !isBool || (op != "==" && op != "!=") {
				return "", unsupported("%s only supports == and != with true/false", l.ref.path())
			}
			return "(" + col + " " + sqlOp + " " + b.param(bv, "boolean") + ")", nil
		case nativeCreateAt:
			n, isNum := toInt64(value)
			if !isNum {
				if i, ok := value.(int64); ok {
					n, isNum = i, true
				}
			}
			if !isNum {
				return "", unsupported("user.createat compares with integers")
			}
			return "(" + col + " " + sqlOp + " " + b.param(n, "bigint") + ")", nil
		default:
			s, isString := value.(string)
			if !isString {
				return "", unsupported("%s compares with strings", l.ref.path())
			}
			return "(" + col + " " + sqlOp + " " + b.param(s, "text") + ")", nil
		}
	}

	// Custom attribute.
	if l.field != nil && l.field.Type == model.PropertyFieldTypeRank {
		name, rank, hasRank := rankOperand(value, l.field)
		if op == "==" || op == "!=" {
			if name == "" {
				return sqlUnknown, nil
			}
			return "(" + b.textAttr(l) + " " + sqlOp + " " + b.param(name, "text") + ")", nil
		}
		if !hasRank {
			return sqlUnknown, nil
		}
		return "(((" + attributesColumn + " -> " + b.keyParam(l.ref.name) + ") ->> 'rank')::bigint " + sqlOp + " " + b.param(rank, "bigint") + ")", nil
	}

	switch v := value.(type) {
	case string:
		return "(" + b.textAttr(l) + " " + sqlOp + " " + b.param(v, "text") + ")", nil
	case bool:
		if op != "==" && op != "!=" {
			return "", unsupported("ordering booleans")
		}
		return "(" + b.jsonAttr(l.ref.name) + " " + sqlOp + " to_jsonb(" + b.param(v, "boolean") + "))", nil
	case int64, uint64, float64:
		key := b.keyParam(l.ref.name)
		return "(CASE WHEN jsonb_typeof(" + attributesColumn + " -> " + key + ") = 'number' THEN (" +
			attributesColumn + " ->> " + key + ")::numeric END " + sqlOp + " " + b.param(v, "numeric") + ")", nil
	case []any:
		if op != "==" && op != "!=" {
			return "", unsupported("ordering lists")
		}
		raw, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return "(" + b.jsonAttr(l.ref.name) + " " + sqlOp + " " + b.param(string(raw), "jsonb") + ")", nil
	}
	return "", unsupported("comparison with %T", value)
}

// rankOperand resolves a comparison operand of a ranked attribute to an
// option name and rank.
func rankOperand(value any, f *fieldInfo) (string, int64, bool) {
	switch v := value.(type) {
	case string:
		r, ok := f.ranks[v]
		return v, r, ok
	case map[string]any:
		name, _ := v["name"].(string)
		if r, ok := toInt64(v["rank"]); ok {
			return name, r, true
		}
		r, ok := f.ranks[name]
		return name, r, ok
	}
	return "", 0, false
}

func constantCompare(op string, l, r any) (string, error) {
	var result bool
	switch op {
	case "==":
		result = fmt.Sprint(l) == fmt.Sprint(r)
	case "!=":
		result = fmt.Sprint(l) != fmt.Sprint(r)
	default:
		ls, lok := l.(string)
		rs, rok := r.(string)
		if !lok || !rok {
			return "", unsupported("ordering constants")
		}
		switch op {
		case "<":
			result = ls < rs
		case "<=":
			result = ls <= rs
		case ">":
			result = ls > rs
		case ">=":
			result = ls >= rs
		}
	}
	if result {
		return "TRUE", nil
	}
	return "FALSE", nil
}

func toStrings(v any) ([]string, bool) {
	switch l := v.(type) {
	case []string:
		return l, true
	case []any:
		out := make([]string, 0, len(l))
		for _, e := range l {
			s, ok := e.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	case string:
		return []string{l}, true
	}
	return nil, false
}

func (b *sqlBuilder) in(left, right celast.Expr) (string, error) {
	l, err := b.operandOf(left)
	if err != nil {
		return "", err
	}
	r, err := b.operandOf(right)
	if err != nil {
		return "", err
	}

	// "value" in user.attributes.multi
	if r.isRef {
		if r.ref.scope == scopeNative {
			return "", unsupported("'in' on %s", r.ref.path())
		}
		v, ok := constantValue(l)
		if !ok {
			return sqlUnknown, nil
		}
		s, isString := v.(string)
		if !isString {
			return "", unsupported("membership test of a non-string value")
		}
		if r.field != nil && r.field.Type == model.PropertyFieldTypeGraph {
			ids, err := b.graph.optionIDsByName(r.field, []string{s})
			if err != nil {
				return "", err
			}
			id, found := ids[s]
			if !found {
				return "FALSE", nil
			}
			s = id
		}
		return "(" + b.jsonAttr(r.ref.name) + " @> jsonb_build_array(" + b.param(s, "text") + "))", nil
	}

	values, ok := constantValue(r)
	if !ok {
		return sqlUnknown, nil
	}
	list, isList := toStrings(values)
	if !isList {
		return "", unsupported("'in' expects a list of strings")
	}

	if !l.isRef {
		v, ok := constantValue(l)
		if !ok {
			return sqlUnknown, nil
		}
		s, _ := v.(string)
		if slices.Contains(list, s) {
			return "TRUE", nil
		}
		return "FALSE", nil
	}

	if l.ref.scope == scopeNative && b.excludeNative {
		return "TRUE", nil
	}
	if len(list) == 0 {
		return "FALSE", nil
	}

	var col string
	if l.ref.scope == scopeNative {
		if l.ref.name != nativeEmail && l.ref.name != nativeID {
			return "", unsupported("'in' on %s", l.ref.path())
		}
		col = nativeColumn(l.ref.name)
	} else {
		col = b.textAttr(l)
	}
	params := make([]string, 0, len(list))
	for _, s := range list {
		params = append(params, b.param(s, "text"))
	}
	return "(" + col + " IN (" + strings.Join(params, ", ") + "))", nil
}

func likeEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// arrayAny renders "the JSON array attribute holds any of values".
func (b *sqlBuilder) arrayAny(name string, values []string) string {
	if len(values) == 0 {
		return "FALSE"
	}
	params := make([]string, 0, len(values))
	for _, v := range values {
		params = append(params, b.param(v, "text"))
	}
	attr := b.jsonAttr(name)
	return "(CASE WHEN jsonb_typeof(" + attr + ") = 'array' THEN EXISTS (SELECT 1 FROM jsonb_array_elements_text(" +
		attr + ") AS mm_e(v) WHERE mm_e.v IN (" + strings.Join(params, ", ") + ")) END)"
}

func (b *sqlBuilder) method(fn string, target celast.Expr, args []celast.Expr) (string, error) {
	t, err := b.operandOf(target)
	if err != nil {
		return "", err
	}
	if len(args) != 1 {
		return "", unsupported("%s expects one argument", fn)
	}
	a, err := b.operandOf(args[0])
	if err != nil {
		return "", err
	}
	if !t.isRef {
		return "", unsupported("%s must be called on a user attribute", fn)
	}
	if t.ref.scope == scopeNative && b.excludeNative {
		return "TRUE", nil
	}

	value, ok := constantValue(a)
	if !ok {
		return sqlUnknown, nil
	}

	switch fn {
	case "startsWith", "endsWith", "contains":
		s, isString := value.(string)
		if !isString {
			return "", unsupported("%s expects a string", fn)
		}
		pattern := likeEscape(s)
		switch fn {
		case "startsWith":
			pattern += "%"
		case "endsWith":
			pattern = "%" + pattern
		default:
			pattern = "%" + pattern + "%"
		}
		var col string
		if t.ref.scope == scopeNative {
			if t.ref.name != nativeEmail && t.ref.name != nativeID {
				return "", unsupported("%s on %s", fn, t.ref.path())
			}
			col = nativeColumn(t.ref.name)
		} else {
			col = b.textAttr(t)
		}
		return "(" + col + " LIKE " + b.param(pattern, "text") + ")", nil

	case fnYoungerThanDays:
		if t.ref.scope != scopeNative || t.ref.name != nativeCreateAt {
			return "", unsupported("youngerThanDays is only supported on user.createat")
		}
		days, isInt := value.(int64)
		if !isInt || days < 0 {
			return "", unsupported("youngerThanDays expects a non-negative integer")
		}
		threshold := b.now.UnixMilli() - days*millisecondsPerDay
		return "(Users.CreateAt > 0 AND Users.CreateAt > " + b.param(threshold, "bigint") + ")", nil

	case fnHasAnyOf, fnHasAllOf:
		if t.ref.scope != scopeUser {
			return "", unsupported("%s on %s", fn, t.ref.path())
		}
		values, isList := toStrings(value)
		if !isList {
			return "", unsupported("%s expects a list of strings", fn)
		}
		if len(values) == 0 {
			return "FALSE", nil
		}
		if fn == fnHasAnyOf {
			return b.arrayAny(t.ref.name, values), nil
		}
		raw, err := json.Marshal(values)
		if err != nil {
			return "", err
		}
		return "(" + b.jsonAttr(t.ref.name) + " @> " + b.param(string(raw), "jsonb") + ")", nil

	case fnCoversAll, fnCoversAny, fnWithinAll, fnWithinAny:
		return b.graphPredicate(fn, t, a, value)

	case fnInCIDR, fnVersionEQ, fnVersionGT, fnVersionGTE, fnVersionLT, fnVersionLTE:
		return "", unsupported("%s can only be evaluated at request time", fn)
	}
	return "", unsupported("function %s", fn)
}

func (b *sqlBuilder) graphPredicate(fn string, t, a operand, value any) (string, error) {
	if t.ref.scope != scopeUser || t.field == nil || t.field.Type != model.PropertyFieldTypeGraph {
		return "", unsupported("%s requires a graph attribute", fn)
	}
	up := fn == fnCoversAll || fn == fnCoversAny
	all := fn == fnCoversAll || fn == fnWithinAll

	var targetIDs []string
	missing := false
	list, isList := toStrings(value)
	if !isList {
		return "", unsupported("%s expects a list of option names or a graph attribute", fn)
	}
	if a.isResource {
		// A channel graph attribute holds option IDs.
		targetIDs = list
	} else {
		idsByName, err := b.graph.optionIDsByName(t.field, list)
		if err != nil {
			return "", err
		}
		for _, n := range list {
			id, found := idsByName[n]
			if !found {
				missing = true
				continue
			}
			targetIDs = append(targetIDs, id)
		}
	}
	if len(targetIDs) == 0 || (all && missing) {
		return "FALSE", nil
	}

	var closure map[string][]string
	var err error
	if up {
		closure, err = b.graph.ancestorsOrSelf(t.field, targetIDs)
	} else {
		closure, err = b.graph.descendantsOrSelf(t.field, targetIDs)
	}
	if err != nil {
		return "", err
	}

	parts := make([]string, 0, len(targetIDs))
	for _, id := range targetIDs {
		parts = append(parts, b.arrayAny(t.ref.name, closure[id]))
	}
	joiner := " OR "
	if all {
		joiner = " AND "
	}
	return "(" + strings.Join(parts, joiner) + ")", nil
}
