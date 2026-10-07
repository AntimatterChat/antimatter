// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useLayoutEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

// How tall a message's text can be before it's folded, in pixels: about twenty lines.
export const LONG_MESSAGE_HEIGHT = 440;

// Texts stay unfolded for the session once shown in full, also when drawn again (scrolling, the thread view).
const expanded = new Set<string>();

// LongBody folds a long message's text, with a fade and a Show more button under it, so that it doesn't take the
// whole channel; Show less folds it again.
export default function LongBody({postId, className, onClickCapture, children}: {postId: string; className: string; onClickCapture?: React.MouseEventHandler; children: React.ReactNode}) {
    const {formatMessage} = useIntl();
    const ref = useRef<HTMLDivElement>(null);
    const [long, setLong] = useState(false);
    const [open, setOpen] = useState(() => expanded.has(postId));

    // Measures the text as it renders and whenever it changes size (images loading, edits).
    useLayoutEffect(() => {
        const el = ref.current;
        if (!el) {
            return undefined;
        }
        const measure = () => setLong(el.scrollHeight > LONG_MESSAGE_HEIGHT + 60);
        measure();
        if (typeof ResizeObserver === 'undefined') {
            return undefined;
        }
        const observer = new ResizeObserver(measure);
        observer.observe(el.firstElementChild || el);
        return () => observer.disconnect();
    }, []);

    const folded = long && !open;
    const toggle = () => {
        if (open) {
            expanded.delete(postId);
        } else {
            expanded.add(postId);
        }
        setOpen(!open);
    };

    return (
        <>
            <div
                ref={ref}
                className={className + ' ' + am({folded})}
                style={folded ? {maxHeight: LONG_MESSAGE_HEIGHT} : undefined}
                onClickCapture={onClickCapture}
            >
                {children}
            </div>
            {long && (
                <button
                    type='button'
                    className={am('more-btn')}
                    aria-expanded={open}
                    onClick={toggle}
                >
                    <Icon
                        name='chev'
                        size='xs'
                    />
                    {open ? formatMessage({id: 'fusion.message.showLess', defaultMessage: 'Show less'}) : formatMessage({id: 'fusion.message.showMore', defaultMessage: 'Show more'})}
                </button>
            )}
        </>
    );
}
