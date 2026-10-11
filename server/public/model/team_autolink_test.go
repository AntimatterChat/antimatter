// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func compile(t *testing.T, l TeamAutolink) *CompiledAutolink {
	t.Helper()
	c, err := l.Compile()
	require.NoError(t, err)
	return c
}

func TestTeamAutolinkReplace(t *testing.T) {
	jira := compile(t, TeamAutolink{Pattern: `(?P<key>LUM-\d+)`, Template: `[${key}](https://jira.example/browse/${key})`})
	assert.Equal(t, "See [LUM-12](https://jira.example/browse/LUM-12) and [LUM-3](https://jira.example/browse/LUM-3).",
		jira.Replace("See LUM-12 and LUM-3."), "after a space, before punctuation")
	assert.Equal(t, "xLUM-12", jira.Replace("xLUM-12"), "not inside a word")

	word := compile(t, TeamAutolink{Pattern: `PR(\d+)`, Template: `[PR$1](https://github.com/lumen/os/pull/$1)`, WordMatch: true})
	assert.Equal(t, "fixed in [PR42](https://github.com/lumen/os/pull/42), not xPR43", word.Replace("fixed in PR42, not xPR43"))

	loose := compile(t, TeamAutolink{Pattern: `CVE-(\d{4})-(\d+)`, Template: `[CVE-$1-$2](https://cve.example/$1/$2)`, DisableNonWordPrefix: true, DisableNonWordSuffix: true})
	assert.Equal(t, "(see [CVE-2026-1234](https://cve.example/2026/1234))", loose.Replace("(see CVE-2026-1234)"))

	disabled := compile(t, TeamAutolink{Pattern: `x`, Template: `y`, Disabled: true})
	assert.Nil(t, disabled)
	assert.Equal(t, "x", disabled.Replace("x"))
}

func TestTeamAutolinksIsValid(t *testing.T) {
	valid := &TeamAutolinks{Links: []*TeamAutolink{{Name: "Jira", Pattern: `LUM-\d+`, Template: `[$0](https://jira.example)`}}}
	require.Nil(t, valid.IsValid())

	for name, links := range map[string]*TeamAutolinks{
		"bad pattern":      {Links: []*TeamAutolink{{Pattern: `(`, Template: "x"}}},
		"empty match":      {Links: []*TeamAutolink{{Pattern: `a*`, Template: "x"}}},
		"no template":      {Links: []*TeamAutolink{{Pattern: `a`}}},
		"nil rule":         {Links: []*TeamAutolink{nil}},
		"bad channel":      {Links: []*TeamAutolink{{Pattern: `a`, Template: "x", Channels: []string{"nope"}}}},
		"too long pattern": {Links: []*TeamAutolink{{Pattern: strings.Repeat("a", TeamAutolinkPatternMax+1), Template: "x"}}},
	} {
		assert.NotNil(t, links.IsValid(), name)
	}
	many := &TeamAutolinks{}
	for range TeamAutolinksMax + 1 {
		many.Links = append(many.Links, &TeamAutolink{Pattern: "a", Template: "b"})
	}
	assert.NotNil(t, many.IsValid())
}

func TestTeamIsValidChecksAutolinks(t *testing.T) {
	team := &Team{Id: NewId(), CreateAt: 1, UpdateAt: 1, DisplayName: "Lumen", Name: "lumen", Email: "a@b.c", Type: TeamOpen, InviteId: NewId()}
	require.Nil(t, team.IsValid())
	team.Autolinks = `{"links":[{"pattern":"(","template":"x"}]}`
	require.NotNil(t, team.IsValid())
	team.Autolinks = `not json`
	require.NotNil(t, team.IsValid())
	team.Autolinks = `{"links":[{"pattern":"LUM-\\d+","template":"x"}],"on_edit":true}`
	require.Nil(t, team.IsValid())
	links, err := ParseTeamAutolinks(team.Autolinks)
	require.NoError(t, err)
	assert.True(t, links.OnEdit)
}

func TestShiftGroupRefs(t *testing.T) {
	assert.Equal(t, "[${2}](x/${3}) ${AutolinkMatch} ${key} $$1 $1x", shiftGroupRefs("[$1](x/${2}) $0 ${key} $$1 $1x", 1))
}

func TestTeamAutolinkWholeMatch(t *testing.T) {
	cve := compile(t, TeamAutolink{Pattern: `CVE-\d{4}-\d+`, Template: `[$0](https://cve.example/$0)`})
	assert.Equal(t, "fix [CVE-2026-1](https://cve.example/CVE-2026-1).", cve.Replace("fix CVE-2026-1."))
	word := compile(t, TeamAutolink{Pattern: `LUM-(\d+)`, Template: `[$0](https://jira.example/$1)`, WordMatch: true})
	assert.Equal(t, "[LUM-7](https://jira.example/7)", word.Replace("LUM-7"))
}
