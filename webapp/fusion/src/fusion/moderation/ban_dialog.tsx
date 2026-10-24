// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import {Client4} from 'mattermost-redux/client';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import {Dialog} from 'fusion/components/layer';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {Select} from 'fusion/modals/settings_controls';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';

import {BAN_DURATIONS, BAN_PURGE_WINDOWS} from './moderation';

// BanDialog bans someone from the community, as Discord's: for how long, how far back their messages there are
// deleted, and why. The server removes them from the community and all its channels; the ban list is in Community
// settings › Members, where it's lifted.
export default function BanDialog({userId, onClose}: {userId: string; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const toast = useToast();
    const team = useSelector(getCurrentTeam);
    const name = useDisplayName(useUser(userId));
    const [duration, setDuration] = useState(String(BAN_DURATIONS[0].ms));
    const [purge, setPurge] = useState(String(BAN_PURGE_WINDOWS[0].seconds));
    const [reason, setReason] = useState('');
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');

    const submit = async () => {
        if (!team || busy) {
            return;
        }
        setBusy(true);
        const ms = Number(duration);
        try {
            const result = await Client4.banTeamMember(team.id, userId, {
                reason: reason.trim(),
                expire_at: ms ? Date.now() + ms : 0,
                delete_messages_seconds: Number(purge),
            });
            toast(result.purge_job_id ? formatMessage({id: 'fusion.ban.doneDeleting', defaultMessage: '{name} was banned from {team}. Their messages are being deleted.'}, {name, team: team.display_name}) : formatMessage({id: 'fusion.ban.done', defaultMessage: '{name} was banned from {team}'}, {name, team: team.display_name}));
            onClose();
        } catch (err) {
            setBusy(false);
            setError((err as {message?: string}).message || formatMessage({id: 'fusion.ban.failed', defaultMessage: 'The ban couldn\'t be saved.'}));
        }
    };

    return (
        <Dialog
            label={formatMessage({id: 'fusion.ban.title', defaultMessage: 'Ban {name} from {team}?'}, {name, team: team?.display_name})}
            onClose={onClose}
            onSubmit={submit}
        >
            <div className={am('pane')}>
                <h2>{formatMessage({id: 'fusion.ban.title', defaultMessage: 'Ban {name} from {team}?'}, {name, team: team?.display_name})}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.ban.lead', defaultMessage: '{name} leaves every channel of the community and can\'t come back, by an invite, a link or joining, until the ban ends or is lifted in Community settings › Members.'}, {name})}</p>
                <Select
                    title={formatMessage({id: 'fusion.ban.duration', defaultMessage: 'Duration'})}
                    value={duration}
                    options={BAN_DURATIONS.map((d): [string, string] => [String(d.ms), formatMessage(d.label)])}
                    onChange={setDuration}
                />
                <Select
                    title={formatMessage({id: 'fusion.ban.purge', defaultMessage: 'Delete their messages'})}
                    desc={formatMessage({id: 'fusion.ban.purgeDesc', defaultMessage: 'In this community only, with the replies to them.'})}
                    value={purge}
                    options={BAN_PURGE_WINDOWS.map((w): [string, string] => [String(w.seconds), formatMessage(w.label)])}
                    onChange={setPurge}
                />
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.ban.reason', defaultMessage: 'Reason (optional, seen by moderators)'})}</span>
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
                        {formatMessage({id: 'fusion.ban.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button
                        type='submit'
                        className={am('btn', 'danger')}
                        disabled={busy}
                    >
                        {formatMessage({id: 'fusion.ban.submit', defaultMessage: 'Ban'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
