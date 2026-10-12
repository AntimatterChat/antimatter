// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import {Dialog} from 'fusion/components/layer';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';

import {TIMEOUT_DURATIONS, timeOutMember} from './moderation';

// TimeoutDialog times someone out of the team, as Discord's: how long, and why.
export default function TimeoutDialog({userId, onClose}: {userId: string; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const team = useSelector(getCurrentTeam);
    const name = useDisplayName(useUser(userId));
    const [duration, setDuration] = useState(TIMEOUT_DURATIONS[0].ms);
    const [reason, setReason] = useState('');
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');

    const submit = async () => {
        if (!team || busy) {
            return;
        }
        setBusy(true);
        const result = await dispatch(timeOutMember(team.id, userId, Date.now() + duration, reason.trim()));
        setBusy(false);
        if ('error' in result && result.error) {
            setError((result.error as {message?: string}).message || formatMessage({id: 'fusion.timeout.failed', defaultMessage: 'The timeout couldn\'t be set.'}));
            return;
        }
        toast(formatMessage({id: 'fusion.timeout.done', defaultMessage: '{name} is timed out'}, {name}));
        onClose();
    };

    return (
        <Dialog
            label={formatMessage({id: 'fusion.timeout.title', defaultMessage: 'Time out {name}'}, {name})}
            onClose={onClose}
            onSubmit={submit}
        >
            <div className={am('pane')}>
                <h2>{formatMessage({id: 'fusion.timeout.title', defaultMessage: 'Time out {name}'}, {name})}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.timeout.lead', defaultMessage: 'They can read the community\'s channels, but not send messages, react, attach files or mention, until the timeout ends. Direct messages stay open.'})}</p>
                <div
                    className={am('timeout-durations')}
                    role='radiogroup'
                    aria-label={formatMessage({id: 'fusion.timeout.duration', defaultMessage: 'Duration'})}
                >
                    {TIMEOUT_DURATIONS.map((d) => (
                        <button
                            key={d.ms}
                            type='button'
                            role='radio'
                            aria-checked={duration === d.ms}
                            className={am('onboarding-option', {on: duration === d.ms})}
                            onClick={() => setDuration(d.ms)}
                        >
                            {formatMessage(d.label)}
                        </button>
                    ))}
                </div>
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.timeout.reason', defaultMessage: 'Reason (optional)'})}</span>
                    <textarea
                        rows={2}
                        value={reason}
                        maxLength={512}
                        onChange={(e) => setReason(e.target.value)}
                    />
                </label>
                {error && (
                    <p
                        className={am('lead')}
                        role='alert'
                        style={{color: 'var(--am-danger)'}}
                    >
                        {error}
                    </p>
                )}
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.timeout.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button
                        type='submit'
                        className={am('btn', 'danger')}
                        disabled={busy}
                    >
                        {formatMessage({id: 'fusion.timeout.submit', defaultMessage: 'Time out'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
