// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost/server/public/model"
)

// TestClientLicense: the official desktop and mobile apps turn on their Enterprise Advanced features (they need a
// known SKU) and show the site's name as the company.
func TestClientLicense(t *testing.T) {
	license := clientLicense(model.NewPointer("Lumen Labs"))
	assert.Equal(t, "true", license["IsLicensed"])
	assert.Equal(t, "advanced", license["SkuShortName"])
	assert.Equal(t, model.LicenseShortSkuEnterpriseAdvanced, license["SkuShortName"])
	assert.Equal(t, "Lumen Labs", license["Company"])
	assert.Equal(t, "false", license["MHPNS"])
	assert.Equal(t, "false", license["Cloud"])
	assert.Equal(t, "false", license["IsTrial"])

	assert.Equal(t, "Antimatter", clientLicense(nil)["Company"])
	assert.Equal(t, "Antimatter", clientLicense(model.NewPointer("  "))["Company"])
	assert.Equal(t, "Lumen Labs", clientLicense(model.NewPointer(" Lumen Labs "))["Company"])

	// Each call gets its own copy.
	license["SkuShortName"] = "changed"
	assert.Equal(t, "advanced", clientLicense(nil)["SkuShortName"])
	assert.NotContains(t, compatClientLicense, "Company")
}
