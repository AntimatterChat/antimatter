// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {getDesktopVersion} from '@mattermost/shared/utils/user_agent';

// "Antimatter/0.9.0", "Antimatter/0.9.0-rc-1", "Antimatter/0.9.0-dev.12": a whole user-agent token
const ANTIMATTER_DESKTOP_TOKEN = /(?:^|\s)Antimatter\/(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)(?=\s|$)/;

/**
 * The desktop app's own version, to show to people (the About dialog).
 *
 * Antimatter Desktop puts its real version in an "Antimatter/<version>" user-agent token, and the version of the
 * Mattermost Desktop App it is compatible with in the "Mattermost/<version>" token that getDesktopVersion reads.
 * Version comparisons (feature checks, product notices) keep using getDesktopVersion; only display uses this.
 * Falls back to getDesktopVersion for the Mattermost Desktop App, which has no Antimatter token.
 */
export function getDisplayDesktopVersion(): string {
    const match = ANTIMATTER_DESKTOP_TOKEN.exec(window.navigator.userAgent);
    return match?.[1] || getDesktopVersion();
}
