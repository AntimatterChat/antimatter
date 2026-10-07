// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useLayoutEffect, useRef, useState, useSyncExternalStore} from 'react';
import {useIntl} from 'react-intl';

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

// SpoilerVeil hides a spoiler's text and files in place until its reader clicks it, as in Discord: the text becomes
// bars and the files are blurred, at their own size, so that showing it moves nothing around it.
// Below this width, in pixels, the veil's label is the crossed eye alone, without "Spoiler".
const LABEL_MIN_WIDTH = 110;

export function SpoilerVeil({hidden, onShow, children}: {hidden: boolean; onShow: () => void; children: React.ReactNode}) {
    const {formatMessage} = useIntl();
    const ref = useRef<HTMLDivElement>(null);
    const [wide, setWide] = useState(true);

    // The label fits the veil: short spoilers only get the icon.
    useLayoutEffect(() => {
        const el = ref.current;
        if (!el) {
            return undefined;
        }
        const measure = () => setWide(el.getBoundingClientRect().width >= LABEL_MIN_WIDTH);
        measure();
        if (typeof ResizeObserver === 'undefined') {
            return undefined;
        }
        const observer = new ResizeObserver(measure);
        observer.observe(el);
        return () => observer.disconnect();
    }, [hidden]);

    if (!hidden) {
        return <>{children}</>;
    }
    return (
        <div
            ref={ref}
            className={am('spoiler-veil')}
            role='button'
            tabIndex={0}
            title={formatMessage({id: 'fusion.spoiler.show', defaultMessage: 'Click to show the spoiler'})}
            aria-label={formatMessage({id: 'fusion.spoiler.label', defaultMessage: 'Spoiler, click to show it'})}
            onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                onShow();
            }}
            onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    onShow();
                }
            }}
        >
            {/* Nothing inside can be clicked, focused or read out while hidden. */}
            <div
                aria-hidden='true'
                inert={true}
            >
                {children}
            </div>
            <span
                className={am('spoiler-label')}
                aria-hidden='true'
            >
                <Icon
                    name='eye-off'
                    size='sm'
                />
                {wide && formatMessage({id: 'fusion.spoiler.title', defaultMessage: 'Spoiler'})}
            </span>
        </div>
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
