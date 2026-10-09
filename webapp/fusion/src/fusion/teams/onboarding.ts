// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {joinChannel} from 'mattermost-redux/actions/channels';
import {addUsersToGroup} from 'mattermost-redux/actions/groups';
import {savePreferences} from 'mattermost-redux/actions/preferences';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import type {ActionFuncAsync} from 'types/store';

import {loadTeams} from './teams';

// A team's onboarding, as Discord's: a welcome, then questions whose answers give people roles (open to all) and
// channels (public ones). Kept as JSON on the team; each person answers it once per team.
export type OnboardingOption = {id: string; label: string; role_ids: string[]; channel_ids: string[]};
export type OnboardingQuestion = {id: string; title: string; multiple: boolean; options: OnboardingOption[]};
export type Onboarding = {welcome: string; questions: OnboardingQuestion[]};

// The teams each person went through the onboarding of (or skipped it): one preference per team.
export const CATEGORY_ONBOARDED = 'fusion_onboarded';

export const EMPTY_ONBOARDING: Onboarding = {welcome: '', questions: []};

const newId = () => Math.random().toString(36).slice(2, 10);

export function newQuestion(): OnboardingQuestion {
    return {id: newId(), title: '', multiple: true, options: [newOption()]};
}

export function newOption(): OnboardingOption {
    return {id: newId(), label: '', role_ids: [], channel_ids: []};
}

// parseOnboarding reads a team's onboarding, keeping only what's well formed; undefined when there's none.
export function parseOnboarding(json: string | undefined): Onboarding | undefined {
    if (!json) {
        return undefined;
    }
    try {
        const raw = JSON.parse(json);
        const strings = (v: unknown) => (Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : []);
        const questions: OnboardingQuestion[] = (Array.isArray(raw.questions) ? raw.questions : []).map((q: Record<string, unknown>) => ({
            id: String(q.id || newId()),
            title: String(q.title || ''),
            multiple: q.multiple !== false,
            options: (Array.isArray(q.options) ? q.options : []).map((o: Record<string, unknown>) => ({
                id: String(o.id || newId()),
                label: String(o.label || ''),
                role_ids: strings(o.role_ids),
                channel_ids: strings(o.channel_ids),
            })).filter((o: OnboardingOption) => o.label),
        })).filter((q: OnboardingQuestion) => q.title && q.options.length);
        const welcome = typeof raw.welcome === 'string' ? raw.welcome : '';
        return welcome || questions.length ? {welcome, questions} : undefined;
    } catch {
        return undefined;
    }
}

// chosenGrants gathers the roles and channels of the options chosen.
export function chosenGrants(onboarding: Onboarding, chosen: string[]): {roleIds: string[]; channelIds: string[]} {
    const options = onboarding.questions.flatMap((q) => q.options).filter((o) => chosen.includes(o.id));
    return {
        roleIds: [...new Set(options.flatMap((o) => o.role_ids))],
        channelIds: [...new Set(options.flatMap((o) => o.channel_ids))],
    };
}

// finishOnboarding gives someone what they chose, and remembers they went through the team's onboarding.
export function finishOnboarding(teamId: string, onboarding: Onboarding | undefined, chosen: string[]): ActionFuncAsync {
    return async (dispatch, getState) => {
        const me = getCurrentUserId(getState());
        if (onboarding) {
            const {roleIds, channelIds} = chosenGrants(onboarding, chosen);
            await Promise.all([
                ...roleIds.map((id) => dispatch(addUsersToGroup(id, [me]))),
                ...channelIds.map((id) => dispatch(joinChannel(me, teamId, id))),
            ]);
            if (roleIds.length) {
                dispatch(loadTeams());
            }
        }
        await dispatch(savePreferences(me, [{user_id: me, category: CATEGORY_ONBOARDED, name: teamId, value: 'true'}]));
        return {data: true};
    };
}
