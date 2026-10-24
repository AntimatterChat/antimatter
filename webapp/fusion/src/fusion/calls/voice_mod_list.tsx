// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {getMissingProfilesByIds} from 'mattermost-redux/actions/users';

import Avatar from 'fusion/components/avatar';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {Section} from 'fusion/modals/settings_controls';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {useCallActions} from './actions';
import {getCallsAPI, getModDeafens, getModMutes, isVoiceChannel, useCallsAPIs} from './calls_api';
import {canModerateCallsIn} from './hooks';
import type {CallsLastingMute} from './plugin_api/calls_public_api';

// canListVoiceMutes tells whether the user sees the lasting mutes and deafens of a voice channel in its settings, as
// Calls lets them list them: those who may moderate its calls (not as a call's host).
export function canListVoiceMutes(state: GlobalState, channelId: string): boolean {
    return Boolean(getCallsAPI()?.moderation?.listModMutes) && isVoiceChannel(state, channelId) && canModerateCallsIn(state, channelId);
}

export function useCanListVoiceMutes(channelId: string): boolean {
    useCallsAPIs();
    return useSelector((state: GlobalState) => canListVoiceMutes(state, channelId));
}

function ByLine({userId, at, muted}: {userId: string; at: number; muted: boolean}) {
    const {formatMessage, formatDate} = useIntl();
    const name = useDisplayName(useUser(userId)) || userId;
    const date = formatDate(at, {dateStyle: 'medium', timeStyle: 'short'});
    return (
        <span>
            {muted ? formatMessage({id: 'fusion.calls.modList.mutedBy', defaultMessage: 'Muted by {name} on {date}'}, {name, date}) : formatMessage({id: 'fusion.calls.modList.deafenedBy', defaultMessage: 'Deafened by {name} on {date}'}, {name, date})}
        </span>
    );
}

function MuteRow({entry, channelId, onLifted}: {entry: CallsLastingMute; channelId: string; onLifted: () => void}) {
    const {formatMessage} = useIntl();
    const actions = useCallActions();
    const name = useDisplayName(useUser(entry.userId));
    const outranked = formatMessage({id: 'fusion.calls.modList.outranked', defaultMessage: 'Given by someone above you: they, or someone above them, can lift it'});
    const lift = (what: 'mute' | 'deafen') => async () => {
        await (what === 'mute' ? actions.modMute(entry, channelId, false) : actions.modDeafen(entry, channelId, false));
        onLifted();
    };
    return (
        <div className={am('toggle-row', 'person-row', 'voice-mod-row')}>
            <Avatar
                userId={entry.userId}
                size='sm'
            />
            <div className={am('txt')}>
                <b>{name || entry.userId}</b>
                {entry.mutedBy && (
                    <ByLine
                        userId={entry.mutedBy}
                        at={entry.mutedAt}
                        muted={true}
                    />
                )}
                {entry.deafenedBy && (
                    <ByLine
                        userId={entry.deafenedBy}
                        at={entry.deafenedAt}
                        muted={false}
                    />
                )}
            </div>
            <div className={am('request-actions')}>
                {entry.mutedBy && (
                    <button
                        type='button'
                        className={am('btn')}
                        disabled={!entry.canLiftMute}
                        title={entry.canLiftMute ? undefined : outranked}
                        onClick={lift('mute')}
                    >
                        {formatMessage({id: 'fusion.calls.menu.liftModMute', defaultMessage: 'Allow to unmute'})}
                    </button>
                )}
                {entry.deafenedBy && (
                    <button
                        type='button'
                        className={am('btn')}
                        disabled={!entry.canLiftDeafen}
                        title={entry.canLiftDeafen ? undefined : outranked}
                        onClick={lift('deafen')}
                    >
                        {formatMessage({id: 'fusion.calls.menu.modUndeafen', defaultMessage: 'Undeafen'})}
                    </button>
                )}
            </div>
        </div>
    );
}

// VoiceModerationSection is the voice channel's settings tab that lists who a moderator muted or deafened
// there (it lasts until lifted, also while they're away), who did and since when, with a button to lift each, for
// those who may moderate the channel's calls (see canListVoiceMutes). Lifting follows the participant menu's rule:
// only who ranks at least as high as whoever gave it; the others' buttons are disabled, with why.
export default function VoiceModerationSection({channelId}: {channelId: string}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const [list, setList] = useState<CallsLastingMute[] | null>(null);
    const [failed, setFailed] = useState(false);
    const [version, setVersion] = useState(0);

    // The moderator mutes and deafens Calls tells of, as they change: the list is loaded again then.
    const mutes = useSelector((state: GlobalState) => getModMutes(state, channelId));
    const deafens = useSelector((state: GlobalState) => getModDeafens(state, channelId));

    useEffect(() => {
        const listModMutes = getCallsAPI()?.moderation?.listModMutes;
        if (!listModMutes) {
            return undefined;
        }
        let active = true;
        listModMutes(channelId).then((result) => {
            if (active) {
                setList(result);
                setFailed(false);
            }
        }).catch(() => {
            if (active) {
                setFailed(true);
            }
        });
        return () => {
            active = false;
        };
    }, [channelId, mutes, deafens, version]);

    useEffect(() => {
        const ids = new Set<string>();
        for (const entry of list || []) {
            [entry.userId, entry.mutedBy, entry.deafenedBy].forEach((id) => id && ids.add(id));
        }
        if (ids.size) {
            dispatch(getMissingProfilesByIds([...ids]));
        }
    }, [list, dispatch]);

    let body: React.ReactNode = null;
    if (failed) {
        body = <div className={am('empty')}>{formatMessage({id: 'fusion.calls.modList.failed', defaultMessage: 'The list couldn\'t be loaded.'})}</div>;
    } else if (list && !list.length) {
        body = <div className={am('empty')}>{formatMessage({id: 'fusion.calls.modList.none', defaultMessage: 'Nobody is muted or deafened here.'})}</div>;
    } else if (list) {
        body = list.map((entry) => (
            <MuteRow
                key={entry.userId}
                entry={entry}
                channelId={channelId}
                onLifted={() => setVersion((v) => v + 1)}
            />
        ));
    }

    return (
        <section aria-label={formatMessage({id: 'fusion.calls.modList.title', defaultMessage: 'Muted and deafened'})}>
            <Section title={formatMessage({id: 'fusion.calls.modList.title', defaultMessage: 'Muted and deafened'})}/>
            <p className={am('lead')}>{formatMessage({id: 'fusion.calls.modList.lead', defaultMessage: 'People a moderator muted or deafened in this voice channel. It lasts until lifted, also when they leave and come back.'})}</p>
            {body}
        </section>
    );
}
