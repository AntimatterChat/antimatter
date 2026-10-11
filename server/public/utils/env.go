// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package utils

import "os"

const (
	// EnvPrefix is the prefix of the server's environment variables, as in AM_SQLSETTINGS_DATASOURCE.
	EnvPrefix = "AM_"
	// LegacyEnvPrefix is the Mattermost prefix, still accepted so existing MM_* setups keep working.
	LegacyEnvPrefix = "MM_"
)

// LookupServerEnv looks up the server environment variable called name, given without its
// prefix: AM_<name> is used when it is set, even to an empty value, otherwise MM_<name>.
func LookupServerEnv(name string) (string, bool) {
	if value, ok := os.LookupEnv(EnvPrefix + name); ok {
		return value, true
	}
	return os.LookupEnv(LegacyEnvPrefix + name)
}

// GetServerEnv works like LookupServerEnv but returns an empty string when neither
// AM_<name> nor MM_<name> is set.
func GetServerEnv(name string) string {
	value, _ := LookupServerEnv(name)
	return value
}
