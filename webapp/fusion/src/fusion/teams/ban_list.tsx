// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch} from 'react-redux';

import type {MemberBan} from '@mattermost/types/moderation';

import {getMissingProfilesByIds} from 'mattermost-redux/actions/users';
import {Client4} from 'mattermost-redux/client';

import Avatar from 'fusion/components/avatar';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {Section} from 'fusion/modals/settings_controls';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';

// BanRow is someone banned from the community: who, who banned them and when, until when, why; and lifting it.
function BanRow({ban, onLifted}: {ban: MemberBan; onLifted: (userId: string) => void}) {
    const {formatMessage, formatDate} = useIntl();
    const toast = useToast();
    const user = useUser(ban.user_id);
    const name = useDisplayName(user);
    const by = useDisplayName(useUser(ban.created_by));
    const [busy, setBusy] = useState(false);
    const date = (at: number) => formatDate(at, {year: 'numeric', month: 'short', day: 'numeric'});

    const lift = async () => {
        setBusy(true);
        try {
            await Client4.unbanTeamMember(ban.team_id, ban.user_id);
            toast(formatMessage({id: 'fusion.bans.lifted', defaultMessage: '{name}\'s ban was lifted'}, {name}));
            onLifted(ban.user_id);
        } catch (error) {
            setBusy(false);
            toast((error as {message?: string}).message || formatMessage({id: 'fusion.bans.liftFailed', defaultMessage: 'The ban couldn\'t be lifted.'}));
        }
    };

    return (
        <div
            className={am('toggle-row', 'person-row', 'ban-row')}
            data-user={ban.user_id}
        >
            <Avatar
                userId={ban.user_id}
                size='sm'
            />
            <div className={am('txt')}>
                <b>{name || ban.user_id}</b>
                <span>
                    {[
                        user ? '@' + user.username : '',
                        formatMessage({id: 'fusion.bans.by', defaultMessage: 'Banned by {name} on {date}'}, {name: by || ban.created_by, date: date(ban.create_at)}),
                        ban.expire_at ? formatMessage({id: 'fusion.bans.until', defaultMessage: 'until {date}'}, {date: date(ban.expire_at)}) : formatMessage({id: 'fusion.bans.untilLifted', defaultMessage: 'until it\'s lifted'}),
                    ].filter(Boolean).join(' · ')}
                </span>
                {ban.reason && <span className={am('ban-reason')}>{ban.reason}</span>}
            </div>
            <button
                type='button'
                className={am('btn')}
                disabled={busy}
                onClick={lift}
            >
                {formatMessage({id: 'fusion.bans.lift', defaultMessage: 'Lift ban'})}
            </button>
        </div>
    );
}

// BanList is the community's bans, in Community settings › Members, for those who may ban (its admins, and those who
// may remove members): the one place a ban is lifted. Bans are made from someone's menu or mod view.
export default function BanList({teamId}: {teamId: string}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const [bans, setBans] = useState<MemberBan[] | null>(null);
    const [failed, setFailed] = useState(false);

    useEffect(() => {
        let live = true;
        setBans(null);
        setFailed(false);
        Client4.getTeamBans(teamId).
            then((list) => {
                if (!live) {
                    return;
                }
                setBans(list);

                // The people banned, and who banned them, may be strangers to this session.
                const ids = [...new Set(list.flatMap((b) => [b.user_id, b.created_by]).filter(Boolean))];
                if (ids.length) {
                    dispatch(getMissingProfilesByIds(ids));
                }
            }).
            catch(() => live && setFailed(true));
        return () => {
            live = false;
        };
    }, [teamId, dispatch]);

    return (
        <section
            className={am('ban-list')}
            aria-label={formatMessage({id: 'fusion.bans.title', defaultMessage: 'Banned'})}
        >
            <Section title={formatMessage({id: 'fusion.bans.title', defaultMessage: 'Banned'})}/>
            {failed && <p className={am('field-error')}>{formatMessage({id: 'fusion.bans.loadFailed', defaultMessage: 'The bans couldn\'t be loaded.'})}</p>}
            {!failed && bans === null && <p className={am('lead')}>{'…'}</p>}
            {bans?.length === 0 && <p className={am('lead')}>{formatMessage({id: 'fusion.bans.none', defaultMessage: 'Nobody is banned. To ban someone, open their menu or their mod view.'})}</p>}
            {bans?.map((ban) => (
                <BanRow
                    key={ban.user_id}
                    ban={ban}
                    onLifted={(userId) => setBans((list) => (list || []).filter((b) => b.user_id !== userId))}
                />
            ))}
        </section>
    );
}
