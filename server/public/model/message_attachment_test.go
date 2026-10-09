// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageAttachment_IsValid(t *testing.T) {
	tests := map[string]struct {
		attachment *MessageAttachment
		wantErr    string
	}{
		"valid attachment": {
			attachment: &MessageAttachment{
				Text: "This is a test",
			},
			wantErr: "",
		},
		"invalid color": {
			attachment: &MessageAttachment{
				Color: "invalid",
			},
			wantErr: "invalid style 'invalid' - must be one of [good, warning, danger] or a hex color",
		},
		"valid predefined color": {
			attachment: &MessageAttachment{
				Color: "good",
			},
			wantErr: "",
		},
		"valid warning color": {
			attachment: &MessageAttachment{
				Color: "warning",
			},
			wantErr: "",
		},
		"valid danger color": {
			attachment: &MessageAttachment{
				Color: "danger",
			},
			wantErr: "",
		},
		"valid hex color": {
			attachment: &MessageAttachment{
				Color: "#FF0000",
			},
			wantErr: "",
		},
		"author link without name": {
			attachment: &MessageAttachment{
				AuthorLink: "http://example.com",
			},
			wantErr: "author link cannot be set without author name",
		},
		"invalid author link": {
			attachment: &MessageAttachment{
				AuthorName: "Author",
				AuthorLink: "invalid-url",
			},
			wantErr: "invalid author link URL",
		},
		"invalid author icon": {
			attachment: &MessageAttachment{
				AuthorIcon: "invalid-url",
			},
			wantErr: "invalid author icon URL",
		},
		"title link without title": {
			attachment: &MessageAttachment{
				TitleLink: "http://example.com",
			},
			wantErr: "title link cannot be set without title",
		},
		"invalid title link": {
			attachment: &MessageAttachment{
				Title:     "Title",
				TitleLink: "invalid-url",
			},
			wantErr: "invalid title link URL",
		},
		"invalid image URL": {
			attachment: &MessageAttachment{
				ImageURL: "invalid-url",
			},
			wantErr: "invalid image URL",
		},
		"invalid thumb URL": {
			attachment: &MessageAttachment{
				ThumbURL: "invalid-url",
			},
			wantErr: "invalid thumb URL",
		},
		"invalid footer icon": {
			attachment: &MessageAttachment{
				FooterIcon: "invalid-url",
			},
			wantErr: "invalid footer icon URL",
		},
		"invalid timestamp type": {
			attachment: &MessageAttachment{
				Timestamp: []string{"invalid"},
			},
			wantErr: "timestamp must be either a string or int64",
		},
		"valid timestamp string": {
			attachment: &MessageAttachment{
				Timestamp: "1234567890",
			},
			wantErr: "",
		},
		"valid timestamp int64": {
			attachment: &MessageAttachment{
				Timestamp: int64(1234567890),
			},
			wantErr: "",
		},
		"invalid action": {
			attachment: &MessageAttachment{
				Actions: []*PostAction{
					{
						Name: "", // Invalid - missing name
					},
				},
			},
			wantErr: "action must have a name",
		},
		"invalid field value type": {
			attachment: &MessageAttachment{
				Fields: []*MessageAttachmentField{
					{
						Title: "Title",
						Value: []string{"invalid"},
					},
				},
			},
			wantErr: "value must be either a string or int",
		},
		"valid fields": {
			attachment: &MessageAttachment{
				Fields: []*MessageAttachmentField{
					{
						Title: "Title",
						Value: "string value",
					},
					{
						Title: "Number",
						Value: 42,
					},
				},
			},
			wantErr: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.attachment.IsValid()
			if tc.wantErr == "" {
				assert.NoError(t, err, name)
			} else {
				assert.ErrorContains(t, err, tc.wantErr, name)
			}
		})
	}
}

func TestParseMessageAttachment(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		post := &Post{}
		attachments := []*MessageAttachment{}

		ParseMessageAttachment(post, attachments)

		expectedPost := &Post{
			Type: PostTypeMessageAttachment,
			Props: map[string]any{
				PostPropsAttachments: []*MessageAttachment{},
			},
		}
		assert.Equal(t, expectedPost, post)
	})

	t.Run("list with nil", func(t *testing.T) {
		post := &Post{}
		attachments := []*MessageAttachment{
			nil,
		}

		ParseMessageAttachment(post, attachments)

		expectedPost := &Post{
			Type: PostTypeMessageAttachment,
			Props: map[string]any{
				PostPropsAttachments: []*MessageAttachment{},
			},
		}
		assert.Equal(t, expectedPost, post)
	})
}

func TestMessageAttachment_Equals_PrimitiveAndNonPrimitiveField(t *testing.T) {
	// Field with primitive type (string)
	attachment1 := &MessageAttachment{
		Fields: []*MessageAttachmentField{
			{
				Title: "Field1",
				Value: "value",
				Short: true,
			},
		},
	}
	attachment2 := &MessageAttachment{
		Fields: []*MessageAttachmentField{
			{
				Title: "Field1",
				Value: "value",
				Short: true,
			},
		},
	}
	assert.True(t, attachment1.Equals(attachment2), "Attachments with identical primitive field values should be equal")

	// Field with non-primitive type ([]interface{})
	attachment3 := &MessageAttachment{
		Fields: []*MessageAttachmentField{
			{
				Title: "Field1",
				Value: []any{"value", 2},
				Short: true,
			},
		},
	}
	attachment4 := &MessageAttachment{
		Fields: []*MessageAttachmentField{
			{
				Title: "Field1",
				Value: []any{"value", 2},
				Short: true,
			},
		},
	}
	assert.True(t, attachment3.Equals(attachment4), "Attachments with identical non-primitive ([]interface{}) field values should be equal")
}

func TestMessageAttachment_StagesAndChart(t *testing.T) {
	threshold := 10.0
	card := func() *MessageAttachment {
		return &MessageAttachment{
			Title:  "Pipeline #4821 failed on main",
			Stages: []*MessageAttachmentStage{{Name: "build", Status: "passed"}, {Name: "test", Status: "failed"}, {Name: "flash", Status: "skipped"}},
			Chart:  &MessageAttachmentChart{Values: []float64{2.1, 4.8, 12.9}, Threshold: &threshold, Label: "mbar"},
		}
	}

	t.Run("valid", func(t *testing.T) {
		require.NoError(t, card().IsValid())
	})

	t.Run("invalid stages and chart", func(t *testing.T) {
		a := card()
		a.Stages = append(a.Stages, &MessageAttachmentStage{Name: "deploy", Status: "exploded"}, &MessageAttachmentStage{Status: "passed"}, nil)
		a.Chart.Values = []float64{1}
		err := a.IsValid()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status 'exploded'")
		assert.Contains(t, err.Error(), "stage at index 4 needs a name")
		assert.Contains(t, err.Error(), "stage at index 5 needs a name")
		assert.Contains(t, err.Error(), "a chart needs 2 to")
	})

	t.Run("too many stages", func(t *testing.T) {
		a := card()
		for range MessageAttachmentStagesMax {
			a.Stages = append(a.Stages, &MessageAttachmentStage{Name: "x", Status: "pending"})
		}
		assert.ErrorContains(t, a.IsValid(), "at most 20 stages")
	})

	t.Run("equals", func(t *testing.T) {
		assert.True(t, card().Equals(card()))
		other := card()
		other.Stages[1].Status = "passed"
		assert.False(t, card().Equals(other))
		other = card()
		higher := 11.0
		other.Chart.Threshold = &higher
		assert.False(t, card().Equals(other))
		other = card()
		other.Chart = nil
		assert.False(t, card().Equals(other))
	})

	t.Run("kept when a post's attachments are parsed", func(t *testing.T) {
		post := &Post{}
		post.AddProp(PostPropsAttachments, []any{map[string]any{
			"title":  "CI",
			"stages": []any{map[string]any{"name": "build", "status": "running"}},
			"chart":  map[string]any{"values": []any{1, 2, 3}, "threshold": 2},
		}})
		attachments := post.Attachments()
		require.Len(t, attachments, 1)
		require.Len(t, attachments[0].Stages, 1)
		assert.Equal(t, "running", attachments[0].Stages[0].Status)
		require.NotNil(t, attachments[0].Chart)
		assert.Equal(t, []float64{1, 2, 3}, attachments[0].Chart.Values)
		assert.Equal(t, 2.0, *attachments[0].Chart.Threshold)
	})
}
