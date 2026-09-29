// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package accesscontrol

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestCheckExpression(t *testing.T) {
	e := newTestEnv(t)

	t.Run("valid", func(t *testing.T) {
		errs, appErr := e.svc.CheckExpression(e.rctx, `user.attributes.department == "Engineering" && user.email.endsWith("@example.com")`)
		require.Nil(t, appErr)
		assert.Empty(t, errs)
	})

	t.Run("syntax error has a position", func(t *testing.T) {
		errs, appErr := e.svc.CheckExpression(e.rctx, "user.attributes.department ==")
		require.Nil(t, appErr)
		require.NotEmpty(t, errs)
		assert.Equal(t, 1, errs[0].Line)
		assert.Greater(t, errs[0].Column, 0)
	})

	t.Run("unknown attribute is reported where it is", func(t *testing.T) {
		errs, appErr := e.svc.CheckExpression(e.rctx, "user.attributes.department == \"x\" &&\n  user.attributes.nope == \"y\"")
		require.Nil(t, appErr)
		require.Len(t, errs, 1)
		assert.Equal(t, 2, errs[0].Line)
		assert.Equal(t, 2, errs[0].Column)
		assert.Contains(t, errs[0].Message, "nope")
	})

	t.Run("non boolean", func(t *testing.T) {
		errs, appErr := e.svc.CheckExpression(e.rctx, `"abc"`)
		require.Nil(t, appErr)
		require.NotEmpty(t, errs)
	})

	t.Run("unknown function", func(t *testing.T) {
		errs, appErr := e.svc.CheckExpression(e.rctx, `user.attributes.department.frobnicate("x")`)
		require.Nil(t, appErr)
		require.NotEmpty(t, errs)
	})

	t.Run("custom functions type check", func(t *testing.T) {
		for _, expr := range []string{
			`user.createat.youngerThanDays(30)`,
			`user.attributes.skills.hasAnyOf(["Go", "SQL"])`,
			`user.attributes.programs.coversAll(["Air"])`,
			`user.session.ip.inCIDR("10.0.0.0/8")`,
			`user.session.client_version.versionGTE("6.0.0")`,
			`user.attributes.clearance >= resource.attributes.minClearance`,
		} {
			errs, appErr := e.svc.CheckExpression(e.rctx, expr)
			require.Nil(t, appErr)
			assert.Empty(t, errs, expr)
		}
	})
}

func TestIDRewrite(t *testing.T) {
	e := newTestEnv(t)
	cat, appErr := e.svc.catalog(e.rctx)
	require.Nil(t, appErr)

	src := `user.attributes.department == "user.attributes.department" && resource.attributes.department in ['a'] && xuser.attributes.department == "b"`
	ids := namesToIDs(src, cat, nil)
	assert.Equal(t, `user.attributes.id_`+fieldDepartment+` == "user.attributes.department" && resource.attributes.id_`+fieldChanDept+` in ['a'] && xuser.attributes.department == "b"`, ids)
	assert.Equal(t, src, idsToNames(ids, cat))

	var unknown []string
	namesToIDs(`user.attributes.missing == "x"`, cat, func(scope, name string) { unknown = append(unknown, name) })
	assert.Equal(t, []string{"missing"}, unknown)
}

func evalFor(t *testing.T, e *testEnv, subject model.Subject, resourceID, expr string) (bool, error) {
	t.Helper()
	cat, appErr := e.svc.catalog(e.rctx)
	require.Nil(t, appErr)
	ec := e.svc.newEvalContext(e.rctx, cat, &subject, model.Resource{ID: resourceID, Type: model.AccessControlPolicyTypeChannel})
	return ec.eval(expr)
}

func TestEvaluation(t *testing.T) {
	e := newTestEnv(t)
	channelID := model.NewId()
	e.channelAttrs[channelID] = map[string]any{
		"minClearance":    map[string]any{"name": "Secret", "rank": float64(2)},
		"channelPrograms": []any{optAir},
		"department":      "Engineering",
	}

	subject := subjectWith(map[string]any{
		"department": "Engineering",
		"clearance":  map[string]any{"name": "Secret", "rank": float64(2)},
		"skills":     []any{"Go", "SQL"},
		"programs":   []any{optAir},
	})
	subject.Email = "jane@example.com"
	subject.EmailVerified = true
	subject.CreateAt = e.svc.now().UnixMilli() - 5*millisecondsPerDay
	subject.Session = map[string]any{"ip": "10.1.2.3", "client_version": "6.2.1"}

	cases := []struct {
		expr string
		want bool
		err  bool
	}{
		{`user.attributes.department == "Engineering"`, true, false},
		{`user.attributes.department != "Engineering"`, false, false},
		{`user.attributes.department in ["Sales", "Engineering"]`, true, false},
		{`user.attributes.department.startsWith("Eng")`, true, false},
		{`user.attributes.nothere == "x"`, false, true},
		{`user.attributes.nothere == "x" || true`, true, false},
		{`user.attributes.clearance == "Secret"`, true, false},
		{`user.attributes.clearance >= "Secret"`, true, false},
		{`user.attributes.clearance > "Secret"`, false, false},
		{`user.attributes.clearance < "TopSecret"`, true, false},
		{`user.attributes.clearance >= "Unknown"`, false, true},
		{`user.attributes.clearance >= resource.attributes.minClearance`, true, false},
		{`"Go" in user.attributes.skills && "SQL" in user.attributes.skills`, true, false},
		{`("Rust" in user.attributes.skills || "Go" in user.attributes.skills)`, true, false},
		{`user.attributes.skills.hasAllOf(["Go", "Rust"])`, false, false},
		{`user.attributes.skills.hasAnyOf(["Go", "Rust"])`, true, false},
		{`user.attributes.skills.hasAnyOf([])`, false, false},
		{`"Air" in user.attributes.programs`, true, false},
		{`"F-18 Program" in user.attributes.programs`, false, false},
		{`user.attributes.programs.coversAll(["F-18 Program"])`, true, false},
		{`user.attributes.programs.coversAll(["F-18 Program", "Sea"])`, false, false},
		{`user.attributes.programs.coversAny(["F-18 Program", "Sea"])`, true, false},
		{`user.attributes.programs.withinAll(["All Programs"])`, true, false},
		{`user.attributes.programs.withinAny(["F-18 Program"])`, false, false},
		{`user.attributes.programs.coversAll(resource.attributes.channelPrograms)`, true, false},
		{`user.attributes.programs.coversAll(["Nope"])`, false, false},
		{`user.attributes.department == resource.attributes.department`, true, false},
		{`user.email.endsWith("@example.com") && user.verified == true && user.isbot == false`, true, false},
		{`user.createat.youngerThanDays(7)`, true, false},
		{`user.createat.youngerThanDays(3)`, false, false},
		{`user.session.ip.inCIDR("10.0.0.0/8")`, true, false},
		{`user.session.ip.inCIDR("192.168.0.0/16")`, false, false},
		{`user.session.client_version.versionGTE("6.0.0")`, true, false},
		{`user.session.client_version.versionLT("6.0.0")`, false, false},
	}
	for _, tc := range cases {
		got, err := evalFor(t, e, subject, channelID, tc.expr)
		if tc.err {
			assert.Error(t, err, tc.expr)
			continue
		}
		require.NoError(t, err, tc.expr)
		assert.Equal(t, tc.want, got, tc.expr)
	}
}

func TestExpressionToVisualAST(t *testing.T) {
	e := newTestEnv(t)

	t.Run("table editor shapes", func(t *testing.T) {
		expr := strings.Join([]string{
			`user.attributes.department == "Engineering"`,
			`user.attributes.department in ["US", "CA"]`,
			`"Go" in user.attributes.skills`,
			`"SQL" in user.attributes.skills`,
			`("A" in user.attributes.skills || "B" in user.attributes.skills)`,
			`user.attributes.clearance >= resource.attributes.minClearance`,
			`user.attributes.programs.coversAll(["F-18 Program"])`,
			`user.verified == true`,
			`user.createat.youngerThanDays(30)`,
			`user.email.startsWith("a")`,
		}, " && ")
		ast, appErr := e.svc.ExpressionToVisualAST(e.rctx, expr)
		require.Nil(t, appErr)
		require.Len(t, ast.Conditions, 9)

		c := ast.Conditions
		assert.Equal(t, model.Condition{Attribute: "user.attributes.department", Operator: "==", Value: "Engineering", AttributeType: "text"}, c[0])
		assert.Equal(t, "in", c[1].Operator)
		assert.Equal(t, []any{"US", "CA"}, c[1].Value)
		assert.Equal(t, "hasAllOf", c[2].Operator)
		assert.Equal(t, []any{"Go", "SQL"}, c[2].Value)
		assert.Equal(t, "multiselect", c[2].AttributeType)
		assert.Equal(t, "hasAnyOf", c[3].Operator)
		assert.Equal(t, []any{"A", "B"}, c[3].Value)
		assert.Equal(t, model.Condition{Attribute: "user.attributes.clearance", Operator: ">=", Value: "resource.attributes.minClearance", ValueType: model.AttrValue, AttributeType: "rank"}, c[4])
		assert.Equal(t, model.Condition{Attribute: "user.attributes.programs", Operator: "coversAll", Value: []any{"F-18 Program"}, AttributeType: "graph"}, c[5])
		assert.Equal(t, model.Condition{Attribute: "user.verified", Operator: "==", Value: true, AttributeType: "select"}, c[6])
		assert.Equal(t, model.Condition{Attribute: "user.createat", Operator: "youngerThanDays", Value: int64(30), AttributeType: "text"}, c[7])
		assert.Equal(t, "startsWith", c[8].Operator)
	})

	t.Run("stored IDs are shown as names", func(t *testing.T) {
		ast, appErr := e.svc.ExpressionToVisualAST(e.rctx, `user.attributes.id_`+fieldDepartment+` == "x"`)
		require.Nil(t, appErr)
		require.Len(t, ast.Conditions, 1)
		assert.Equal(t, "user.attributes.department", ast.Conditions[0].Attribute)
	})

	t.Run("masked placeholder", func(t *testing.T) {
		ast, appErr := e.svc.ExpressionToVisualAST(e.rctx, `user.attributes.department in ["--------", "Sales"]`)
		require.Nil(t, appErr)
		assert.True(t, ast.Conditions[0].HasMaskedValues)
		assert.Equal(t, []any{"Sales"}, ast.Conditions[0].Value)
	})

	t.Run("true is empty", func(t *testing.T) {
		ast, appErr := e.svc.ExpressionToVisualAST(e.rctx, `true`)
		require.Nil(t, appErr)
		assert.Empty(t, ast.Conditions)
	})

	t.Run("complex expression is rejected", func(t *testing.T) {
		_, appErr := e.svc.ExpressionToVisualAST(e.rctx, `user.attributes.department == "a" || user.attributes.clearance == "Secret"`)
		require.NotNil(t, appErr)
		assert.Equal(t, 400, appErr.StatusCode)
	})
}

func TestBuildQuery(t *testing.T) {
	e := newTestEnv(t)
	channelID := model.NewId()
	e.channelAttrs[channelID] = map[string]any{
		"minClearance":    map[string]any{"name": "Secret", "rank": float64(2)},
		"channelPrograms": []any{optAir},
	}

	t.Run("text and native", func(t *testing.T) {
		q, args, appErr := e.svc.buildQuery(e.rctx, `user.attributes.department == "Eng" && user.email.endsWith("@x_y.com") && user.verified == true`, "", false)
		require.Nil(t, appErr)
		assert.Equal(t, `(((UserAttributeView.Attributes ->> $1::text) = $2::text) AND (Users.Email LIKE $3::text) AND (Users.EmailVerified = $4::boolean))`, q)
		assert.Equal(t, []any{"department", "Eng", `%@x\_y.com`, true}, args)
	})

	t.Run("exclude native", func(t *testing.T) {
		q, args, appErr := e.svc.buildQuery(e.rctx, `user.attributes.department == "Eng" && user.createat.youngerThanDays(3)`, "", true)
		require.Nil(t, appErr)
		assert.Equal(t, `(((UserAttributeView.Attributes ->> $1::text) = $2::text) AND TRUE)`, q)
		assert.Len(t, args, 2)
	})

	t.Run("rank", func(t *testing.T) {
		q, args, appErr := e.svc.buildQuery(e.rctx, `user.attributes.clearance >= "Secret"`, "", false)
		require.Nil(t, appErr)
		assert.Equal(t, `(((UserAttributeView.Attributes -> $1::text) ->> 'rank')::bigint >= $2::bigint)`, q)
		assert.Equal(t, []any{"clearance", int64(2)}, args)
	})

	t.Run("rank against channel", func(t *testing.T) {
		q, args, appErr := e.svc.buildQuery(e.rctx, `user.attributes.clearance > resource.attributes.minClearance`, channelID, false)
		require.Nil(t, appErr)
		assert.Contains(t, q, `::bigint > $2::bigint`)
		assert.Equal(t, int64(2), args[1])
	})

	t.Run("channel required", func(t *testing.T) {
		_, _, appErr := e.svc.buildQuery(e.rctx, `user.attributes.clearance > resource.attributes.minClearance`, "", false)
		require.NotNil(t, appErr)
	})

	t.Run("multiselect", func(t *testing.T) {
		q, args, appErr := e.svc.buildQuery(e.rctx, `("Go" in user.attributes.skills || "SQL" in user.attributes.skills) && user.attributes.skills.hasAllOf(["A", "B"])`, "", false)
		require.Nil(t, appErr)
		assert.Equal(t, `((((UserAttributeView.Attributes -> $1::text) @> jsonb_build_array($2::text)) OR ((UserAttributeView.Attributes -> $3::text) @> jsonb_build_array($4::text))) AND ((UserAttributeView.Attributes -> $5::text) @> $6::jsonb))`, q)
		assert.Equal(t, `["A","B"]`, args[5])
	})

	t.Run("in list", func(t *testing.T) {
		q, _, appErr := e.svc.buildQuery(e.rctx, `user.attributes.department in ["a", "b"] && !(user.attributes.department in [])`, "", false)
		require.Nil(t, appErr)
		assert.Equal(t, `(((UserAttributeView.Attributes ->> $1::text) IN ($2::text, $3::text)) AND (NOT FALSE))`, q)
	})

	t.Run("graph", func(t *testing.T) {
		q, args, appErr := e.svc.buildQuery(e.rctx, `user.attributes.programs.coversAll(["F-18 Program"])`, "", false)
		require.Nil(t, appErr)
		assert.Contains(t, q, "jsonb_array_elements_text")
		// f18 is covered by itself, air and all.
		assert.ElementsMatch(t, []any{"programs", optF18, optAir, optAll}, args)
	})

	t.Run("session is not queryable", func(t *testing.T) {
		_, _, appErr := e.svc.buildQuery(e.rctx, `user.session.ip == "x"`, "", false)
		require.NotNil(t, appErr)
		assert.Equal(t, 400, appErr.StatusCode)
	})
}
