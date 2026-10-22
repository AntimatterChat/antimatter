// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// The AD/LDAP and SAML services sync user attributes through the
// PropertySyncer their backends hand them. These tests cover that wiring and
// what the synced values and options then allow everyone else to do.
func TestAttributeSyncBackends(t *testing.T) {
	mainHelper.Parallel(t)
	h := setupPropertySyncTest(t)

	ldapTemplate, ldapSelect := h.createAttribute(model.PropertyFieldTypeSelect, ldapAttrs("l"))
	_, samlMulti := h.createAttribute(model.PropertyFieldTypeMultiselect, samlAttrs("groups"))
	_, both := h.createAttribute(model.PropertyFieldTypeText, model.StringInterface{
		model.PropertyFieldAttrLDAP: "departmentNumber",
		model.PropertyFieldAttrSAML: "department",
	})

	t.Run("the AD/LDAP backend syncs only the attributes synced from AD/LDAP", func(t *testing.T) {
		syncer, appErr := (&ldapBackend{a: h.App}).NewAttributeSyncer(request.TestContext(t))
		require.Nil(t, appErr)
		require.True(t, syncer.HasMappings())
		assert.Equal(t, []string{"departmentNumber", "l"}, syncer.ExternalAttributes())

		_, appErr = syncer.SyncUser(request.TestContext(t), h.BasicUser.Id, map[string][]string{
			"l":                {"Paris"},
			"departmentNumber": {"R&D"},
			"groups":           {"not synced from AD/LDAP"},
		}, model.PropertySyncOptions{PruneOrphanedOptions: true})
		require.Nil(t, appErr)

		assert.Equal(t, h.optionID(ldapTemplate.ID, "Paris"), h.valueString(h.BasicUser.Id, ldapSelect.ID), "the missing option was created on the template")
		assert.Equal(t, "R&D", h.valueString(h.BasicUser.Id, both.ID), "a field naming both sources is synced from AD/LDAP")
		assert.Nil(t, h.value(h.BasicUser.Id, samlMulti.ID), "a SAML attribute is not written by the AD/LDAP sync")
	})

	t.Run("the SAML backend syncs only the attributes synced from SAML alone", func(t *testing.T) {
		syncer, appErr := (&samlBackend{a: h.App}).NewAttributeSyncer(request.TestContext(t))
		require.Nil(t, appErr)
		assert.Equal(t, []string{"groups"}, syncer.ExternalAttributes(), "a field naming both sources is left to AD/LDAP")

		_, appErr = syncer.SyncUser(request.TestContext(t), h.BasicUser2.Id, map[string][]string{
			"groups":     {"Sales", "Support"},
			"department": {"ignored"},
		}, model.PropertySyncOptions{PruneOrphanedOptions: true})
		require.Nil(t, appErr)

		assert.ElementsMatch(t, h.optionIDs(samlMulti.ID, "Sales", "Support"), h.valueStrings(h.BasicUser2.Id, samlMulti.ID))
		assert.Equal(t, "", h.valueString(h.BasicUser2.Id, both.ID))
	})

	t.Run("a user cannot overwrite or clear a value the sync wrote", func(t *testing.T) {
		before := h.valueString(h.BasicUser.Id, ldapSelect.ID)
		require.NotEmpty(t, before)

		for _, raw := range []string{`"` + h.optionID(ldapTemplate.ID, "Paris") + `"`, `""`} {
			_, appErr := h.App.UpsertPropertyValues(h.emptyContextWithCallerID(h.BasicUser.Id), []*model.PropertyValue{{
				GroupID:    h.groupID,
				TargetType: model.PropertyValueTargetTypeUser,
				TargetID:   h.BasicUser.Id,
				FieldID:    ldapSelect.ID,
				Value:      json.RawMessage(raw),
			}}, model.PropertyFieldObjectTypeUser, h.BasicUser.Id, "")
			require.NotNil(t, appErr, "a user must not change a value the identity provider decides")
		}
		assert.Equal(t, before, h.valueString(h.BasicUser.Id, ldapSelect.ID))
	})

	t.Run("an admin cannot overwrite a synced value either", func(t *testing.T) {
		_, appErr := h.App.UpsertPropertyValues(h.adminRctx, []*model.PropertyValue{{
			GroupID:    h.groupID,
			TargetType: model.PropertyValueTargetTypeUser,
			TargetID:   h.BasicUser2.Id,
			FieldID:    samlMulti.ID,
			Value:      json.RawMessage(`[]`),
		}}, model.PropertyFieldObjectTypeUser, h.BasicUser2.Id, "")
		require.NotNil(t, appErr)
		assert.Equal(t, "app.property.sync_lock.app_error", appErr.Id)
		assert.Len(t, h.valueStrings(h.BasicUser2.Id, samlMulti.ID), 2)
	})

	t.Run("an admin cannot add an option to a synced attribute", func(t *testing.T) {
		_, appErr := h.App.CreatePropertyFieldOptions(h.adminRctx, h.groupID, ldapTemplate.ID, []*model.PropertyFieldOption{{Name: "Lyon"}}, "")
		require.NotNil(t, appErr)
		assert.Equal(t, "app.property_field.synced_options_locked.app_error", appErr.Id)
		assert.Equal(t, []string{"Paris"}, h.optionNames(ldapTemplate.ID))
	})

	t.Run("the other sync cannot change the options of an attribute it does not sync", func(t *testing.T) {
		rctx := RequestContextWithCallerID(request.TestContext(t), model.CallerIDSAMLSync)
		_, appErr := h.App.CreatePropertyFieldOptions(rctx, h.groupID, ldapTemplate.ID, []*model.PropertyFieldOption{{Name: "Lyon"}}, "")
		require.NotNil(t, appErr)
		assert.Equal(t, []string{"Paris"}, h.optionNames(ldapTemplate.ID))
	})

	t.Run("no syncer is needed when nothing is synced from a source", func(t *testing.T) {
		h2 := setupPropertySyncTest(t)
		syncer, appErr := (&samlBackend{a: h2.App}).NewAttributeSyncer(request.TestContext(t))
		require.Nil(t, appErr)
		assert.False(t, syncer.HasMappings())
	})
}
