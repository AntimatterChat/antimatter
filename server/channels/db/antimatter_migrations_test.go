// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package db

import (
	"io/fs"
	"path"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Antimatter's own schema migrations are numbered from 1000001 (see UPSTREAM_MERGE_RULES.md, section F).
const antimatterFirstMigration = 1000001

var migrationName = regexp.MustCompile(`^(\d+)_([a-z0-9_]+)\.(up|down)\.sql$`)

// rbacTouch matches SQL that reads or writes who holds which role or permission: the Roles and Schemes tables, the
// Roles column of Users, and the scheme flags and explicit roles of team and channel members.
var rbacTouch = regexp.MustCompile(`(?i)\b(roles|schemes|schemeadmin|schemeuser|schemeguest|explicitroles)\b`)

// rbacMigrationsReviewed lists the Antimatter migrations allowed to touch roles, each reviewed for who it promotes.
// Adding one here is a deliberate decision: say in the commit what it grants and to whom, and add a test that
// ordinary users, guests and members come out of it with no more than they had.
var rbacMigrationsReviewed = map[string]bool{}

func antimatterMigrations(t *testing.T) map[string]string {
	t.Helper()
	files, err := fs.ReadDir(Assets(), "migrations/postgres")
	require.NoError(t, err)
	out := map[string]string{}
	for _, f := range files {
		m := migrationName.FindStringSubmatch(f.Name())
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		require.NoError(t, err)
		if n < antimatterFirstMigration {
			continue
		}
		data, err := fs.ReadFile(Assets(), path.Join("migrations/postgres", f.Name()))
		require.NoError(t, err)
		out[f.Name()] = string(data)
	}
	return out
}

func TestAntimatterMigrationsNaming(t *testing.T) {
	files, err := fs.ReadDir(Assets(), "migrations/postgres")
	require.NoError(t, err)
	migrations := antimatterMigrations(t)
	require.NotEmpty(t, migrations)

	for name := range migrations {
		m := migrationName.FindStringSubmatch(name)
		assert.True(t, strings.HasPrefix(m[2], "antimatter_"), "%s: Antimatter migrations are named <number>_antimatter_<what>", name)
		other := strings.TrimSuffix(name, "."+m[3]+".sql") + map[string]string{"up": ".down.sql", "down": ".up.sql"}[m[3]]
		_, ok := migrations[other]
		assert.True(t, ok, "%s has no %s", name, other)
	}

	// Upstream's migrations stay below Antimatter's range, so morph applies both by name whatever order they merge in.
	for _, f := range files {
		m := migrationName.FindStringSubmatch(f.Name())
		if m == nil {
			continue
		}
		if strings.HasPrefix(m[2], "antimatter_") {
			n, _ := strconv.Atoi(m[1])
			assert.GreaterOrEqual(t, n, antimatterFirstMigration, "%s: Antimatter migrations are numbered from %d", f.Name(), antimatterFirstMigration)
		}
	}
}

// TestAntimatterMigrationsLeaveRolesAlone makes sure no Antimatter schema migration changes who holds which role:
// an update must never make ordinary users admins, nor guests members.
func TestAntimatterMigrationsLeaveRolesAlone(t *testing.T) {
	for name, sql := range antimatterMigrations(t) {
		if rbacMigrationsReviewed[name] {
			continue
		}
		// Comments may mention roles freely; only the statements count.
		var statements []string
		for line := range strings.SplitSeq(sql, "\n") {
			if i := strings.Index(line, "--"); i >= 0 {
				line = line[:i]
			}
			statements = append(statements, line)
		}
		match := rbacTouch.FindString(strings.Join(statements, "\n"))
		assert.Empty(t, match, "%s touches %q: a migration that changes roles needs a review (see rbacMigrationsReviewed)", name, match)
	}
}
