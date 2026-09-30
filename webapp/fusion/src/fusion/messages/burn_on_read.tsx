// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {Preferences} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getBool} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {burnPostNow} from 'actions/burn_on_read_deletion';
import {revealBurnOnReadPost} from 'actions/burn_on_read_posts';
import {openModal} from 'actions/views/modals';
import {getBurnOnReadRecipientData} from 'selectors/burn_on_read_recipients';

import BurnOnReadConfirmationModal from 'components/burn_on_read_confirmation_modal';
import BurnOnReadExpirationHandler from 'components/post_view/burn_on_read_expiration_handler';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';
import {useBurnOnReadDeleteModal} from 'hooks/useBurnOnReadDeleteModal';
import {ModalIdentifiers} from 'utils/constants';

import type {GlobalState} from 'types/store';

// A Burn-on-Read message's timers: metadata.expire_at once opened (or, for the sender, once everyone opened it), and
// props.expire_at, how long it lives unopened.
function toNumber(value: unknown): number | null {
    const n = typeof value === 'number' ? value : parseInt(String(value ?? ''), 10);
    return Number.isFinite(n) && n > 0 ? n : null;
}

export function burnTimers(post: Post) {
    return {expireAt: toNumber(post.metadata?.expire_at), maxExpireAt: toNumber(post.props?.expire_at)};
}

// useCountdown gives the time left until a moment as m:ss (or h:mm:ss), updated every second.
function useCountdown(until: number | null): string {
    const [now, setNow] = useState(Date.now);
    useEffect(() => {
        if (!until) {
            return undefined;
        }
        const timer = setInterval(() => setNow(Date.now()), 1000);
        return () => clearInterval(timer);
    }, [until]);
    if (!until) {
        return '';
    }
    const left = Math.max(0, Math.round((until - now) / 1000));
    const h = Math.floor(left / 3600);
    const m = Math.floor((left % 3600) / 60);
    const s = String(left % 60).padStart(2, '0');
    return h ? `${h}:${String(m).padStart(2, '0')}:${s}` : `${m}:${s}`;
}

// useBurnDuration describes how long a message lives once opened: its own read_duration, else the server's setting.
function useBurnDuration(post: Post): string {
    const {formatMessage} = useIntl();
    const config = useSelector(getConfig);
    const ms = toNumber(post.props?.read_duration);
    const seconds = ms ? Math.round(ms / 1000) : parseInt(config.BurnOnReadDurationSeconds || '600', 10);
    if (seconds < 60) {
        return formatMessage({id: 'fusion.burn.seconds', defaultMessage: '{count, plural, one {# second} other {# seconds}}'}, {count: seconds});
    }
    if (seconds < 3600) {
        return formatMessage({id: 'fusion.burn.minutes', defaultMessage: '{count, plural, one {# minute} other {# minutes}}'}, {count: Math.round(seconds / 60)});
    }
    return formatMessage({id: 'fusion.burn.hours', defaultMessage: '{count, plural, one {# hour} other {# hours}}'}, {count: Math.round(seconds / 3600)});
}

// BurnCover hides a Burn-on-Read message until its reader opens it: the mockup's .burn-cover.
export function BurnCover({post}: {post: Post}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const duration = useBurnDuration(post);
    const [opening, setOpening] = useState(false);
    const [error, setError] = useState('');

    const reveal = async () => {
        if (opening) {
            return;
        }
        setOpening(true);
        setError('');
        const result = await dispatch(revealBurnOnReadPost(post.id));
        setOpening(false);
        if (result && 'error' in result && result.error) {
            const status = (result.error as {status_code?: number}).status_code;
            if (status === 404) {
                setError(formatMessage({id: 'fusion.burn.gone', defaultMessage: 'This message is no longer available.'}));
            } else if (status === 403) {
                setError(formatMessage({id: 'fusion.burn.forbidden', defaultMessage: "You don't have permission to read this message."}));
            } else {
                setError(formatMessage({id: 'fusion.burn.failed', defaultMessage: 'The message could not be opened. Try again.'}));
            }
        }
    };

    return (
        <button
            className={am('burn-cover')}
            aria-busy={opening}
            onClick={reveal}
        >
            <Icon
                name='flame'
                size='sm'
            />
            <span role={error ? 'alert' : undefined}>
                {error || formatMessage({id: 'fusion.burn.cover', defaultMessage: 'Click to read — this message burns {duration} after you open it'}, {duration})}
            </span>
        </button>
    );
}

// BurnTag is the header tag of a Burn-on-Read message: "Burn on read" until it's opened, then "Burning…" with the
// time left. It also schedules the message's removal when a timer runs out. The sender can burn it for everyone.
export function BurnTag({post, showTag = true}: {post: Post; showTag?: boolean}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const me = useSelector(getCurrentUserId);
    const skipConfirmation = useSelector((state: GlobalState) => getBool(state, Preferences.CATEGORY_BURN_ON_READ, Preferences.BURN_ON_READ_SKIP_CONFIRMATION, false));
    const recipients = useSelector((state: GlobalState) => getBurnOnReadRecipientData(state, post, me));
    const {expireAt, maxExpireAt} = burnTimers(post);
    const left = useCountdown(expireAt);
    const sender = post.user_id === me;
    const handlers = useBurnOnReadDeleteModal({postId: post.id, userId: me, isSender: sender});

    const handler = (
        <BurnOnReadExpirationHandler
            postId={post.id}
            expireAt={expireAt}
            maxExpireAt={expireAt ? null : maxExpireAt}
        />
    );
    if (!showTag) {
        return handler;
    }

    const label = expireAt ? formatMessage({id: 'fusion.burn.burning', defaultMessage: 'Burning… {left}'}, {left}) : formatMessage({id: 'fusion.burn.tag', defaultMessage: 'Burn on read'});
    let title = formatMessage({id: 'fusion.burn.tagTitle', defaultMessage: 'This message is deleted after it is read'});
    if (sender) {
        title = formatMessage({id: 'fusion.burn.senderTitle', defaultMessage: 'Click to delete the message for everyone'});
        if (recipients) {
            title += ' · ' + formatMessage({id: 'fusion.burn.readBy', defaultMessage: 'Read by {count} of {total}'}, {count: recipients.revealedCount, total: recipients.totalRecipients});
        }
    }

    const burnNow = () => {
        if (skipConfirmation && !sender) {
            dispatch(burnPostNow(post.id));
            return;
        }
        dispatch(openModal({
            modalId: ModalIdentifiers.BURN_ON_READ_CONFIRMATION,
            dialogType: BurnOnReadConfirmationModal,
            dialogProps: {show: true, ...handlers},
        }));
    };

    const content = (
        <>
            <Icon name='flame'/>
            {label}
        </>
    );

    return (
        <>
            {handler}
            {sender || expireAt ? (
                <button
                    type='button'
                    className={am('meta-tag', 'burn')}
                    title={sender ? title : formatMessage({id: 'fusion.burn.burnNow', defaultMessage: 'Click to burn it now'})}
                    onClick={burnNow}
                >
                    {content}
                </button>
            ) : (
                <span
                    className={am('meta-tag', 'burn')}
                    title={title}
                >
                    {content}
                </span>
            )}
        </>
    );
}
