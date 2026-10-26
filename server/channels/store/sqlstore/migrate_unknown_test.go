// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package sqlstore

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestEmbeddedMigrationVersions(t *testing.T) {
	versions, err := embeddedMigrationVersions(model.DatabaseDriverPostgres)
	require.NoError(t, err)
	require.True(t, versions[1], "Mattermost's first migration")
	require.True(t, versions[235], "the last Mattermost migration this server carries")
	require.True(t, versions[1000001], "Antimatter's own migrations")
	require.False(t, versions[0])
}

// A database migrated by a newer Mattermost has Mattermost migrations this server doesn't carry: they are listed
// (and logged as a warning at startup), Antimatter's own and the known ones aren't.
func TestUnknownUpstreamMigrations(t *testing.T) {
	known := map[int]bool{1: true, 2: true, 235: true, 1000001: true}
	applied := []model.AppliedMigration{
		{Version: 1000002, Name: "antimatter_newer_than_this_build"},
		{Version: 237, Name: "add_another_thing"},
		{Version: 236, Name: "add_something"},
		{Version: 235, Name: "known"},
		{Version: 1000001, Name: "antimatter_known"},
		{Version: 2, Name: "known_too"},
	}

	unknown := unknownUpstreamMigrations(applied, known)
	require.Equal(t, []model.AppliedMigration{
		{Version: 236, Name: "add_something"},
		{Version: 237, Name: "add_another_thing"},
	}, unknown)
	require.Equal(t, []string{"000236_add_something", "000237_add_another_thing"}, describeMigrations(unknown))

	require.Empty(t, unknownUpstreamMigrations(applied[3:], known))
	require.Empty(t, unknownUpstreamMigrations(nil, known))
}
