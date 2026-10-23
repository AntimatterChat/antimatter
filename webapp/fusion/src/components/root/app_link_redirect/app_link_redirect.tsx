// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useSelector} from 'react-redux';
import {Redirect, useLocation} from 'react-router-dom';

import {getMyTeams} from 'mattermost-redux/selectors/entities/teams';

import type {GlobalState} from 'types/store';

// Antimatter: the apps that had a full page and are now apps of the side panel (Playbooks, Kanban). Their links
// (/playbooks/runs/<id>..., /kanban/team/<team>/<board>... in messages, emails and notifications) open in a community,
// under the route the app registers there (registerNeedsTeamRoute: /<team>/<app id>/...), where the app opens a
// channel with its panel on the page the link pointed to. An app that still registers a full page at one of these
// paths keeps it: its route comes first. (/boards, Kanban's name before, is sent to /kanban first.)
export const APP_LINK_ROUTES: Record<string, string> = {
    '/playbooks': 'playbooks',
    '/kanban': 'focalboard',
};

// Pages that moved to a community's own page: the AI chat's agents page (the Agents plugin's full page, where agents
// were managed) goes to the community's /integrations/agents, which sends on to where agents are made now (the System
// Console's AI settings for a system admin, the community's settings for its admins).
export const MOVED_PAGE_ROUTES: Record<string, string> = {
    '/plug/mattermost-ai/agents': 'integrations/agents',
};

export const APP_LINK_PATHS = [...Object.keys(APP_LINK_ROUTES), ...Object.keys(MOVED_PAGE_ROUTES)];

/**
 * appLinkTarget is where an app's old link opens in a community: /playbooks/runs/<id> is /<team>/playbooks/runs/<id>,
 * /kanban/team/<id>/<board> is /<team>/focalboard/team/<id>/<board>, the AI chat's agents page (and its pages) the
 * community's /integrations/agents (which sends on to the AI settings). Null for a path that isn't an app's.
 */
export function appLinkTarget(pathname: string, team: string): string | null {
    for (const [base, page] of Object.entries(MOVED_PAGE_ROUTES)) {
        if (pathname === base || pathname.startsWith(base + '/')) {
            return `/${team}/${page}`;
        }
    }
    for (const base of Object.keys(APP_LINK_ROUTES)) {
        if (pathname === base || pathname.startsWith(base + '/')) {
            return `/${team}/${APP_LINK_ROUTES[base]}${pathname.slice(base.length)}`;
        }
    }
    return null;
}

type Props = {

    // The community to open the link in (the last one viewed); the first of yours when there is none.
    teamName: string;
};

export default function AppLinkRedirect({teamName}: Props) {
    const location = useLocation();
    const firstTeamName = useSelector((state: GlobalState) => {
        const teams = getMyTeams(state).filter((team) => team.delete_at === 0);
        teams.sort((a, b) => a.display_name.localeCompare(b.display_name));
        return teams[0]?.name ?? '';
    });

    const team = teamName || firstTeamName;
    const target = team ? appLinkTarget(location.pathname, team) : null;
    if (!target) {
        return <Redirect to='/'/>;
    }

    return (
        <Redirect
            to={{
                ...location,
                pathname: target,
            }}
        />
    );
}
