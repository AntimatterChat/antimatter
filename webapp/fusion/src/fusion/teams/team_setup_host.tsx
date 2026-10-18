// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useDispatch, useSelector, useStore} from 'react-redux';

import {fetchServerCategories} from 'mattermost-redux/actions/channel_categories';
import {savePreferences} from 'mattermost-redux/actions/preferences';
import {get as getPreference, isCustomGroupsEnabled} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {isTeamAdmin} from 'fusion/sidebar/server_categories';

import type {GlobalState} from 'types/store';

import {getCommunityRules} from './community_rules';
import {CommunityRulesDialog} from './community_rules_dialog';
import {CATEGORY_ONBOARDED, parseOnboarding} from './onboarding';
import OnboardingDialog from './onboarding_dialog';
import TeamLayoutDialog from './team_layout_dialog';
import {loadTeams} from './teams';
import {isTeamSetUp} from './templates';

// The teams each admin was offered a layout for, once.
const CATEGORY_LAYOUT_OFFERED = 'fusion_layout_offered';

// TeamSetupHost greets people in a team: its admins with a layout to start from, once, when the team has neither
// categories for everyone nor roles; everyone with the team's rules to agree to, when it has some they haven't agreed
// to (on joining it, and each time its admins ask everyone to agree again), then with its onboarding, the first time
// they open it.
export default function TeamSetupHost() {
    const dispatch = useDispatch();
    const store = useStore<GlobalState>();
    const team = useSelector(getCurrentTeam);
    const me = useSelector(getCurrentUserId);
    const onboarding = parseOnboarding(team?.onboarding);
    const onboarded = useSelector((state: GlobalState) => Boolean(team && getPreference(state, CATEGORY_ONBOARDED, team.id, '')));
    const [offering, setOffering] = useState(false);
    const [skipped, setSkipped] = useState<string | null>(null);

    // The rules are offered once per version in a session: put off ("Not now"), the message box's notice offers them.
    const rules = useSelector((state: GlobalState) => (team ? getCommunityRules(state, team.id) : undefined));
    const rulesKey = rules ? `${rules.team_id}:${rules.version}` : '';
    const [rulesSeen, setRulesSeen] = useState<string[]>([]);
    const customGroups = useSelector(isCustomGroupsEnabled);

    // Whether the team is set up is told once its roles and categories are loaded, not before.
    useEffect(() => {
        setOffering(false);
        const teamId = team?.id;
        const state = store.getState();
        if (!teamId || !customGroups || getPreference(state, CATEGORY_LAYOUT_OFFERED, teamId, '') || !isTeamAdmin(state, teamId)) {
            return undefined;
        }
        let cancelled = false;
        Promise.all([dispatch(loadTeams()), dispatch(fetchServerCategories(teamId))]).then((results) => {
            const failed = results.some((r) => (r as {error?: unknown})?.error);
            if (cancelled || failed || isTeamSetUp(store.getState(), teamId)) {
                return;
            }
            setOffering(true);
            dispatch(savePreferences(me, [{user_id: me, category: CATEGORY_LAYOUT_OFFERED, name: teamId, value: 'true'}]));
        });
        return () => {
            cancelled = true;
        };
    }, [team?.id, me, customGroups, dispatch, store]);

    if (offering) {
        return (
            <TeamLayoutDialog
                first={true}
                onClose={() => setOffering(false)}
            />
        );
    }
    if (team && rules?.must_accept && !rulesSeen.includes(rulesKey)) {
        return (
            <CommunityRulesDialog
                teamId={team.id}
                onClose={() => setRulesSeen([...rulesSeen, rulesKey])}
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
