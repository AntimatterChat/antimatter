// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"strings"

	"github.com/mattermost/mattermost/server/public/utils"
)

const (
	// ServiceEnvironmentProduction represents the production self-managed or cloud
	// environments. This can be configured explicitly with MM_SERVICEENVIRONMENT explicitly
	// set to "production", but is also the default for any production builds.
	ServiceEnvironmentProduction = "production"
	// ServiceEnvironmentTest represents testing environments in which MM_SERVICEENVIRONMENT
	// is set explicitly to "test".
	ServiceEnvironmentTest = "test"
	// ServiceEnvironmentDev represents development environments. This can be configured
	// explicitly with MM_SERVICEENVIRONMENT set to "dev", but is also the default for any
	// non-production builds.
	ServiceEnvironmentDev = "dev"
)

// GetServiceEnvironment returns the currently configured external service environment,
// deciding which public key is used to validate enterprise licenses, which telemetry keys are
// active, and which Stripe keys are in use.
//
// To configure an environment other than default, set AM_SERVICEENVIRONMENT (or
// MM_SERVICEENVIRONMENT) before starting the application. Production builds default to ServiceEnvironmentProduction, and
// non-production builds default to ServiceEnvironmentDev.
//
// Note that this configuration is explicitly not part of the model.Config data structure, as it
// should never be persisted to the config store nor accidentally configured in any other way than
// the AM_SERVICEENVIRONMENT (or MM_SERVICEENVIRONMENT) variable.
func GetServiceEnvironment() string {
	externalServiceEnvironment := strings.TrimSpace(strings.ToLower(utils.GetServerEnv("SERVICEENVIRONMENT")))

	switch externalServiceEnvironment {
	case ServiceEnvironmentProduction, ServiceEnvironmentTest, ServiceEnvironmentDev:
		return externalServiceEnvironment
	}

	return getDefaultServiceEnvironment()
}
