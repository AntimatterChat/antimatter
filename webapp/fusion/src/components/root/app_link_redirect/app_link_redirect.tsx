// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useSelector} from 'react-redux';
import {Redirect, useLocation} from 'react-router-dom';

import {getMyTeams} from 'mattermost-redux/selectors/entities/teams';

import type {GlobalState} from 'types/store';

// Antimatter: the apps that had a full page and are now apps of the side panel (Playbooks). Their links
// (/playbooks/runs/<id>... in messages, emails and notifications) open in a community, /<team>/playbooks/..., where
// the app opens a channel with its panel on the page the link pointed to. An app that still registers a full page
// at one of these paths keeps it: its route comes first.
export const APP_LINK_PATHS = ['/playbooks'];

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
    if (!team) {
        return <Redirect to='/'/>;
    }

    return (
        <Redirect
            to={{
                ...location,
                pathname: `/${team}${location.pathname}`,
            }}
        />
    );
}
