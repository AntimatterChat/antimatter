// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {savePreferences} from 'mattermost-redux/actions/preferences';
import {patchTeam} from 'mattermost-redux/actions/teams';
import {getMyChannels} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import Icon from 'fusion/components/icon';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {CATEGORY_ONBOARDED, EMPTY_ONBOARDING, newOption, newQuestion, parseOnboarding} from './onboarding';
import type {Onboarding, OnboardingOption, OnboardingQuestion} from './onboarding';
import {getTeamRoles} from './roles';

// OnboardingSettings is the Onboarding tab of the community settings, where a team's admins write its onboarding: a
// welcome and questions, each answer giving roles open to all and public channels. Without questions nor welcome,
// there's no onboarding. Save saves it; Cancel takes back the changes not saved.
export default function OnboardingSettings({setUnsaved}: {setUnsaved?: (unsaved: boolean) => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const team = useSelector(getCurrentTeam);
    const me = useSelector(getCurrentUserId);
    const saved = useMemo(() => parseOnboarding(team?.onboarding) || EMPTY_ONBOARDING, [team?.onboarding]);
    const [draft, setDraft] = useState<Onboarding>(saved);
    const changed = JSON.stringify(draft) !== JSON.stringify(saved);
    useEffect(() => {
        setUnsaved?.(changed);
    }, [changed, setUnsaved]);
    useEffect(() => () => setUnsaved?.(false), [setUnsaved]);
    const roles = useSelector((state: GlobalState) => (team ? getTeamRoles(state, team.id) : []));
    const openRoles = useMemo(() => roles.filter((r) => r.self_assignable), [roles]);
    const myChannels = useSelector(getMyChannels);
    const channels = useMemo(() => myChannels.filter((c) => c.team_id === team?.id && c.type === 'O' && c.delete_at === 0).sort((a, b) => a.display_name.localeCompare(b.display_name)), [myChannels, team?.id]);

    const setQuestion = (q: OnboardingQuestion, patch: Partial<OnboardingQuestion>) => setDraft({...draft, questions: draft.questions.map((x) => (x.id === q.id ? {...x, ...patch} : x))});
    const setOption = (q: OnboardingQuestion, o: OnboardingOption, patch: Partial<OnboardingOption>) => setQuestion(q, {options: q.options.map((x) => (x.id === o.id ? {...x, ...patch} : x))});
    const toggle = (list: string[], id: string) => (list.includes(id) ? list.filter((x) => x !== id) : [...list, id]);

    const save = async () => {
        if (!team) {
            return;
        }
        const cleaned = {
            welcome: draft.welcome.trim(),
            questions: draft.questions.map((q) => ({...q, title: q.title.trim(), options: q.options.filter((o) => o.label.trim())})).filter((q) => q.title && q.options.length),
        };
        const onboarding = cleaned.welcome || cleaned.questions.length ? JSON.stringify(cleaned) : '';
        const result = await dispatch(patchTeam({id: team.id, onboarding}));
        if ('error' in result && result.error) {
            toast((result.error as {message?: string}).message || formatMessage({id: 'fusion.onboarding.failed', defaultMessage: 'The onboarding couldn\'t be saved.'}));
            return;
        }

        // Its author has seen it.
        dispatch(savePreferences(me, [{user_id: me, category: CATEGORY_ONBOARDED, name: team.id, value: 'true'}]));
        toast(formatMessage({id: 'fusion.onboarding.saved', defaultMessage: 'Onboarding saved: newcomers see it the first time they open the community'}));

        // What was saved, cleaned, is what's edited now.
        setDraft(parseOnboarding(onboarding) || EMPTY_ONBOARDING);
    };

    return (
        <div className={am('roles-pane')}>
            <p className={am('lead')}>{formatMessage({id: 'fusion.onboarding.editLead', defaultMessage: 'Newcomers see it the first time they open the community. Their answers give them roles open to all and public channels.'})}</p>
            <label className={am('field')}>
                <span>{formatMessage({id: 'fusion.onboarding.welcome', defaultMessage: 'Welcome'})}</span>
                <textarea
                    rows={3}
                    value={draft.welcome}
                    maxLength={2000}
                    onChange={(e) => setDraft({...draft, welcome: e.target.value})}
                />
            </label>
            {draft.questions.map((q) => (
                <div
                    key={q.id}
                    className={am('override-row')}
                >
                    <div className={am('override-head')}>
                        <input
                            type='text'
                            className={am('grow')}
                            value={q.title}
                            placeholder={formatMessage({id: 'fusion.onboarding.question', defaultMessage: 'Question, e.g. What brings you here?'})}
                            onChange={(e) => setQuestion(q, {title: e.target.value})}
                        />
                        <label className={am('check-row')}>
                            <input
                                type='checkbox'
                                checked={q.multiple}
                                onChange={(e) => setQuestion(q, {multiple: e.target.checked})}
                            />
                            {formatMessage({id: 'fusion.onboarding.multiple', defaultMessage: 'Several answers'})}
                        </label>
                        <button
                            type='button'
                            className={am('icon-btn')}
                            aria-label={formatMessage({id: 'fusion.onboarding.removeQuestion', defaultMessage: 'Remove the question'})}
                            onClick={() => setDraft({...draft, questions: draft.questions.filter((x) => x.id !== q.id)})}
                        >
                            <Icon
                                name='trash'
                                size='sm'
                            />
                        </button>
                    </div>
                    {q.options.map((o) => (
                        <div
                            key={o.id}
                            className={am('onboarding-edit-option')}
                        >
                            <input
                                type='text'
                                value={o.label}
                                placeholder={formatMessage({id: 'fusion.onboarding.answer', defaultMessage: 'Answer'})}
                                onChange={(e) => setOption(q, o, {label: e.target.value})}
                            />
                            <div className={am('chip-picks')}>
                                {openRoles.map((r) => (
                                    <button
                                        key={r.id}
                                        type='button'
                                        className={am('chip-pick', {on: o.role_ids.includes(r.id)})}
                                        style={r.color && o.role_ids.includes(r.id) ? {borderColor: r.color, color: r.color} : undefined}
                                        onClick={() => setOption(q, o, {role_ids: toggle(o.role_ids, r.id)})}
                                    >
                                        {r.display_name}
                                    </button>
                                ))}
                                {channels.map((c) => (
                                    <button
                                        key={c.id}
                                        type='button'
                                        className={am('chip-pick', {on: o.channel_ids.includes(c.id)})}
                                        onClick={() => setOption(q, o, {channel_ids: toggle(o.channel_ids, c.id)})}
                                    >
                                        {`#${c.display_name}`}
                                    </button>
                                ))}
                            </div>
                        </div>
                    ))}
                    <button
                        type='button'
                        className={am('btn', 'small')}
                        onClick={() => setQuestion(q, {options: [...q.options, newOption()]})}
                    >
                        {formatMessage({id: 'fusion.onboarding.addAnswer', defaultMessage: 'Add an answer'})}
                    </button>
                </div>
            ))}
            {!openRoles.length && <p className={am('lead')}>{formatMessage({id: 'fusion.onboarding.noOpenRoles', defaultMessage: 'Answers can give roles that anyone can take: mark some so in Roles.'})}</p>}
            <div className={am('modal-actions')}>
                <button
                    type='button'
                    className={am('btn')}
                    onClick={() => setDraft({...draft, questions: [...draft.questions, newQuestion()]})}
                >
                    {formatMessage({id: 'fusion.onboarding.addQuestion', defaultMessage: 'Add a question'})}
                </button>
                <button
                    type='button'
                    className={am('btn')}
                    disabled={!changed}
                    onClick={() => setDraft(saved)}
                >
                    {formatMessage({id: 'fusion.onboarding.cancel', defaultMessage: 'Cancel'})}
                </button>
                <button
                    type='button'
                    className={am('btn', 'primary')}
                    disabled={!changed}
                    onClick={save}
                >
                    {formatMessage({id: 'fusion.onboarding.save', defaultMessage: 'Save'})}
                </button>
            </div>
        </div>
    );
}
