// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package accesscontrol

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	celast "github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/operators"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Visual operators beyond the comparison symbols.
const (
	opIn = "in"
)

// Condition source forms.
const (
	formInfix = ""      // attr op value, attr in [..]
	formChain = "chain" // "a" in attr && "b" in attr
	formOr    = "or"    // ("a" in attr || "b" in attr)
	formCall  = "call"  // attr.fn(arg)
)

var stringMethodOperators = []string{
	"startsWith", "endsWith", "contains",
	fnInCIDR, fnVersionEQ, fnVersionGT, fnVersionGTE, fnVersionLT, fnVersionLTE,
}

// condition is one row of the visual (table) form of an expression: a single
// test on one attribute. Conditions are implicitly AND-ed.
type condition struct {
	attr     attrRef
	operator string
	// values are the literal operands; for list operators all elements.
	values []any
	// listForm is true when the operand is a list (in, hasAnyOf, hasAllOf,
	// graph predicates).
	listForm bool
	// target is set when the operand is another attribute (value_type 1).
	target *attrRef
	// memberChain marks a hasAllOf built from `"v" in attr` conjuncts.
	memberChain bool
	// form records how the condition is written, so it can be printed back.
	form string
	// exprs are the top-level conjuncts this condition was built from.
	exprs []celast.Expr
}

// key identifies a condition for pairing a masked submission with the stored
// expression: the attribute and the shape of the test.
func (c *condition) key() string {
	return c.attr.path() + "|" + c.operator
}

// isPlaceholder reports whether the condition is the `attr in []` stand-in the
// editor submits for a row whose values are all hidden from the caller.
func (c *condition) isPlaceholder() bool {
	if c.operator != opIn || c.target != nil {
		return false
	}
	for _, v := range c.values {
		if s, ok := v.(string); !ok || s != model.MaskingTokenValue {
			return false
		}
	}
	return true
}

type unsupportedExpressionError struct{ reason string }

func (e *unsupportedExpressionError) Error() string {
	return "expression cannot be represented as simple conditions: " + e.reason
}

// memberTest recognizes `"value" in attr`.
func memberTest(e celast.Expr) (attrRef, string, bool) {
	call, ok := callOf(e, operators.In)
	if !ok || call.IsMemberFunction() || len(call.Args()) != 2 {
		return attrRef{}, "", false
	}
	v, ok := literalOf(call.Args()[0])
	if !ok {
		return attrRef{}, "", false
	}
	s, ok := v.(string)
	if !ok {
		return attrRef{}, "", false
	}
	r, ok := refOf(call.Args()[1])
	if !ok {
		return attrRef{}, "", false
	}
	return r, s, true
}

// extractConditions turns an expression into AND-ed conditions, or fails when
// the expression uses constructs the visual form cannot represent.
func extractConditions(root celast.Expr) ([]*condition, error) {
	if v, ok := literalOf(root); ok {
		if b, isBool := v.(bool); isBool && b {
			return []*condition{}, nil
		}
		return nil, &unsupportedExpressionError{"constant expression"}
	}

	var conds []*condition
	memberChains := map[attrRef]*condition{}

	for _, c := range flatten(root, operators.LogicalAnd) {
		// "v" in attr — part of a has-all-of chain on a multi-valued attribute.
		if r, v, ok := memberTest(c); ok {
			if existing, found := memberChains[r]; found {
				existing.values = append(existing.values, v)
				existing.exprs = append(existing.exprs, c)
				continue
			}
			cond := &condition{attr: r, operator: fnHasAllOf, values: []any{v}, listForm: true, memberChain: true, form: formChain, exprs: []celast.Expr{c}}
			memberChains[r] = cond
			conds = append(conds, cond)
			continue
		}

		// ("a" in attr || "b" in attr) — has any of.
		if parts := flatten(c, operators.LogicalOr); len(parts) > 1 {
			var attr attrRef
			values := make([]any, 0, len(parts))
			ok := true
			for i, p := range parts {
				r, v, isMember := memberTest(p)
				if !isMember || (i > 0 && r != attr) {
					ok = false
					break
				}
				attr = r
				values = append(values, v)
			}
			if !ok {
				return nil, &unsupportedExpressionError{"'||' is only supported between membership tests on the same attribute"}
			}
			conds = append(conds, &condition{attr: attr, operator: fnHasAnyOf, values: values, listForm: true, form: formOr, exprs: []celast.Expr{c}})
			continue
		}

		cond, err := extractCondition(c)
		if err != nil {
			return nil, err
		}
		cond.exprs = []celast.Expr{c}
		conds = append(conds, cond)
	}
	return conds, nil
}

func extractCondition(e celast.Expr) (*condition, error) {
	call, ok := callOf(e)
	if !ok {
		return nil, &unsupportedExpressionError{"expected a comparison or function call"}
	}
	fn := call.FunctionName()

	if !call.IsMemberFunction() {
		args := call.Args()
		if op, isCmp := comparisonOperators[fn]; isCmp && len(args) == 2 {
			left, right := args[0], args[1]
			lr, lok := refOf(left)
			rr, rok := refOf(right)
			switch {
			case lok && rok:
				if lr.scope == scopeResource && rr.scope != scopeResource {
					lr, rr = rr, lr
					op = flippedComparison[op]
				}
				target := rr
				return &condition{attr: lr, operator: op, target: &target}, nil
			case lok:
				v, isLit := literalOf(right)
				if !isLit {
					return nil, &unsupportedExpressionError{"comparison operand must be a literal"}
				}
				return &condition{attr: lr, operator: op, values: []any{v}}, nil
			case rok:
				v, isLit := literalOf(left)
				if !isLit {
					return nil, &unsupportedExpressionError{"comparison operand must be a literal"}
				}
				return &condition{attr: rr, operator: flippedComparison[op], values: []any{v}}, nil
			}
			return nil, &unsupportedExpressionError{"comparison must involve an attribute"}
		}

		if fn == operators.In && len(args) == 2 {
			r, isRef := refOf(args[0])
			if !isRef {
				return nil, &unsupportedExpressionError{"unsupported 'in' expression"}
			}
			list, isList := stringListOf(args[1])
			if !isList {
				return nil, &unsupportedExpressionError{"'in' expects a list of string literals"}
			}
			values := make([]any, 0, len(list))
			for _, v := range list {
				values = append(values, v)
			}
			return &condition{attr: r, operator: opIn, values: values, listForm: true}, nil
		}
		return nil, &unsupportedExpressionError{fmt.Sprintf("unsupported operator %s", fn)}
	}

	// Member calls: attr.fn(arg)
	r, isRef := refOf(call.Target())
	if !isRef {
		return nil, &unsupportedExpressionError{fmt.Sprintf("%s must be called on an attribute", fn)}
	}
	args := call.Args()
	if len(args) != 1 {
		return nil, &unsupportedExpressionError{fmt.Sprintf("%s expects one argument", fn)}
	}
	arg := args[0]

	switch {
	case slices.Contains(stringMethodOperators, fn):
		v, isLit := literalOf(arg)
		if !isLit {
			return nil, &unsupportedExpressionError{fmt.Sprintf("%s expects a literal argument", fn)}
		}
		return &condition{attr: r, operator: fn, values: []any{v}, form: formCall}, nil
	case fn == fnYoungerThanDays:
		v, isLit := literalOf(arg)
		if !isLit {
			return nil, &unsupportedExpressionError{"youngerThanDays expects an integer"}
		}
		return &condition{attr: r, operator: fn, values: []any{v}, form: formCall}, nil
	case fn == fnHasAnyOf || fn == fnHasAllOf || slices.Contains(graphFunctions, fn):
		if target, ok := refOf(arg); ok {
			t := target
			return &condition{attr: r, operator: fn, target: &t, form: formCall}, nil
		}
		list, isList := stringListOf(arg)
		if !isList {
			return nil, &unsupportedExpressionError{fmt.Sprintf("%s expects a list of string literals or an attribute", fn)}
		}
		values := make([]any, 0, len(list))
		for _, v := range list {
			values = append(values, v)
		}
		return &condition{attr: r, operator: fn, values: values, listForm: true, form: formCall}, nil
	}
	return nil, &unsupportedExpressionError{fmt.Sprintf("unsupported function %s", fn)}
}

// attributeType returns the type reported for a condition's attribute.
func attributeType(c *condition, cat *catalog) string {
	switch c.attr.scope {
	case scopeNative:
		return nativeAttributeTypes[c.attr.name]
	case scopeUser, scopeResource, scopeSession:
		if f := cat.lookupScope(c.attr.scope, c.attr.name); f != nil {
			return string(f.Type)
		}
		if id, ok := fieldIDFromIdent(c.attr.name); ok {
			if f := cat.lookupID(id); f != nil {
				return string(f.Type)
			}
		}
	}

	// Unknown field: infer from the shape of the test.
	switch {
	case slices.Contains(graphFunctions, c.operator):
		return string(model.PropertyFieldTypeGraph)
	case c.operator == fnHasAnyOf || c.operator == fnHasAllOf:
		return string(model.PropertyFieldTypeMultiselect)
	case c.operator == "<" || c.operator == "<=" || c.operator == ">" || c.operator == ">=":
		return string(model.PropertyFieldTypeRank)
	}
	return string(model.PropertyFieldTypeText)
}

// toVisual renders a condition for the webapp's table editor.
func (c *condition) toVisual(cat *catalog) model.Condition {
	vc := model.Condition{
		Attribute:     c.attr.path(),
		Operator:      c.operator,
		ValueType:     model.LiteralValue,
		AttributeType: attributeType(c, cat),
	}

	if c.target != nil {
		vc.Value = c.target.path()
		vc.ValueType = model.AttrValue
		return vc
	}

	values := make([]any, 0, len(c.values))
	for _, v := range c.values {
		if s, ok := v.(string); ok && s == model.MaskingTokenValue {
			vc.HasMaskedValues = true
			continue
		}
		values = append(values, v)
	}

	if c.listForm {
		vc.Value = values
		return vc
	}
	if len(values) == 0 {
		vc.Value = ""
		return vc
	}
	vc.Value = values[0]
	return vc
}

// ExpressionToVisualAST implements PolicyAdministrationPointInterface.
func (s *Service) ExpressionToVisualAST(rctx request.CTX, expression string) (*model.VisualExpression, *model.AppError) {
	cat, appErr := s.catalog(rctx)
	if appErr != nil {
		return nil, appErr
	}

	expression = strings.TrimSpace(idsToNames(expression, cat))
	if expression == "" {
		return &model.VisualExpression{Conditions: []model.Condition{}}, nil
	}

	parsed, celErrs, err := s.parseExpression(expression)
	if err != nil {
		return nil, model.NewAppError("ExpressionToVisualAST", "app.pap.expression_to_visual_ast.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if len(celErrs) > 0 {
		return nil, model.NewAppError("ExpressionToVisualAST", "app.pap.expression_to_visual_ast.app_error", nil, celErrs[0].Message, http.StatusBadRequest)
	}

	conds, err := extractConditions(parsed.Expr())
	if err != nil {
		return nil, model.NewAppError("ExpressionToVisualAST", "app.pap.expression_to_visual_ast.app_error", nil, err.Error(), http.StatusBadRequest)
	}

	out := &model.VisualExpression{Conditions: make([]model.Condition, 0, len(conds))}
	for _, c := range conds {
		out.Conditions = append(out.Conditions, c.toVisual(cat))
	}
	return out, nil
}
