// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useDispatch, useSelector, useStore} from 'react-redux';

import {savePreferences} from 'mattermost-redux/actions/preferences';
import {get as getPreference} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {isTeamAdmin} from 'fusion/sidebar/server_categories';

import type {GlobalState} from 'types/store';

import {CATEGORY_ONBOARDED, parseOnboarding} from './onboarding';
import OnboardingDialog from './onboarding_dialog';
import TeamLayoutDialog from './team_layout_dialog';
import {isTeamSetUp} from './templates';

// The teams each admin was offered a layout for, once.
const CATEGORY_LAYOUT_OFFERED = 'fusion_layout_offered';

// How long to let a team's roles and categories load before telling it isn't set up.
const SETTLE_MS = 2000;

// TeamSetupHost greets people in a team: its admins with a layout to start from, once, when the team has neither
// categories for everyone nor roles; everyone with the team's onboarding, the first time they open it.
export default function TeamSetupHost() {
    const dispatch = useDispatch();
    const store = useStore<GlobalState>();
    const team = useSelector(getCurrentTeam);
    const me = useSelector(getCurrentUserId);
    const onboarding = parseOnboarding(team?.onboarding);
    const onboarded = useSelector((state: GlobalState) => Boolean(team && getPreference(state, CATEGORY_ONBOARDED, team.id, '')));
    const [offering, setOffering] = useState(false);
    const [skipped, setSkipped] = useState<string | null>(null);

    useEffect(() => {
        setOffering(false);
        const teamId = team?.id;
        if (!teamId) {
            return undefined;
        }
        const timer = setTimeout(() => {
            const state = store.getState();
            const loaded = state.entities.channelCategories.serverCategories?.[teamId] !== undefined;
            const offered = getPreference(state, CATEGORY_LAYOUT_OFFERED, teamId, '');
            if (loaded && !offered && isTeamAdmin(state, teamId) && !isTeamSetUp(state, teamId)) {
                setOffering(true);
                dispatch(savePreferences(me, [{user_id: me, category: CATEGORY_LAYOUT_OFFERED, name: teamId, value: 'true'}]));
            }
        }, SETTLE_MS);
        return () => clearTimeout(timer);
    }, [team?.id, me, dispatch, store]);

    if (offering) {
        return (
            <TeamLayoutDialog
                first={true}
                onClose={() => setOffering(false)}
            />
        );
    }
    if (team && onboarding && !onboarded && skipped !== team.id) {
        return (
            <OnboardingDialog
                team={team}
                onboarding={onboarding}
                onClose={() => setSkipped(team.id)}
            />
        );
    }
    return null;
}
