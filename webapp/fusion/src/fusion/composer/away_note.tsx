// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo, useState} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useSelector} from 'react-redux';

import {getChannel, getDirectTeammate} from 'mattermost-redux/selectors/entities/channels';
import {getTeammateNameDisplaySetting} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentUserId, getDndEndTimeForUserId, getStatusForUserId, makeGetProfilesInChannel} from 'mattermost-redux/selectors/entities/users';
import {displayUsername} from 'mattermost-redux/utils/user_utils';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';
import Constants, {UserStatuses} from 'utils/constants';

import type {GlobalState} from 'types/store';

// The conversations where the note was dismissed, until the page reloads, as the sleep note.
const dismissed = new Set<string>();

type Away = {name: string; status: string; until: number};

// AwayNote warns, above the message box of a direct or group message, that the others won't see it right away: they
// turned Do Not Disturb on (until when), or are out of office.
export default function AwayNote({channelId}: {channelId: string}) {
    const {formatMessage, formatDate} = useIntl();
    const getProfiles = useMemo(() => makeGetProfilesInChannel(), []);
    const channel = useSelector((state: GlobalState) => getChannel(state, channelId));
    const me = useSelector(getCurrentUserId);
    const nameDisplay = useSelector(getTeammateNameDisplaySetting);
    const [, setDismissed] = useState(0);

    // As "status|until|name" strings, which compare equal from one render to the next.
    const awayKeys = useSelector((state: GlobalState): string[] => {
        const teammate = getDirectTeammate(state, channelId);
        const others = teammate ? [teammate].filter((u) => u.id !== me) : getProfiles(state, channelId, {active: true}).filter((u) => u.id !== me);
        return others.
            filter((u) => !u.is_bot).
            map((u) => [getStatusForUserId(state, u.id), getDndEndTimeForUserId(state, u.id) || 0, u.first_name || displayUsername(u, nameDisplay)].join('|')).
            filter((key) => key.startsWith(UserStatuses.DND + '|') || key.startsWith(UserStatuses.OUT_OF_OFFICE + '|'));
    }, shallowEqual);
    const away: Away[] = awayKeys.map((key) => {
        const [status, until, ...name] = key.split('|');
        return {status, until: Number(until), name: name.join('|')};
    });

    if (!channel || (channel.type !== Constants.DM_CHANNEL && channel.type !== Constants.GM_CHANNEL) || dismissed.has(channelId) || !away.length) {
        return null;
    }

    const dnd = away.filter((a) => a.status === UserStatuses.DND);
    const ooo = away.filter((a) => a.status === UserStatuses.OUT_OF_OFFICE);
    const names = (list: Away[]) => list.map((a) => a.name).join(', ');
    const parts = [];
    if (dnd.length === 1 && dnd[0].until > 0) {
        parts.push(formatMessage(
            {id: 'fusion.away.dndUntil', defaultMessage: '{name} has Do Not Disturb on until {time}, so won\'t be notified.'},
            {name: dnd[0].name, time: formatDate(dnd[0].until * 1000, {weekday: 'short', hour: 'numeric', minute: '2-digit'})},
        ));
    } else if (dnd.length) {
        parts.push(formatMessage(
            {id: 'fusion.away.dnd', defaultMessage: '{names} {count, plural, one {has} other {have}} Do Not Disturb on, so won\'t be notified.'},
            {names: names(dnd), count: dnd.length},
        ));
    }
    if (ooo.length) {
        parts.push(formatMessage(
            {id: 'fusion.away.ooo', defaultMessage: '{names} {count, plural, one {is} other {are}} out of office.'},
            {names: names(ooo), count: ooo.length},
        ));
    }

    return (
        <div
            className={am('sleep-note', 'away-note')}
            role='note'
        >
            <Icon
                name={dnd.length ? 'bell-off' : 'leave'}
                size='sm'
            />
            <span>{parts.join(' ')}</span>
            <button
                type='button'
                className={am('sleep-x')}
                aria-label={formatMessage({id: 'fusion.sleep.dismiss', defaultMessage: 'Dismiss'})}
                title={formatMessage({id: 'fusion.sleep.dismiss', defaultMessage: 'Dismiss'})}
                onClick={() => {
                    dismissed.add(channelId);
                    setDismissed((n) => n + 1);
                }}
            >
                <Icon
                    name='x'
                    size='xs'
                />
            </button>
        </div>
    );
}
