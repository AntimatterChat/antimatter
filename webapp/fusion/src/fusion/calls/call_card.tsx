// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import type {IntlShape} from 'react-intl';
import {useSelector} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {useCallActions, useChannelLabel} from './actions';
import {useCall, useCallsAvailable, useLocalCall, useParticipants} from './hooks';

// The Calls plugin's post for a call (start, end, and in direct messages the ringing outcome).
const CALL_POST_TYPE = 'custom_calls';

function isCallPost(post: Post): boolean {
    return (post.type as string) === CALL_POST_TYPE;
}

// useCallCard tells whether a post is drawn as the call card: a Calls post, unless a Calls build without the API
// still draws its own.
export function useCallCard(post?: Post): boolean {
    const available = useCallsAvailable();
    const pluginCard = useSelector((state: GlobalState) => Boolean(state.plugins.postTypes[CALL_POST_TYPE]));
    return Boolean(post && isCallPost(post) && (available || !pluginCard));
}

const MINUTE = 60 * 1000;

// durationLabel is the mockup's sinceLabel: whole minutes (at least 1), or hours and minutes from an hour up.
export function durationLabel(ms: number, intl: IntlShape): string {
    const minutes = Math.max(1, Math.round(ms / MINUTE));
    if (minutes >= 60) {
        return intl.formatMessage({id: 'fusion.calls.hours', defaultMessage: '{hours}h {minutes}m'}, {hours: Math.floor(minutes / 60), minutes: minutes % 60});
    }
    return intl.formatMessage({id: 'fusion.calls.minutes', defaultMessage: '{minutes} min'}, {minutes});
}

// useNow re-renders every minute, for elapsed times.
function useNow(active: boolean): number {
    const [now, setNow] = useState(Date.now());
    useEffect(() => {
        if (!active) {
            return undefined;
        }
        setNow(Date.now());
        const timer = setInterval(() => setNow(Date.now()), MINUTE);
        return () => clearInterval(timer);
    }, [active]);
    return now;
}

function num(value: unknown): number {
    return typeof value === 'number' ? value : 0;
}

// CallCard is the mockup's call card in place of the Calls plugin's call post: who is in the call and a button to
// join or leave it while it runs, and how long it lasted once it's over.
export default function CallCard({post}: {post: Post}) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const actions = useCallActions();
    const call = useCall(post.channel_id);
    const participants = useParticipants(post.channel_id);
    const local = useLocalCall();
    const where = useChannelLabel(post.channel_id);
    const starter = useDisplayName(useUser(post.user_id));

    const title = typeof post.props?.title === 'string' ? post.props.title : '';
    const startAt = num(post.props?.start_at);
    const endAt = num(post.props?.end_at);
    const status = typeof post.props?.call_status === 'string' ? post.props.call_status : '';
    const live = !endAt && Boolean(call) && (call!.threadId === post.id || call!.startAt === startAt);
    const now = useNow(live);

    if (!live) {
        let sub = '';
        if (status === 'no_answer') {
            sub = formatMessage({id: 'fusion.calls.card.noAnswer', defaultMessage: 'No answer'});
        } else if (status === 'declined') {
            sub = formatMessage({id: 'fusion.calls.card.declined', defaultMessage: 'Declined'});
        } else if (status === 'canceled_by_caller') {
            sub = formatMessage({id: 'fusion.calls.card.canceled', defaultMessage: 'Canceled'});
        } else if (endAt && startAt) {
            sub = formatMessage({id: 'fusion.calls.card.lasted', defaultMessage: 'Lasted {duration}'}, {duration: durationLabel(endAt - startAt, intl)});
        }
        return (
            <div className={am('call-card', 'ended')}>
                <span className={am('ph')}><Icon name='phone'/></span>
                <span className={am('grow')}>
                    <b>{formatMessage({id: 'fusion.calls.card.ended', defaultMessage: 'Call ended'})}</b>
                    {sub && <span className={am('sub')}>{sub}</span>}
                </span>
            </div>
        );
    }

    const inThis = local?.channelId === post.channel_id;
    const faces = participants.slice(0, 4);
    return (
        <div className={am('call-card')}>
            <span className={am('ph')}><Icon name='phone'/></span>
            <span className={am('grow')}>
                <b>{title || formatMessage({id: 'fusion.calls.card.callIn', defaultMessage: 'Call in {where}'}, {where})}</b>
                <span className={am('sub')}>
                    {formatMessage(
                        {id: 'fusion.calls.card.sub', defaultMessage: 'Started by {name} · {since} · {count, plural, one {# person} other {# people}}'},
                        {name: starter, since: durationLabel(now - (startAt || call!.startAt), intl), count: participants.length},
                    )}
                </span>
            </span>
            {faces.length > 0 && (
                <span className={am('faces')}>
                    {faces.map((p) => (
                        <Avatar
                            key={p.sessionId}
                            userId={p.userId}
                            size='sm'
                        />
                    ))}
                </span>
            )}
            {inThis ? (
                <button
                    className={am('btn', 'danger')}
                    style={{height: 32}}
                    onClick={actions.leaveCall}
                >
                    <Icon
                        name='hangup'
                        size='sm'
                    />
                    {formatMessage({id: 'fusion.calls.card.leave', defaultMessage: 'Leave'})}
                </button>
            ) : (
                <button
                    className={am('btn')}
                    style={{height: 32, background: 'var(--am-ok)', color: '#fff'}}
                    onClick={() => actions.startOrJoinCall(post.channel_id)}
                >
                    <Icon
                        name='phone'
                        size='sm'
                    />
                    {formatMessage({id: 'fusion.calls.card.join', defaultMessage: 'Join'})}
                </button>
            )}
        </div>
    );
}
