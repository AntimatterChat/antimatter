// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

// IsSpoiler tells whether the post is a spoiler: clients hide its text and files until the reader chooses to see them,
// and notifications and quotes of it leave its text out.
func (o *Post) IsSpoiler() bool {
	switch v := o.GetProp(PostPropsSpoiler).(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}
