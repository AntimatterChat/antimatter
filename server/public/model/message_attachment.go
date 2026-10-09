// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"

	"github.com/hashicorp/go-multierror"
)

var (
	linkWithTextRegex = regexp.MustCompile(`<([^<\|]+)\|([^>]+)>`)
	hexColorRegex     = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

type MessageAttachment struct {
	Id         int64                     `json:"id"`
	Fallback   string                    `json:"fallback"`
	Color      string                    `json:"color"`
	Pretext    string                    `json:"pretext"`
	AuthorName string                    `json:"author_name"`
	AuthorLink string                    `json:"author_link"`
	AuthorIcon string                    `json:"author_icon"`
	Title      string                    `json:"title"`
	TitleLink  string                    `json:"title_link"`
	Text       string                    `json:"text"`
	Fields     []*MessageAttachmentField `json:"fields"`
	ImageURL   string                    `json:"image_url"`
	ThumbURL   string                    `json:"thumb_url"`
	Footer     string                    `json:"footer"`
	FooterIcon string                    `json:"footer_icon"`
	Timestamp  any                       `json:"ts"` // This is either a string or an int64
	Actions    []*PostAction             `json:"actions,omitempty"`
	// Antimatter: a pipeline's stages as pills, and a small chart of recent values, as in the Fusion mockup's CI and
	// monitoring cards.
	Stages []*MessageAttachmentStage `json:"stages,omitempty"`
	Chart  *MessageAttachmentChart   `json:"chart,omitempty"`
}

const (
	MessageAttachmentStagesMax     = 20
	MessageAttachmentStageNameMax  = 64
	MessageAttachmentChartValueMax = 200
	MessageAttachmentChartLabelMax = 128
)

// The states of a pipeline stage.
var messageAttachmentStageStatuses = []string{"passed", "failed", "running", "pending", "skipped", "canceled"}

// MessageAttachmentStage is one stage of a CI pipeline: build, test, deploy...
type MessageAttachmentStage struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// MessageAttachmentChart is a sparkline: values in time order, and a threshold drawn as a line (an alert's limit).
type MessageAttachmentChart struct {
	Values    []float64 `json:"values"`
	Threshold *float64  `json:"threshold,omitempty"`
	Label     string    `json:"label,omitempty"`
}

func (s *MessageAttachment) IsValid() error {
	var multiErr *multierror.Error

	if s.Color != "" {
		validStyles := []string{"good", "warning", "danger"}
		// If not a predefined style, check if it's a hex color
		if !slices.Contains(validStyles, s.Color) && !hexColorRegex.MatchString(s.Color) {
			multiErr = multierror.Append(multiErr, fmt.Errorf("invalid style '%s' - must be one of [good, warning, danger] or a hex color", s.Color))
		}
	}

	if s.AuthorLink != "" {
		if s.AuthorName == "" {
			multiErr = multierror.Append(multiErr, fmt.Errorf("author link cannot be set without author name"))
		}

		if !IsValidHTTPURL(s.AuthorLink) {
			multiErr = multierror.Append(multiErr, fmt.Errorf("invalid author link URL"))
		}
	}

	if s.AuthorIcon != "" && !IsValidHTTPURL(s.AuthorIcon) {
		multiErr = multierror.Append(multiErr, fmt.Errorf("invalid author icon URL"))
	}

	if s.TitleLink != "" {
		if s.Title == "" {
			multiErr = multierror.Append(multiErr, fmt.Errorf("title link cannot be set without title"))
		}

		if !IsValidHTTPURL(s.TitleLink) {
			multiErr = multierror.Append(multiErr, fmt.Errorf("invalid title link URL"))
		}
	}

	for _, field := range s.Fields {
		if err := field.IsValid(); err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}

	if s.ImageURL != "" && !IsValidHTTPURL(s.ImageURL) {
		multiErr = multierror.Append(multiErr, fmt.Errorf("invalid image URL"))
	}

	if s.ThumbURL != "" && !IsValidHTTPURL(s.ThumbURL) {
		multiErr = multierror.Append(multiErr, fmt.Errorf("invalid thumb URL"))
	}

	if s.FooterIcon != "" && !IsValidHTTPURL(s.FooterIcon) {
		multiErr = multierror.Append(multiErr, fmt.Errorf("invalid footer icon URL"))
	}

	// Validate timestamp is either string or int64
	if s.Timestamp != nil {
		switch s.Timestamp.(type) {
		case string, int64:
			// Valid types
		default:
			multiErr = multierror.Append(multiErr, fmt.Errorf("timestamp must be either a string or int64"))
		}
	}

	for i, action := range s.Actions {
		if err := action.IsValid(); err != nil {
			multiErr = multierror.Append(multiErr, multierror.Prefix(err, fmt.Sprintf("action at index %d is invalid:", i)))
		}
	}

	if len(s.Stages) > MessageAttachmentStagesMax {
		multiErr = multierror.Append(multiErr, fmt.Errorf("at most %d stages", MessageAttachmentStagesMax))
	}
	for i, stage := range s.Stages {
		if stage == nil || stage.Name == "" || len(stage.Name) > MessageAttachmentStageNameMax {
			multiErr = multierror.Append(multiErr, fmt.Errorf("stage at index %d needs a name of at most %d characters", i, MessageAttachmentStageNameMax))
		} else if !slices.Contains(messageAttachmentStageStatuses, stage.Status) {
			multiErr = multierror.Append(multiErr, fmt.Errorf("stage at index %d has an invalid status '%s'", i, stage.Status))
		}
	}

	if s.Chart != nil {
		if len(s.Chart.Values) < 2 || len(s.Chart.Values) > MessageAttachmentChartValueMax {
			multiErr = multierror.Append(multiErr, fmt.Errorf("a chart needs 2 to %d values", MessageAttachmentChartValueMax))
		}
		if len(s.Chart.Label) > MessageAttachmentChartLabelMax {
			multiErr = multierror.Append(multiErr, fmt.Errorf("chart label is longer than %d characters", MessageAttachmentChartLabelMax))
		}
	}

	return multiErr.ErrorOrNil()
}

func (s *MessageAttachment) Equals(input *MessageAttachment) bool {
	// Direct comparison of simple types

	if s.Id != input.Id {
		return false
	}

	if s.Fallback != input.Fallback {
		return false
	}

	if s.Color != input.Color {
		return false
	}

	if s.Pretext != input.Pretext {
		return false
	}

	if s.AuthorName != input.AuthorName {
		return false
	}

	if s.AuthorLink != input.AuthorLink {
		return false
	}

	if s.AuthorIcon != input.AuthorIcon {
		return false
	}

	if s.Title != input.Title {
		return false
	}

	if s.TitleLink != input.TitleLink {
		return false
	}

	if s.Text != input.Text {
		return false
	}

	if s.ImageURL != input.ImageURL {
		return false
	}

	if s.ThumbURL != input.ThumbURL {
		return false
	}

	if s.Footer != input.Footer {
		return false
	}

	if s.FooterIcon != input.FooterIcon {
		return false
	}

	// Compare length & slice values of fields
	if len(s.Fields) != len(input.Fields) {
		return false
	}

	for j := range s.Fields {
		if !s.Fields[j].Equals(input.Fields[j]) {
			return false
		}
	}

	// Compare length & slice values of actions
	if len(s.Actions) != len(input.Actions) {
		return false
	}

	for j := range s.Actions {
		if !s.Actions[j].Equals(input.Actions[j]) {
			return false
		}
	}

	if !slices.EqualFunc(s.Stages, input.Stages, func(a, b *MessageAttachmentStage) bool {
		return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
	}) {
		return false
	}

	if !s.Chart.Equals(input.Chart) {
		return false
	}

	return s.Timestamp == input.Timestamp
}

func (c *MessageAttachmentChart) Equals(input *MessageAttachmentChart) bool {
	if c == nil || input == nil {
		return c == input
	}
	if c.Label != input.Label || !slices.Equal(c.Values, input.Values) {
		return false
	}
	if c.Threshold == nil || input.Threshold == nil {
		return c.Threshold == input.Threshold
	}
	return *c.Threshold == *input.Threshold
}

type MessageAttachmentField struct {
	Title string              `json:"title"`
	Value any                 `json:"value"`
	Short SlackCompatibleBool `json:"short"`
}

func (s *MessageAttachmentField) IsValid() error {
	var multiErr *multierror.Error

	if s.Value != nil {
		switch s.Value.(type) {
		case string, int:
			// Valid types
		default:
			multiErr = multierror.Append(multiErr, fmt.Errorf("value must be either a string or int"))
		}
	}

	return multiErr.ErrorOrNil()
}

func (s *MessageAttachmentField) Equals(input *MessageAttachmentField) bool {
	if s.Title != input.Title {
		return false
	}

	if reflect.ValueOf(input.Value).Type().Comparable() && reflect.ValueOf(s.Value).Type().Comparable() && reflect.ValueOf(input.Value).Type() == reflect.ValueOf(s.Value).Type() {
		if s.Value != input.Value {
			return false
		}
	} else {
		if !reflect.DeepEqual(s.Value, input.Value) {
			return false
		}
	}

	return s.Short == input.Short
}

func StringifyMessageAttachmentFieldValue(a []*MessageAttachment) []*MessageAttachment {
	var nonNilAttachments []*MessageAttachment
	for _, attachment := range a {
		if attachment == nil {
			continue
		}
		nonNilAttachments = append(nonNilAttachments, attachment)

		var nonNilFields []*MessageAttachmentField
		for _, field := range attachment.Fields {
			if field == nil {
				continue
			}
			nonNilFields = append(nonNilFields, field)

			if field.Value != nil {
				// Ensure the value is set to a string if it is set
				field.Value = fmt.Sprintf("%v", field.Value)
			}
		}
		attachment.Fields = nonNilFields
	}
	return nonNilAttachments
}

// This method only parses and processes the attachments,
// all else should be set in the post which is passed
func ParseMessageAttachment(post *Post, attachments []*MessageAttachment) {
	if post.Type == "" {
		post.Type = PostTypeMessageAttachment
	}

	postAttachments := []*MessageAttachment{}

	for _, attachment := range attachments {
		if attachment == nil {
			continue
		}

		attachment.Text = ParseSlackLinksToMarkdown(attachment.Text)
		attachment.Pretext = ParseSlackLinksToMarkdown(attachment.Pretext)

		for _, field := range attachment.Fields {
			if field == nil {
				continue
			}
			if value, ok := field.Value.(string); ok {
				field.Value = ParseSlackLinksToMarkdown(value)
			}
		}
		postAttachments = append(postAttachments, attachment)
	}
	post.AddProp(PostPropsAttachments, postAttachments)
}

func ParseSlackLinksToMarkdown(text string) string {
	return linkWithTextRegex.ReplaceAllString(text, "[${2}](${1})")
}
