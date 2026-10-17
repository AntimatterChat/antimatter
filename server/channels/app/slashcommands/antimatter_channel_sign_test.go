// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost/server/public/shared/i18n"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

// Antimatter writes channels #channel-name: the built-in commands that take a channel say so in their hints (they
// still accept ~channel too).
func TestChannelCommandHintsUseHash(t *testing.T) {
	th := setup(t)
	T := i18n.GetUserTranslations("en")

	for _, trigger := range []string{CmdJoin, CmdMute, CmdInvite} {
		provider := app.GetCommandProvider(trigger)
		if !assert.NotNil(t, provider, trigger) {
			continue
		}
		hint := provider.GetCommand(th.App, T).AutoCompleteHint
		assert.Contains(t, hint, "#[channel]", "/%s", trigger)
		assert.NotContains(t, hint, "~", "/%s", trigger)
	}
}
