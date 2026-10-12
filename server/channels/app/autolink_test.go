// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestApplyAutolinks(t *testing.T) {
	compile := func(l model.TeamAutolink) *model.CompiledAutolink {
		c, err := l.Compile()
		require.NoError(t, err)
		return c
	}
	jira := compile(model.TeamAutolink{Pattern: `(LUM-\d+)`, Template: `[$1](https://jira.example/browse/$1)`})
	firmwareOnly := compile(model.TeamAutolink{Pattern: `(rc\d+)`, Template: `[$1](https://ci.example/$1)`, Channels: []string{"firmware"}})
	links := []*model.CompiledAutolink{jira, firmwareOnly}

	assert.Equal(t, "Fix [LUM-12](https://jira.example/browse/LUM-12) in [rc3](https://ci.example/rc3)",
		ApplyAutolinks("Fix LUM-12 in rc3", links, "firmware", false))
	assert.Equal(t, "Fix [LUM-12](https://jira.example/browse/LUM-12) in rc3", ApplyAutolinks("Fix LUM-12 in rc3", links, "random", false), "channel scope")
	assert.Equal(t, "see `LUM-12` and [LUM-12](https://elsewhere)", ApplyAutolinks("see `LUM-12` and [LUM-12](https://elsewhere)", links, "random", false),
		"never in code or links")
	assert.Equal(t, "```\nLUM-12\n```", ApplyAutolinks("```\nLUM-12\n```", links, "random", false), "nor code blocks")
	assert.Equal(t, "**[LUM-1](https://jira.example/browse/LUM-1)** then [LUM-2](https://jira.example/browse/LUM-2)",
		ApplyAutolinks("**LUM-1** then LUM-2", links, "random", false), "offsets follow earlier replacements")
	assert.Equal(t, "LUM-12", ApplyAutolinks("LUM-12", links, "random", true), "bots are left alone")
	botsToo := compile(model.TeamAutolink{Pattern: `(LUM-\d+)`, Template: `[$1](x)`, ProcessBotPosts: true})
	assert.Equal(t, "[LUM-12](x)", ApplyAutolinks("LUM-12", []*model.CompiledAutolink{botsToo}, "random", true))
}

func TestApplyAutolinksBounded(t *testing.T) {
	// Three rules turning every "a" into 1,000 of them would make a billion: the message stays as written instead.
	var links []*model.CompiledAutolink
	for range 3 {
		c, err := (&model.TeamAutolink{Pattern: `a`, Template: strings.Repeat("a", 1000), DisableNonWordPrefix: true, DisableNonWordSuffix: true}).Compile()
		require.NoError(t, err)
		links = append(links, c)
	}
	start := time.Now()
	assert.Equal(t, "a a", ApplyAutolinks("a a", links, "random", false))
	assert.Less(t, time.Since(start), 5*time.Second)

	// Within the limit, rules still chain.
	assert.Equal(t, strings.Repeat("a", 1000), ApplyAutolinks("a", links[:1], "random", false))
	// Whole words too: each rule turns the word into 500 of them.
	c, err := (&model.TeamAutolink{Pattern: `a`, Template: strings.TrimSpace(strings.Repeat("a ", 500)), WordMatch: true}).Compile()
	require.NoError(t, err)
	assert.Equal(t, "x a y", ApplyAutolinks("x a y", []*model.CompiledAutolink{c, c, c}, "random", false))

	// A message whose other parts (code here) are already past the longest one stored leaves its text as written:
	// it doesn't escape the bound. (Two rules, so that a regression fails here rather than taking all memory.)
	long := "`" + strings.Repeat("b", model.PostMessageMaxBytesV2) + "` a"
	assert.Equal(t, long, ApplyAutolinks(long, links[:2], "random", false))
}

func TestTeamAutolinksCache(t *testing.T) {
	team := &model.Team{Id: model.NewId(), Autolinks: `{"links":[{"pattern":"a","template":"b"}]}`}
	first := teamAutolinks(team)
	require.Len(t, first.links, 1)
	assert.Same(t, first, teamAutolinks(team), "compiled once")
	team.Autolinks = `{"links":[{"pattern":"a","template":"b"},{"pattern":"c","template":"d","disabled":true}],"on_edit":true}`
	second := teamAutolinks(team)
	assert.NotSame(t, first, second, "recompiled after a change")
	assert.True(t, second.onEdit)
	assert.Len(t, second.links, 1, "disabled rules are skipped")
	team.Autolinks = ""
	assert.Nil(t, teamAutolinks(team))
}
