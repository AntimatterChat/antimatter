// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch} from 'react-redux';

import type {Team} from '@mattermost/types/teams';

import {Dialog} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';

import {finishOnboarding} from './onboarding';
import type {Onboarding} from './onboarding';

// OnboardingDialog welcomes someone to a team the first time they open it, as Discord's onboarding: the team's
// questions, whose answers give them roles and channels.
export default function OnboardingDialog({team, onboarding, onClose}: {team: Team; onboarding: Onboarding; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const [chosen, setChosen] = useState<string[]>([]);
    const [saving, setSaving] = useState(false);

    const pick = (questionMultiple: boolean, questionOptions: string[], id: string) => {
        if (chosen.includes(id)) {
            setChosen(chosen.filter((c) => c !== id));
        } else if (questionMultiple) {
            setChosen([...chosen, id]);
        } else {
            setChosen([...chosen.filter((c) => !questionOptions.includes(c)), id]);
        }
    };

    const finish = async (answers: string[]) => {
        setSaving(true);
        await dispatch(finishOnboarding(team.id, onboarding, answers));
        onClose();
    };

    return (
        <Dialog
            label={formatMessage({id: 'fusion.onboarding.title', defaultMessage: 'Welcome to {team}'}, {team: team.display_name})}
            onClose={() => finish([])}
            small={false}
            className={am('wide')}
        >
            <div className={am('pane', 'roles-pane', 'onboarding')}>
                <h2>{formatMessage({id: 'fusion.onboarding.title', defaultMessage: 'Welcome to {team}'}, {team: team.display_name})}</h2>
                {onboarding.welcome && <p className={am('lead')}>{onboarding.welcome}</p>}
                {onboarding.questions.map((q) => (
                    <div
                        key={q.id}
                        className={am('onboarding-question')}
                    >
                        <h3 className={am('section-title')}>{q.title}</h3>
                        <div
                            className={am('onboarding-options')}
                            role={q.multiple ? 'group' : 'radiogroup'}
                            aria-label={q.title}
                        >
                            {q.options.map((o) => (
                                <button
                                    key={o.id}
                                    type='button'
                                    role={q.multiple ? 'checkbox' : 'radio'}
                                    aria-checked={chosen.includes(o.id)}
                                    className={am('onboarding-option', {on: chosen.includes(o.id)})}
                                    onClick={() => pick(q.multiple, q.options.map((x) => x.id), o.id)}
                                >
                                    {o.label}
                                </button>
                            ))}
                        </div>
                    </div>
                ))}
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        disabled={saving}
                        onClick={() => finish([])}
                    >
                        {formatMessage({id: 'fusion.onboarding.skip', defaultMessage: 'Skip'})}
                    </button>
                    <button
                        type='button'
                        className={am('btn', 'primary')}
                        disabled={saving}
                        onClick={() => finish(chosen)}
                    >
                        {formatMessage({id: 'fusion.onboarding.done', defaultMessage: 'Done'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
