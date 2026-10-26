// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package sqlstore

import (
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/db"
)

// antimatterMigrationVersionStart numbers Antimatter's own migrations; the versions below it are Mattermost's.
const antimatterMigrationVersionStart = 1000000

// embeddedMigrationVersions gives the versions of the migrations this server carries for the driver.
func embeddedMigrationVersions(driverName string) (map[int]bool, error) {
	entries, err := db.Assets().ReadDir(path.Join("migrations", driverName))
	if err != nil {
		return nil, err
	}
	versions := make(map[int]bool, len(entries)/2)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		prefix, _, _ := strings.Cut(entry.Name(), "_")
		version, err := strconv.Atoi(prefix)
		if err != nil {
			continue
		}
		versions[version] = true
	}
	return versions, nil
}

// unknownUpstreamMigrations gives the applied Mattermost migrations (versions below
// antimatterMigrationVersionStart) this server doesn't carry, oldest first: migrations a newer Mattermost applied
// before the database was moved to Antimatter.
func unknownUpstreamMigrations(applied []model.AppliedMigration, known map[int]bool) []model.AppliedMigration {
	var unknown []model.AppliedMigration
	for _, migration := range applied {
		if migration.Version < antimatterMigrationVersionStart && !known[migration.Version] {
			unknown = append(unknown, migration)
		}
	}
	slices.SortFunc(unknown, func(a, b model.AppliedMigration) int { return a.Version - b.Version })
	return unknown
}

// warnAboutUnknownUpstreamMigrations logs a warning when the database has Mattermost migrations applied that this
// server doesn't know, which happens when it comes from a Mattermost version newer than the one Antimatter is based
// on. The migration engine skips them without a word, but they may have changed tables the server writes to (a new
// column without a default, say). The server still starts: most such migrations are harmless, and refusing would
// leave the admin with no way forward but restoring a backup.
func (ss *SqlStore) warnAboutUnknownUpstreamMigrations() {
	exists, err := ss.tableExists("db_migrations")
	if err != nil || !exists {
		return
	}
	applied, err := ss.GetAppliedMigrations()
	if err != nil {
		ss.Logger().Warn("Couldn't list the applied database migrations", mlog.Err(err))
		return
	}
	known, err := embeddedMigrationVersions(ss.DriverName())
	if err != nil {
		ss.Logger().Warn("Couldn't list the database migrations of this server", mlog.Err(err))
		return
	}
	if unknown := unknownUpstreamMigrations(applied, known); len(unknown) > 0 {
		ss.Logger().Warn(unknownUpstreamMigrationsWarning, mlog.Array("unknown_migrations", describeMigrations(unknown)))
	}
}

const unknownUpstreamMigrationsWarning = "The database has migrations applied that this server doesn't know: it was migrated by a newer Mattermost version " +
	"than the one this server is based on. They are left as they are; if they changed tables this server writes to, some writes may fail. " +
	"Restore a backup taken before that upgrade, or check the migrations listed here, if errors follow."

// describeMigrations names migrations as their files do: 000236_add_something.
func describeMigrations(migrations []model.AppliedMigration) []string {
	names := make([]string, 0, len(migrations))
	for _, migration := range migrations {
		names = append(names, fmt.Sprintf("%06d_%s", migration.Version, migration.Name))
	}
	return names
}
