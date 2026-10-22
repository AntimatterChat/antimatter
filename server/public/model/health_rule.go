// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

type HealthArea string

const (
	AreaAuth          HealthArea = "auth"
	AreaDatabase      HealthArea = "database"
	AreaSearch        HealthArea = "search"
	AreaJobs          HealthArea = "jobs"
	AreaCluster       HealthArea = "cluster"
	AreaNotifications HealthArea = "notifications"
	AreaCompliance    HealthArea = "compliance"
	AreaPlatform      HealthArea = "platform"
)

func AllHealthAreas() []HealthArea {
	return []HealthArea{
		AreaAuth,
		AreaDatabase,
		AreaSearch,
		AreaJobs,
		AreaCluster,
		AreaNotifications,
		AreaCompliance,
		AreaPlatform,
	}
}

type RuleText struct {
	TitleID       string `json:"title_id"`
	RemediationID string `json:"remediation_id"`
	DocsURL       string `json:"docs_url,omitempty"`
	ConsolePath   string `json:"console_path,omitempty"`
}
