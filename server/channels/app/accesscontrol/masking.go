// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package accesscontrol

import (
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"

	celast "github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/parser"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// literalTest is a set of literal operands tested against one attribute.
type literalTest struct {
	ref attrRef
	// literals are the string literal nodes compared with the attribute.
	literals []celast.Expr
	// list is set when the literals are the elements of a list literal.
	list celast.Expr
}

// maskingObjectType returns the CPA object type of a masked attribute scope,
// or "" when values of the scope are never masked.
func maskingObjectType(scope string) string {
	switch scope {
	case scopeUser:
		return model.PropertyFieldObjectTypeUser
	case scopeResource:
		return model.PropertyFieldObjectTypeChannel
	}
	return ""
}

func isStringLiteral(e celast.Expr) (string, bool) {
	v, ok := literalOf(e)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// literalTests finds every string literal tested against a maskable attribute.
func literalTests(root celast.Expr) []literalTest {
	var tests []literalTest
	operandTest := func(r attrRef, e celast.Expr) {
		if maskingObjectType(r.scope) == "" {
			return
		}
		if _, ok := isStringLiteral(e); ok {
			tests = append(tests, literalTest{ref: r, literals: []celast.Expr{e}})
			return
		}
		if e.Kind() == celast.ListKind {
			var lits []celast.Expr
			for _, el := range e.AsList().Elements() {
				if _, ok := isStringLiteral(el); ok {
					lits = append(lits, el)
				}
			}
			if len(lits) > 0 {
				tests = append(tests, literalTest{ref: r, literals: lits, list: e})
			}
		}
	}

	walk(root, func(n celast.Expr) bool {
		if n.Kind() != celast.CallKind {
			return true
		}
		call := n.AsCall()
		fn := call.FunctionName()
		if call.IsMemberFunction() {
			if r, ok := refOf(call.Target()); ok {
				for _, a := range call.Args() {
					operandTest(r, a)
				}
			}
			return true
		}
		args := call.Args()
		_, isCmp := comparisonOperators[fn]
		if (isCmp || fn == operators.In) && len(args) == 2 {
			if r, ok := refOf(args[0]); ok {
				operandTest(r, args[1])
			}
			if r, ok := refOf(args[1]); ok {
				operandTest(r, args[0])
			}
		}
		return true
	})
	return tests
}

// maskingParse parses an expression (field IDs are turned back into names)
// with macro call tracking so it can be printed back.
func (s *Service) maskingParse(rctx request.CTX, expression string) (*celast.AST, error) {
	if cat, err := s.catalogs.get(rctx); err == nil {
		expression = idsToNames(expression, cat)
	}
	parsed, celErrs, err := s.parseExpression(expression)
	if err != nil {
		return nil, err
	}
	if len(celErrs) > 0 {
		return nil, fmt.Errorf("invalid expression: %s", celErrs[0].Message)
	}
	return parsed, nil
}

// allRoots returns the expression root plus every tracked macro call, which
// hold their own copies of the macro arguments.
func allRoots(a *celast.AST) []celast.Expr {
	roots := []celast.Expr{a.Expr()}
	for _, m := range a.SourceInfo().MacroCalls() {
		roots = append(roots, m)
	}
	return roots
}

type maskInfoCache struct {
	resolver model.MaskingFieldResolver
	infos    map[string]*model.MaskingFieldInfo
	errs     map[string]error
}

func newMaskInfoCache(resolver model.MaskingFieldResolver) *maskInfoCache {
	return &maskInfoCache{resolver: resolver, infos: map[string]*model.MaskingFieldInfo{}, errs: map[string]error{}}
}

func (m *maskInfoCache) get(r attrRef) (*model.MaskingFieldInfo, error) {
	key := r.scope + "/" + r.name
	if info, ok := m.infos[key]; ok {
		return info, nil
	}
	if err, ok := m.errs[key]; ok {
		return nil, err
	}
	info, err := m.resolver.Resolve(maskingObjectType(r.scope), r.name)
	if err == nil && info == nil {
		err = fmt.Errorf("no visibility information for %s", r.path())
	}
	if err != nil {
		m.errs[key] = err
		return nil, err
	}
	m.infos[key] = info
	return info, nil
}

// hidden reports whether lit is hidden. On resolver errors it fails closed.
func (m *maskInfoCache) hidden(r attrRef, lit string) (bool, error) {
	if lit == model.MaskingTokenValue {
		return false, nil
	}
	info, err := m.get(r)
	if err != nil {
		return true, err
	}
	return info.IsValueHidden(lit), nil
}

// HasMaskedValuesForCaller implements PolicyAdministrationPointInterface.
func (s *Service) HasMaskedValuesForCaller(rctx request.CTX, expression string, resolver model.MaskingFieldResolver) (bool, *model.AppError) {
	if resolver == nil {
		return true, model.NewAppError("HasMaskedValuesForCaller", "app.pap.has_masked_values.app_error", nil, "resolver is nil", http.StatusInternalServerError)
	}
	if strings.TrimSpace(expression) == "" {
		return false, nil
	}
	parsed, err := s.maskingParse(rctx, expression)
	if err != nil {
		return true, model.NewAppError("HasMaskedValuesForCaller", "app.pap.has_masked_values.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}
	cache := newMaskInfoCache(resolver)
	for _, root := range allRoots(parsed) {
		for _, t := range literalTests(root) {
			for _, lit := range t.literals {
				str, _ := isStringLiteral(lit)
				hidden, err := cache.hidden(t.ref, str)
				if err != nil {
					return true, model.NewAppError("HasMaskedValuesForCaller", "app.pap.has_masked_values.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
				}
				if hidden {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func unparse(a *celast.AST) (string, error) {
	return parser.Unparse(a.Expr(), a.SourceInfo(), parser.WrapOnColumn(math.MaxInt32))
}

// MaskExpressionForCaller implements PolicyAdministrationPointInterface. Hidden
// literals become the masking token; several hidden elements of one list
// collapse into a single token so their number is not disclosed.
func (s *Service) MaskExpressionForCaller(rctx request.CTX, expression string, resolver model.MaskingFieldResolver) (string, bool, *model.AppError) {
	if resolver == nil {
		return "", false, model.NewAppError("MaskExpressionForCaller", "app.pap.mask_expression.app_error", nil, "resolver is nil", http.StatusInternalServerError)
	}
	if strings.TrimSpace(expression) == "" {
		return expression, false, nil
	}
	parsed, err := s.maskingParse(rctx, expression)
	if err != nil {
		return "", false, model.NewAppError("MaskExpressionForCaller", "app.pap.mask_expression.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}

	fac := celast.NewExprFactory()
	nextID := int64(1)
	for id := range parsed.SourceInfo().OffsetRanges() {
		if id >= nextID {
			nextID = id + 1
		}
	}
	nextID += 1_000_000

	cache := newMaskInfoCache(resolver)
	masked := false
	for _, root := range allRoots(parsed) {
		for _, t := range literalTests(root) {
			if t.list != nil {
				elems := t.list.AsList().Elements()
				kept := make([]celast.Expr, 0, len(elems))
				anyHidden := false
				for _, el := range elems {
					str, isStr := isStringLiteral(el)
					if isStr {
						// A resolver error hides the value (fail closed).
						if hidden, _ := cache.hidden(t.ref, str); hidden {
							anyHidden = true
							continue
						}
					}
					kept = append(kept, el)
				}
				if anyHidden {
					masked = true
					kept = append(kept, fac.NewLiteral(nextID, types.String(model.MaskingTokenValue)))
					nextID++
					t.list.SetKindCase(fac.NewList(t.list.ID(), kept, nil))
				}
				continue
			}
			for _, lit := range t.literals {
				str, _ := isStringLiteral(lit)
				if hidden, _ := cache.hidden(t.ref, str); hidden {
					masked = true
					lit.SetKindCase(fac.NewLiteral(lit.ID(), types.String(model.MaskingTokenValue)))
				}
			}
		}
	}

	if !masked {
		return expression, false, nil
	}
	out, err := unparse(parsed)
	if err != nil {
		return "", false, model.NewAppError("MaskExpressionForCaller", "app.pap.mask_expression.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return out, true, nil
}

// ValidateExpressionValuesForCaller implements PolicyAdministrationPointInterface.
func (s *Service) ValidateExpressionValuesForCaller(rctx request.CTX, expression string, resolver model.MaskingFieldResolver) *model.AppError {
	if resolver == nil {
		return model.NewAppError("ValidateExpressionValuesForCaller", "app.pap.save_policy.resolver_error", nil, "", http.StatusInternalServerError)
	}
	if strings.TrimSpace(expression) == "" {
		return nil
	}
	parsed, err := s.maskingParse(rctx, expression)
	if err != nil {
		return model.NewAppError("ValidateExpressionValuesForCaller", "app.pap.save_policy.invalid_value", nil, "", http.StatusBadRequest).Wrap(err)
	}
	cache := newMaskInfoCache(resolver)
	for _, root := range allRoots(parsed) {
		for _, t := range literalTests(root) {
			for _, lit := range t.literals {
				str, _ := isStringLiteral(lit)
				hidden, err := cache.hidden(t.ref, str)
				if err != nil {
					return model.NewAppError("ValidateExpressionValuesForCaller", "app.pap.save_policy.resolver_error", nil, "", http.StatusInternalServerError).Wrap(err)
				}
				if hidden {
					return model.NewAppError("ValidateExpressionValuesForCaller", "app.pap.save_policy.invalid_value", nil, "attribute="+t.ref.path(), http.StatusBadRequest)
				}
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Merge of a masked submission with the stored expression.
// ---------------------------------------------------------------------------

func mergeForbidden(rctx request.CTX, reason string) *model.AppError {
	if rctx != nil {
		rctx.Logger().Info("Refusing to merge masked policy expression", mlogReason(reason))
	}
	return model.NewAppError("MergeExpressionWithMaskedValuesCanonical", "app.pap.save_policy.forbidden", nil, "", http.StatusForbidden)
}

// MergeExpressionWithMaskedValuesCanonical implements
// PolicyAdministrationPointInterface. Both expressions are read as AND-ed
// conditions; every stored condition holding values hidden from the caller must
// be paired with a submitted condition on the same attribute and of the same
// kind (or the `attr in []` placeholder), into which the hidden values are
// re-injected.
func (s *Service) MergeExpressionWithMaskedValuesCanonical(rctx request.CTX, submittedExpr, storedExpr string, resolver model.MaskingFieldResolver) (string, *model.AppError) {
	const where = "MergeExpressionWithMaskedValuesCanonical"
	if resolver == nil {
		return "", model.NewAppError(where, "app.pap.merge_expression.app_error", nil, "resolver is nil", http.StatusInternalServerError)
	}
	if strings.TrimSpace(storedExpr) == "" {
		return submittedExpr, nil
	}

	stored, err := s.maskingParse(rctx, storedExpr)
	if err != nil {
		return "", model.NewAppError(where, "app.pap.merge_expression.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	cache := newMaskInfoCache(resolver)
	anyHidden := false
	for _, root := range allRoots(stored) {
		for _, t := range literalTests(root) {
			for _, lit := range t.literals {
				str, _ := isStringLiteral(lit)
				if hidden, _ := cache.hidden(t.ref, str); hidden {
					anyHidden = true
				}
			}
		}
	}
	if !anyHidden {
		return submittedExpr, nil
	}

	if strings.TrimSpace(submittedExpr) == "" {
		return "", mergeForbidden(rctx, "masked_condition_deleted: submitted expression is empty")
	}
	submitted, err := s.maskingParse(rctx, submittedExpr)
	if err != nil {
		return "", model.NewAppError(where, "app.pap.merge_expression.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}

	storedConds, sErr := extractConditions(stored.Expr())
	submittedConds, subErr := extractConditions(submitted.Expr())
	if sErr != nil || subErr != nil {
		// Not representable as conditions: only an unchanged round trip of
		// the masked form can be accepted.
		maskedStored, _, appErr := s.MaskExpressionForCaller(rctx, storedExpr, resolver)
		if appErr != nil {
			return "", appErr
		}
		canonicalSubmitted, uErr := unparse(submitted)
		if uErr == nil {
			if again, pErr := s.maskingParse(rctx, maskedStored); pErr == nil {
				if canonicalStored, uErr2 := unparse(again); uErr2 == nil && canonicalStored == canonicalSubmitted {
					return storedExpr, nil
				}
			}
		}
		return "", mergeForbidden(rctx, "shape_diverged: expression with hidden values is not a simple condition list")
	}

	paired := make([]bool, len(submittedConds))
	replacement := make(map[int]*condition)

	for _, sc := range storedConds {
		var hiddenVals []any
		for _, v := range sc.values {
			str, ok := v.(string)
			if !ok {
				continue
			}
			if hidden, _ := cache.hidden(sc.attr, str); hidden && maskingObjectType(sc.attr.scope) != "" {
				hiddenVals = append(hiddenVals, str)
			}
		}
		if len(hiddenVals) == 0 {
			continue
		}

		idx := -1
		for i, c := range submittedConds {
			if !paired[i] && c.key() == sc.key() {
				idx = i
				break
			}
		}
		if idx < 0 {
			for i, c := range submittedConds {
				if !paired[i] && c.attr == sc.attr && c.isPlaceholder() {
					idx = i
					break
				}
			}
		}
		if idx < 0 {
			return "", model.NewAppError(where, "app.pap.save_policy.masked_condition_deleted", nil, "", http.StatusForbidden)
		}
		paired[idx] = true
		sub := submittedConds[idx]

		merged := &condition{attr: sc.attr, operator: sc.operator, listForm: sc.listForm, memberChain: sc.memberChain, form: sc.form, target: sc.target}
		if !sub.isPlaceholder() {
			merged.operator = sub.operator
			merged.listForm = sub.listForm
			merged.memberChain = sub.memberChain
			merged.form = sub.form
		}

		if merged.listForm {
			for _, v := range sub.values {
				if str, ok := v.(string); ok && str == model.MaskingTokenValue {
					continue
				}
				merged.values = append(merged.values, v)
			}
			for _, h := range hiddenVals {
				if !slices.Contains(merged.values, h) {
					merged.values = append(merged.values, h)
				}
			}
		} else {
			if len(sub.values) != 1 {
				return "", mergeForbidden(rctx, "shape_diverged: single-valued condition")
			}
			if str, ok := sub.values[0].(string); !ok || str != model.MaskingTokenValue {
				// The caller replaced a value they could not see.
				return "", model.NewAppError(where, "app.pap.save_policy.masked_condition_deleted", nil, "", http.StatusForbidden)
			}
			merged.values = []any{hiddenVals[0]}
		}
		replacement[idx] = merged
	}

	parts := make([]string, 0, len(submittedConds))
	for i, c := range submittedConds {
		if r, ok := replacement[i]; ok {
			c = r
		}
		text, err := renderCondition(c)
		if err != nil {
			return "", model.NewAppError(where, "app.pap.merge_expression.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		parts = append(parts, text)
	}
	if len(parts) == 0 {
		return "true", nil
	}
	return strings.Join(parts, " && "), nil
}

// renderLiteral prints a literal in CEL syntax.
func renderLiteral(v any) (string, error) {
	switch l := v.(type) {
	case string:
		return strconv.Quote(l), nil
	case bool:
		return strconv.FormatBool(l), nil
	case int64:
		return strconv.FormatInt(l, 10), nil
	case uint64:
		return strconv.FormatUint(l, 10) + "u", nil
	case float64:
		str := strconv.FormatFloat(l, 'g', -1, 64)
		if !strings.ContainsAny(str, ".eE") {
			str += ".0"
		}
		return str, nil
	case nil:
		return "null", nil
	}
	return "", fmt.Errorf("unsupported literal %T", v)
}

func renderList(values []any) (string, error) {
	parts := make([]string, 0, len(values))
	for _, v := range values {
		s, err := renderLiteral(v)
		if err != nil {
			return "", err
		}
		parts = append(parts, s)
	}
	return "[" + strings.Join(parts, ", ") + "]", nil
}

// renderCondition prints a condition back as CEL.
func renderCondition(c *condition) (string, error) {
	path := c.attr.path()
	if c.target != nil {
		if _, isCmp := flippedComparison[c.operator]; isCmp {
			return path + " " + c.operator + " " + c.target.path(), nil
		}
		return path + "." + c.operator + "(" + c.target.path() + ")", nil
	}

	switch {
	case c.form == formChain:
		parts := make([]string, 0, len(c.values))
		for _, v := range c.values {
			lit, err := renderLiteral(v)
			if err != nil {
				return "", err
			}
			parts = append(parts, lit+" in "+path)
		}
		if len(parts) == 0 {
			return path + " in []", nil
		}
		return strings.Join(parts, " && "), nil
	case c.form == formOr:
		parts := make([]string, 0, len(c.values))
		for _, v := range c.values {
			lit, err := renderLiteral(v)
			if err != nil {
				return "", err
			}
			parts = append(parts, lit+" in "+path)
		}
		if len(parts) == 0 {
			return path + " in []", nil
		}
		if len(parts) == 1 {
			return parts[0], nil
		}
		return "(" + strings.Join(parts, " || ") + ")", nil
	case c.operator == opIn:
		list, err := renderList(c.values)
		if err != nil {
			return "", err
		}
		return path + " in " + list, nil
	case c.form == formCall:
		if c.listForm {
			list, err := renderList(c.values)
			if err != nil {
				return "", err
			}
			return path + "." + c.operator + "(" + list + ")", nil
		}
		if len(c.values) != 1 {
			return "", fmt.Errorf("%s expects one argument", c.operator)
		}
		lit, err := renderLiteral(c.values[0])
		if err != nil {
			return "", err
		}
		return path + "." + c.operator + "(" + lit + ")", nil
	}

	if len(c.values) != 1 {
		return "", fmt.Errorf("%s expects one value", c.operator)
	}
	lit, err := renderLiteral(c.values[0])
	if err != nil {
		return "", err
	}
	return path + " " + c.operator + " " + lit, nil
}

func mlogReason(reason string) mlog.Field {
	return mlog.String("internal_reason", reason)
}
