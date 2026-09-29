// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package accesscontrol

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// parseExpression parses an expression without type checking.
func (s *Service) parseExpression(expression string) (*celast.AST, []model.CELExpressionError, error) {
	env, err := s.celEnv()
	if err != nil {
		return nil, nil, err
	}
	parsed, iss := env.Parse(expression)
	if iss != nil && iss.Err() != nil {
		return nil, issuesToErrors(iss), nil
	}
	return parsed.NativeRep(), nil, nil
}

func issuesToErrors(iss *cel.Issues) []model.CELExpressionError {
	out := []model.CELExpressionError{}
	for _, e := range iss.Errors() {
		line, col := 1, 0
		if e.Location != nil {
			line = e.Location.Line()
			col = e.Location.Column()
			if line < 1 {
				line = 1
			}
			if col < 0 {
				col = 0
			}
		}
		out = append(out, model.CELExpressionError{Line: line, Column: col, Message: e.Message})
	}
	return out
}

func errorAt(a *celast.AST, id int64, msg string) model.CELExpressionError {
	line, col := 1, 0
	if a != nil {
		loc := a.SourceInfo().GetStartLocation(id)
		if loc.Line() > 0 {
			line = loc.Line()
			col = loc.Column()
		}
	}
	return model.CELExpressionError{Line: line, Column: col, Message: msg}
}

// CheckExpression implements PolicyAdministrationPointInterface.
func (s *Service) CheckExpression(rctx request.CTX, expression string) ([]model.CELExpressionError, *model.AppError) {
	env, err := s.celEnv()
	if err != nil {
		return nil, model.NewAppError("CheckExpression", "app.pap.check_expression.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	if strings.TrimSpace(expression) == "" {
		return []model.CELExpressionError{{Line: 1, Column: 0, Message: "expression is empty"}}, nil
	}

	checked, iss := env.Compile(expression)
	if iss != nil && iss.Err() != nil {
		return issuesToErrors(iss), nil
	}
	nativeAST := checked.NativeRep()

	errs := []model.CELExpressionError{}
	if !checked.OutputType().IsAssignableType(cel.BoolType) {
		errs = append(errs, model.CELExpressionError{
			Line:    1,
			Column:  0,
			Message: fmt.Sprintf("expression must evaluate to a boolean, got %s", checked.OutputType().String()),
		})
	}

	cat, _ := s.catalogs.get(rctx)
	selectorErrs := s.checkSelectors(nativeAST, cat)
	if len(selectorErrs) > 0 && cat != nil {
		// The attribute may have been created after the catalog was cached.
		s.catalogs.invalidateAll()
		cat, _ = s.catalogs.get(rctx)
		selectorErrs = s.checkSelectors(nativeAST, cat)
	}
	errs = append(errs, selectorErrs...)
	return errs, nil
}

// checkSelectors reports selectors that reference unknown attributes or
// unknown user fields.
func (s *Service) checkSelectors(a *celast.AST, cat *catalog) []model.CELExpressionError {
	var errs []model.CELExpressionError
	walk(a.Expr(), func(n celast.Expr) bool {
		if r, ok := refOf(n); ok {
			switch r.scope {
			case scopeUser, scopeResource:
				if cat == nil {
					return false
				}
				if cat.lookupScope(r.scope, r.name) != nil {
					return false
				}
				if id, ok := fieldIDFromIdent(r.name); ok {
					if f := cat.lookupID(id); f != nil && f.ObjectType == objectTypeForScope(r.scope) {
						return false
					}
				}
				kind := "user"
				if r.scope == scopeResource {
					kind = "channel"
				}
				errs = append(errs, errorAt(a, selectorRootID(n), fmt.Sprintf("unknown %s attribute %q", kind, r.name)))
			}
			return false
		}

		if n.Kind() == celast.SelectKind {
			sel := n.AsSelect()
			op := sel.Operand()
			if op.Kind() == celast.IdentKind && !sel.IsTestOnly() {
				switch op.AsIdent() {
				case celVarUser:
					if sel.FieldName() != "attributes" && sel.FieldName() != "session" {
						errs = append(errs, errorAt(a, selectorRootID(n), fmt.Sprintf("unknown user field %q", sel.FieldName())))
					}
				case celVarResource:
					if sel.FieldName() != "attributes" && sel.FieldName() != "id" && sel.FieldName() != "type" {
						errs = append(errs, errorAt(a, selectorRootID(n), fmt.Sprintf("unknown resource field %q", sel.FieldName())))
					}
				}
			}
		}
		return true
	})
	return errs
}

// selectorRootID returns the ID of the identifier a selector chain starts
// with, whose location is the start of the selector in the source.
func selectorRootID(e celast.Expr) int64 {
	for e.Kind() == celast.SelectKind {
		e = e.AsSelect().Operand()
	}
	return e.ID()
}
