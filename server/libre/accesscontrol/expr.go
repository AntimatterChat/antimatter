// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package accesscontrol

import (
	"strings"

	celast "github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"

	"github.com/mattermost/mattermost/server/public/model"
)

// Native user attributes exposed as user.<name>.
const (
	nativeID       = "id"
	nativeEmail    = model.NativeAttributePropertyFieldEmail
	nativeVerified = model.NativeAttributePropertyFieldVerified
	nativeIsBot    = model.NativeAttributePropertyFieldIsBot
	nativeCreateAt = model.NativeAttributePropertyFieldCreateAt
)

var nativeAttributeTypes = map[string]string{
	nativeID:       string(model.PropertyFieldTypeText),
	nativeEmail:    string(model.PropertyFieldTypeText),
	nativeVerified: string(model.PropertyFieldTypeSelect),
	nativeIsBot:    string(model.PropertyFieldTypeSelect),
	nativeCreateAt: string(model.PropertyFieldTypeText),
}

// attrRef identifies an attribute selector in an expression.
type attrRef struct {
	scope string // scopeUser, scopeResource, scopeSession, scopeNative
	name  string
}

// path renders the selector as CEL.
func (a attrRef) path() string {
	switch a.scope {
	case scopeUser:
		return "user.attributes." + a.name
	case scopeResource:
		return "resource.attributes." + a.name
	case scopeSession:
		return "user.session." + a.name
	case scopeNative:
		return "user." + a.name
	}
	return ""
}

// refOf recognizes the attribute selectors policies are written with:
// user.attributes.X, resource.attributes.X, user.session.X and user.<native>.
func refOf(e celast.Expr) (attrRef, bool) {
	if e == nil || e.Kind() != celast.SelectKind {
		return attrRef{}, false
	}
	sel := e.AsSelect()
	if sel.IsTestOnly() {
		return attrRef{}, false
	}
	name := sel.FieldName()
	operand := sel.Operand()

	switch operand.Kind() {
	case celast.IdentKind:
		if operand.AsIdent() == celVarUser {
			if _, ok := nativeAttributeTypes[name]; ok {
				return attrRef{scope: scopeNative, name: name}, true
			}
		}
	case celast.SelectKind:
		inner := operand.AsSelect()
		if inner.IsTestOnly() || inner.Operand().Kind() != celast.IdentKind {
			return attrRef{}, false
		}
		root := inner.Operand().AsIdent()
		switch {
		case root == celVarUser && inner.FieldName() == "attributes":
			return attrRef{scope: scopeUser, name: name}, true
		case root == celVarUser && inner.FieldName() == "session":
			return attrRef{scope: scopeSession, name: name}, true
		case root == celVarResource && inner.FieldName() == "attributes":
			return attrRef{scope: scopeResource, name: name}, true
		}
	}
	return attrRef{}, false
}

// literalOf returns the Go value of a scalar literal.
func literalOf(e celast.Expr) (any, bool) {
	if e == nil || e.Kind() != celast.LiteralKind {
		return nil, false
	}
	return literalGo(e.AsLiteral())
}

func literalGo(v ref.Val) (any, bool) {
	switch l := v.(type) {
	case types.String:
		return string(l), true
	case types.Bool:
		return bool(l), true
	case types.Int:
		return int64(l), true
	case types.Uint:
		return uint64(l), true
	case types.Double:
		return float64(l), true
	case types.Null:
		return nil, true
	}
	return nil, false
}

// stringListOf returns the elements of a list literal made only of string literals.
func stringListOf(e celast.Expr) ([]string, bool) {
	if e == nil || e.Kind() != celast.ListKind {
		return nil, false
	}
	elems := e.AsList().Elements()
	out := make([]string, 0, len(elems))
	for _, el := range elems {
		v, ok := literalOf(el)
		if !ok {
			return nil, false
		}
		s, ok := v.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// callOf returns the call expression and whether e is a call to one of fns.
func callOf(e celast.Expr, fns ...string) (celast.CallExpr, bool) {
	if e == nil || e.Kind() != celast.CallKind {
		return nil, false
	}
	call := e.AsCall()
	if len(fns) == 0 {
		return call, true
	}
	for _, fn := range fns {
		if call.FunctionName() == fn {
			return call, true
		}
	}
	return nil, false
}

// flatten splits a tree of the given binary logical operator into its operands.
func flatten(e celast.Expr, op string) []celast.Expr {
	call, ok := callOf(e, op)
	if !ok || call.IsMemberFunction() {
		return []celast.Expr{e}
	}
	var out []celast.Expr
	for _, arg := range call.Args() {
		out = append(out, flatten(arg, op)...)
	}
	return out
}

// comparisonOperators maps CEL operator function names to their symbols.
var comparisonOperators = map[string]string{
	operators.Equals:        "==",
	operators.NotEquals:     "!=",
	operators.Less:          "<",
	operators.LessEquals:    "<=",
	operators.Greater:       ">",
	operators.GreaterEquals: ">=",
}

// flippedComparison is the operator to use when swapping the operands.
var flippedComparison = map[string]string{
	"==": "==",
	"!=": "!=",
	"<":  ">",
	"<=": ">=",
	">":  "<",
	">=": "<=",
}

// walk visits e and all of its descendants, pre-order.
func walk(e celast.Expr, visit func(celast.Expr) bool) {
	if e == nil {
		return
	}
	if !visit(e) {
		return
	}
	switch e.Kind() {
	case celast.CallKind:
		call := e.AsCall()
		if call.IsMemberFunction() {
			walk(call.Target(), visit)
		}
		for _, a := range call.Args() {
			walk(a, visit)
		}
	case celast.SelectKind:
		walk(e.AsSelect().Operand(), visit)
	case celast.ListKind:
		for _, el := range e.AsList().Elements() {
			walk(el, visit)
		}
	case celast.MapKind:
		for _, entry := range e.AsMap().Entries() {
			me := entry.AsMapEntry()
			walk(me.Key(), visit)
			walk(me.Value(), visit)
		}
	case celast.StructKind:
		for _, entry := range e.AsStruct().Fields() {
			walk(entry.AsStructField().Value(), visit)
		}
	case celast.ComprehensionKind:
		comp := e.AsComprehension()
		walk(comp.IterRange(), visit)
		walk(comp.AccuInit(), visit)
		walk(comp.LoopCondition(), visit)
		walk(comp.LoopStep(), visit)
		walk(comp.Result(), visit)
	}
}

// referencedAttributes returns every attribute selector in the expression.
func referencedAttributes(e celast.Expr) []attrRef {
	var refs []attrRef
	seen := map[attrRef]bool{}
	walk(e, func(n celast.Expr) bool {
		if r, ok := refOf(n); ok {
			if !seen[r] {
				seen[r] = true
				refs = append(refs, r)
			}
			return false
		}
		return true
	})
	return refs
}

// referencesScope reports whether the expression reads any attribute of scope.
func referencesScope(e celast.Expr, scope string) bool {
	for _, r := range referencedAttributes(e) {
		if r.scope == scope {
			return true
		}
	}
	return false
}

// referencesVariable reports whether the expression mentions the identifier at all.
func referencesVariable(e celast.Expr, ident string) bool {
	found := false
	walk(e, func(n celast.Expr) bool {
		if found {
			return false
		}
		if n.Kind() == celast.IdentKind && n.AsIdent() == ident {
			found = true
			return false
		}
		return true
	})
	return found
}

// ---------------------------------------------------------------------------
// Text-level rewriting of attribute selectors.
//
// Stored policies reference custom attributes by field ID
// (user.attributes.id_<fieldID>) so that renaming an attribute does not break
// them; editors and evaluation use names. The rewrite works on the source text
// so formatting is preserved, and skips string literals.
// ---------------------------------------------------------------------------

const fieldIDPrefix = "id_"

func isIdentStart(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isIdentPart(r rune) bool {
	return isIdentStart(r) || (r >= '0' && r <= '9')
}

// skipStringLiteral returns the index just past the string literal starting at
// i (which must point at a quote or at a raw/bytes prefix followed by a quote).
func skipStringLiteral(src []rune, i int) int {
	raw := false
	for i < len(src) && (src[i] == 'r' || src[i] == 'R' || src[i] == 'b' || src[i] == 'B') {
		if src[i] == 'r' || src[i] == 'R' {
			raw = true
		}
		i++
	}
	if i >= len(src) {
		return i
	}
	quote := src[i]
	triple := i+2 < len(src) && src[i+1] == quote && src[i+2] == quote
	if triple {
		i += 3
		for i < len(src) {
			if !raw && src[i] == '\\' {
				i += 2
				continue
			}
			if i+2 < len(src) && src[i] == quote && src[i+1] == quote && src[i+2] == quote {
				return i + 3
			}
			i++
		}
		return i
	}
	i++
	for i < len(src) {
		if !raw && src[i] == '\\' {
			i += 2
			continue
		}
		if src[i] == quote {
			return i + 1
		}
		if src[i] == '\n' {
			return i
		}
		i++
	}
	return i
}

// isStringLiteralStart reports whether a string literal starts at i.
func isStringLiteralStart(src []rune, i int) bool {
	if src[i] == '"' || src[i] == '\'' {
		return true
	}
	// Prefixed literals: r"...", b'...', rb"..." etc. Only when not part of an identifier.
	if i > 0 && (isIdentPart(src[i-1]) || src[i-1] == '.') {
		return false
	}
	j := i
	for j < len(src) && j < i+2 && (src[j] == 'r' || src[j] == 'R' || src[j] == 'b' || src[j] == 'B') {
		j++
	}
	return j > i && j < len(src) && (src[j] == '"' || src[j] == '\'')
}

// rewriteAttributeSelectors calls fn for every `user.attributes.<ident>` and
// `resource.attributes.<ident>` selector outside string literals and replaces
// the identifier with its result.
func rewriteAttributeSelectors(expression string, fn func(scope, ident string) string) string {
	if !strings.Contains(expression, ".attributes.") {
		return expression
	}
	src := []rune(expression)
	var b strings.Builder
	b.Grow(len(expression))

	prefixes := []struct {
		text  []rune
		scope string
	}{
		{[]rune("user.attributes."), scopeUser},
		{[]rune("resource.attributes."), scopeResource},
	}

	i := 0
	for i < len(src) {
		if isStringLiteralStart(src, i) {
			end := skipStringLiteral(src, i)
			b.WriteString(string(src[i:end]))
			i = end
			continue
		}

		matched := false
		if i == 0 || (!isIdentPart(src[i-1]) && src[i-1] != '.') {
			for _, p := range prefixes {
				if hasRunePrefix(src[i:], p.text) {
					start := i + len(p.text)
					end := start
					if end < len(src) && isIdentStart(src[end]) {
						end++
						for end < len(src) && isIdentPart(src[end]) {
							end++
						}
					}
					if end > start {
						b.WriteString(string(p.text))
						b.WriteString(fn(p.scope, string(src[start:end])))
						i = end
						matched = true
					}
					break
				}
			}
		}
		if matched {
			continue
		}

		// Copy a whole identifier at once so that e.g. "xuser.attributes" is
		// never matched mid-identifier.
		if isIdentStart(src[i]) {
			end := i + 1
			for end < len(src) && isIdentPart(src[end]) {
				end++
			}
			b.WriteString(string(src[i:end]))
			i = end
			continue
		}
		b.WriteRune(src[i])
		i++
	}
	return b.String()
}

func hasRunePrefix(s, prefix []rune) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := range prefix {
		if s[i] != prefix[i] {
			return false
		}
	}
	return true
}

// fieldIDFromIdent extracts the field ID from an `id_<fieldID>` identifier.
func fieldIDFromIdent(ident string) (string, bool) {
	if !strings.HasPrefix(ident, fieldIDPrefix) {
		return "", false
	}
	id := strings.TrimPrefix(ident, fieldIDPrefix)
	if !model.IsValidId(id) {
		return "", false
	}
	return id, true
}

// idsToNames rewrites id_<fieldID> selectors back to attribute names.
func idsToNames(expression string, cat *catalog) string {
	return rewriteAttributeSelectors(expression, func(scope, ident string) string {
		id, ok := fieldIDFromIdent(ident)
		if !ok {
			return ident
		}
		// A field literally named id_<something> keeps its name.
		if cat.lookupScope(scope, ident) != nil {
			return ident
		}
		if f := cat.lookupID(id); f != nil && f.ObjectType == objectTypeForScope(scope) {
			return f.Name
		}
		return ident
	})
}

// namesToIDs rewrites attribute names to id_<fieldID> selectors. Unknown
// names are reported through the unknown callback and left untouched.
func namesToIDs(expression string, cat *catalog, unknown func(scope, name string)) string {
	return rewriteAttributeSelectors(expression, func(scope, ident string) string {
		if f := cat.lookupScope(scope, ident); f != nil {
			return fieldIDPrefix + f.ID
		}
		if id, ok := fieldIDFromIdent(ident); ok {
			if f := cat.lookupID(id); f != nil && f.ObjectType == objectTypeForScope(scope) {
				return ident
			}
		}
		if unknown != nil {
			unknown(scope, ident)
		}
		return ident
	})
}
