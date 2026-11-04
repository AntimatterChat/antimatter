// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost/server/public/model"
)

// The assistant's orderings moved from sort.Slice/SliceStable to the slices package (the vet check bans sort.*);
// these pin the orders the engines and the feed rely on, ties included.

func TestAssistantTrueFirst(t *testing.T) {
	assert.Equal(t, 0, assistantTrueFirst(true, true))
	assert.Equal(t, 0, assistantTrueFirst(false, false))
	assert.Equal(t, -1, assistantTrueFirst(true, false))
	assert.Equal(t, 1, assistantTrueFirst(false, true))

	t.Run("pinned cards first, the rest in their order", func(t *testing.T) {
		cards := []*model.AssistantCard{
			{Id: "a"}, {Id: "b", Pinned: true}, {Id: "c"}, {Id: "d", Pinned: true}, {Id: "e"},
		}
		slices.SortStableFunc(cards, func(x, y *model.AssistantCard) int { return assistantTrueFirst(x.Pinned, y.Pinned) })
		ids := make([]string, 0, len(cards))
		for _, c := range cards {
			ids = append(ids, c.Id)
		}
		assert.Equal(t, []string{"b", "d", "a", "c", "e"}, ids)
	})
}

func TestAssistantViewersOrder(t *testing.T) {
	viewers := assistantViewersOf(map[string]assistantPresenceEntry{
		"zed":   {OpenedAt: 10},
		"bob":   {OpenedAt: 20},
		"alice": {OpenedAt: 20},
		"carl":  {OpenedAt: 5},
	})
	assert.Equal(t, []assistantViewer{
		{userID: "carl", openedAt: 5},
		{userID: "zed", openedAt: 10},
		{userID: "alice", openedAt: 20},
		{userID: "bob", openedAt: 20},
	}, viewers)
}

func TestAssistantArgumentOptionsPreferTheArgumentsType(t *testing.T) {
	values := []model.AssistantEntityMatch{
		{Type: "service", Value: "web-app"},
		{Type: "host", Value: "db-2"},
		{Type: "ticket", Value: "OPS-42"},
		{Type: "host", Value: "www-1"},
	}
	options := assistantArgumentOptions("hostname", assistantSchemaProperty{Type: "string"}, values)
	got := make([]string, 0, len(options))
	for _, o := range options {
		got = append(got, o.Value)
	}
	// The hosts first, in the order they were named, then the others in theirs.
	assert.Equal(t, []string{"db-2", "www-1", "web-app", "OPS-42"}, got)
	assert.Equal(t, "web-app", values[0].Value, "the caller's values are left as they were")
}

func TestAssistantSchemaPropertyNamesAreSorted(t *testing.T) {
	schema := &assistantSchema{Properties: map[string]assistantSchemaProperty{
		"title": {}, "host": {}, "Body": {}, "assignee": {},
	}}
	assert.Equal(t, []string{"Body", "assignee", "host", "title"}, schema.propertyNames())
}
