// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package accesscontrol

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

type fakeResolver map[string]*model.MaskingFieldInfo

func (f fakeResolver) Resolve(objectType, fieldName string) (*model.MaskingFieldInfo, error) {
	info, ok := f[objectType+"/"+fieldName]
	if !ok {
		return nil, errors.New("unknown field")
	}
	return info, nil
}

func sharedOnly(values ...string) *model.MaskingFieldInfo {
	v := map[string]struct{}{}
	for _, s := range values {
		v[s] = struct{}{}
	}
	return &model.MaskingFieldInfo{Access: model.MaskingFieldAccessSharedOnly, VisibleValues: v}
}

func TestMasking(t *testing.T) {
	e := newTestEnv(t)
	resolver := fakeResolver{
		"user/department": sharedOnly("Engineering"),
		"user/skills":     sharedOnly("Go"),
		"user/clearance":  {Access: model.MaskingFieldAccessPublic},
	}

	t.Run("has masked", func(t *testing.T) {
		has, appErr := e.svc.HasMaskedValuesForCaller(e.rctx, `user.attributes.department == "Engineering"`, resolver)
		require.Nil(t, appErr)
		assert.False(t, has)

		has, appErr = e.svc.HasMaskedValuesForCaller(e.rctx, `user.attributes.department in ["Engineering", "Sales"]`, resolver)
		require.Nil(t, appErr)
		assert.True(t, has)

		_, appErr = e.svc.HasMaskedValuesForCaller(e.rctx, `user.attributes.unknown == "x"`, resolver)
		require.NotNil(t, appErr, "resolver errors fail closed")
	})

	t.Run("mask", func(t *testing.T) {
		masked, did, appErr := e.svc.MaskExpressionForCaller(e.rctx,
			`user.attributes.department in ["Engineering", "Sales", "Ops"] && ("Go" in user.attributes.skills || "Rust" in user.attributes.skills) && user.attributes.clearance == "Secret"`, resolver)
		require.Nil(t, appErr)
		assert.True(t, did)
		assert.Equal(t, `user.attributes.department in ["Engineering", "--------"] && ("Go" in user.attributes.skills || "--------" in user.attributes.skills) && user.attributes.clearance == "Secret"`, masked)

		same, did, appErr := e.svc.MaskExpressionForCaller(e.rctx, `user.attributes.department == "Engineering"`, resolver)
		require.Nil(t, appErr)
		assert.False(t, did)
		assert.Equal(t, `user.attributes.department == "Engineering"`, same)
	})

	t.Run("validate", func(t *testing.T) {
		assert.Nil(t, e.svc.ValidateExpressionValuesForCaller(e.rctx, `user.attributes.department in ["Engineering", "--------"]`, resolver))
		appErr := e.svc.ValidateExpressionValuesForCaller(e.rctx, `user.attributes.department == "Sales"`, resolver)
		require.NotNil(t, appErr)
		assert.Equal(t, "app.pap.save_policy.invalid_value", appErr.Id)
	})

	t.Run("merge", func(t *testing.T) {
		stored := `user.attributes.department in ["Engineering", "Sales"] && user.attributes.clearance == "Secret"`

		merged, appErr := e.svc.MergeExpressionWithMaskedValuesCanonical(e.rctx,
			`user.attributes.department in ["Engineering", "Marketing"] && user.attributes.clearance == "TopSecret"`, stored, resolver)
		require.Nil(t, appErr)
		assert.Equal(t, `user.attributes.department in ["Engineering", "Marketing", "Sales"] && user.attributes.clearance == "TopSecret"`, merged)

		// Fully masked row submitted as the placeholder.
		merged, appErr = e.svc.MergeExpressionWithMaskedValuesCanonical(e.rctx,
			`user.attributes.department in [] && user.attributes.clearance == "Secret"`, `user.attributes.department in ["Sales"] && user.attributes.clearance == "Secret"`, resolver)
		require.Nil(t, appErr)
		assert.Equal(t, `user.attributes.department in ["Sales"] && user.attributes.clearance == "Secret"`, merged)

		// Member chains re-inject into the chain.
		merged, appErr = e.svc.MergeExpressionWithMaskedValuesCanonical(e.rctx,
			`"Go" in user.attributes.skills`, `"Go" in user.attributes.skills && "Rust" in user.attributes.skills`, resolver)
		require.Nil(t, appErr)
		assert.Equal(t, `"Go" in user.attributes.skills && "Rust" in user.attributes.skills`, merged)

		// Dropping a condition that held hidden values is refused.
		_, appErr = e.svc.MergeExpressionWithMaskedValuesCanonical(e.rctx, `user.attributes.clearance == "Secret"`, stored, resolver)
		require.NotNil(t, appErr)
		assert.Equal(t, 403, appErr.StatusCode)

		// Replacing a hidden single value is refused; the token restores it.
		_, appErr = e.svc.MergeExpressionWithMaskedValuesCanonical(e.rctx, `user.attributes.department == "Engineering"`, `user.attributes.department == "Sales"`, resolver)
		require.NotNil(t, appErr)
		merged, appErr = e.svc.MergeExpressionWithMaskedValuesCanonical(e.rctx, `user.attributes.department == "--------"`, `user.attributes.department == "Sales"`, resolver)
		require.Nil(t, appErr)
		assert.Equal(t, `user.attributes.department == "Sales"`, merged)

		// Nothing hidden: the submission is returned untouched.
		merged, appErr = e.svc.MergeExpressionWithMaskedValuesCanonical(e.rctx, `user.attributes.clearance == "x" || true`, `user.attributes.department == "Engineering"`, resolver)
		require.Nil(t, appErr)
		assert.Equal(t, `user.attributes.clearance == "x" || true`, merged)
	})
}
