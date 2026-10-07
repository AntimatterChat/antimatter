// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useSyncExternalStore} from 'react';
import {useIntl} from 'react-intl';

import type {Post} from '@mattermost/types/posts';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

// A spoiler is a message whose text and files stay hidden until its reader chooses to see them, as in Discord. Its
// spoiler prop says so; the server leaves its text out of notifications and quotes as well.
export const SPOILER_PROP = 'spoiler';

export function isSpoiler(post?: {props?: Record<string, unknown>}): boolean {
    const value = post?.props?.[SPOILER_PROP];
    return value === true || value === 'true';
}

// The spoilers shown stay shown for the session, also when their message is drawn again: scrolled back to, or opened
// in a thread.
const shown = new Set<string>();
const listeners = new Set<() => void>();

function subscribe(listener: () => void) {
    listeners.add(listener);
    return () => {
        listeners.delete(listener);
    };
}

export function setSpoilerShown(postId: string, show: boolean) {
    if (show) {
        shown.add(postId);
    } else {
        shown.delete(postId);
    }
    listeners.forEach((listener) => listener());
}

export function useSpoilerShown(postId: string): [boolean, (show: boolean) => void] {
    const isShown = useSyncExternalStore(subscribe, () => shown.has(postId));
    const setShown = useCallback((show: boolean) => setSpoilerShown(postId, show), [postId]);
    return [isShown, setShown];
}

// SpoilerCover stands in for a spoiler's text and files until its reader clicks it.
export function SpoilerCover({post, onShow}: {post: Post; onShow: () => void}) {
    const {formatMessage} = useIntl();
    const files = post.file_ids?.length || post.metadata?.files?.length || 0;
    return (
        <button
            type='button'
            className={am('spoiler-cover')}
            onClick={onShow}
        >
            <Icon name='eye-off'/>
            <span>
                <b>{formatMessage({id: 'fusion.spoiler.title', defaultMessage: 'Spoiler'})}</b>
                <small>
                    {files ? formatMessage({id: 'fusion.spoiler.showWithFiles', defaultMessage: 'Click to show the message and its {count, plural, one {file} other {# files}}'}, {count: files}) : formatMessage({id: 'fusion.spoiler.show', defaultMessage: 'Click to show the message'})}
                </small>
            </span>
        </button>
    );
}

// SpoilerTag marks a spoiler in its message's header; once shown, it hides it again.
export function SpoilerTag({isShown, onHide}: {isShown: boolean; onHide: () => void}) {
    const {formatMessage} = useIntl();
    const label = formatMessage({id: 'fusion.spoiler.title', defaultMessage: 'Spoiler'});
    if (!isShown) {
        return (
            <span className={am('meta-tag', 'spoiler')}>
                <Icon name='eye-off'/>
                {label}
            </span>
        );
    }
    return (
        <button
            type='button'
            className={am('meta-tag', 'spoiler')}
            title={formatMessage({id: 'fusion.spoiler.hide', defaultMessage: 'Hide the spoiler again'})}
            aria-label={formatMessage({id: 'fusion.spoiler.hide', defaultMessage: 'Hide the spoiler again'})}
            onClick={onHide}
        >
            <Icon name='eye-off'/>
            {label}
        </button>
    );
}
