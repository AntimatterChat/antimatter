// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {isDesktopApp} from '@mattermost/shared/utils/user_agent';

import {Permissions} from 'mattermost-redux/constants';
import {getConfig, isMarketplaceEnabled} from 'mattermost-redux/selectors/entities/general';
import {haveISystemPermission, haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUser} from 'mattermost-redux/selectors/entities/users';
import {isSystemAdmin} from 'mattermost-redux/utils/user_utils';

import {makeUrlSafe} from 'utils/url';

import type {GlobalState} from 'types/store';

// Antimatter: the entries the product menu of the classic header had, for Fusion's menus. That menu only shows when an
// app has a page of its own (none of Antimatter's does), so these must not depend on it.

// canVisitSystemConsole tells whether the System Console has a page for you: system admins, and the roles that may read
// a part of it (system manager, user manager, a custom admin role), as the product menu had it.
export function canVisitSystemConsole(state: GlobalState): boolean {
    if (isSystemAdmin(getCurrentUser(state)?.roles || '')) {
        return true;
    }
    return Permissions.SYSCONSOLE_READ_PERMISSIONS.some((permission: string) => haveISystemPermission(state, {permission}));
}

// canOpenMarketplace tells whether the App Marketplace is on and you may install apps.
export function canOpenMarketplace(state: GlobalState): boolean {
    return isMarketplaceEnabled(state) && haveITeamPermission(state, getCurrentTeamId(state), Permissions.SYSCONSOLE_WRITE_PLUGINS);
}

// appDownloadLink is the page with the apps to download, if the server has one and this isn't already the desktop app.
export function appDownloadLink(state: GlobalState): string {
    const link = getConfig(state).AppDownloadLink;
    if (!link || isDesktopApp()) {
        return '';
    }
    return makeUrlSafe(link);
}
