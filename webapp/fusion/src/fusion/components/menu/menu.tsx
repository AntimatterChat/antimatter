// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useLayoutEffect, useRef, useState} from 'react';

import Icon from 'fusion/components/icon';
import type {IconName} from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

type ItemProps = {
    icon?: IconName;

    // An icon of its own instead of a Fusion one: an app's, in grey (MenuAppIcon).
    glyph?: React.ReactNode;
    label: React.ReactNode;
    sub?: React.ReactNode;
    danger?: boolean;
    accent?: boolean;
    checked?: boolean;
    role?: 'menuitem' | 'menuitemradio' | 'menuitemcheckbox';
    disabled?: boolean;

    // The action it stands for (data-action), unique in its menu: what tests and the docs' screenshots find it by.
    action?: string;

    // For an item that shows its choices under it, in the menu (a mute's durations): whether they're shown.
    expanded?: boolean;
    onClick?: (e: React.MouseEvent) => void;
    children?: React.ReactNode;
};

// MenuItem is a row of a .am-plus-pop menu.
export function MenuItem({icon, glyph, label, sub, danger, accent, checked, role = 'menuitem', disabled, action, expanded, onClick, children}: ItemProps) {
    return (
        <button
            type='button'
            role={role}
            aria-checked={role === 'menuitem' ? undefined : Boolean(checked)}
            aria-expanded={expanded}
            data-action={action}
            className={am({danger, accent, cur: role === 'menuitemradio' && checked})}
            disabled={disabled}
            onClick={onClick}
        >
            {glyph || (icon && <Icon name={icon}/>)}
            {sub ? (
                <span>
                    {label}
                    <small className={am('menu-sub')}>{sub}</small>
                </span>
            ) : label}
            {children}
            {checked && role !== 'menuitem' && (
                <span className={am('end')}>
                    <Icon
                        name='check'
                        size='sm'
                    />
                </span>
            )}
        </button>
    );
}

export function MenuSeparator() {
    return <hr/>;
}

export function MenuHeading({children}: {children: React.ReactNode}) {
    return <div className={am('pop-h')}>{children}</div>;
}

type FlyoutProps = {
    icon?: IconName;
    label: React.ReactNode;
    menuLabel: string;
    children: React.ReactNode;
};

// The submenu's width (see .am-fly-menu) and its gap to the menu.
const FLY_WIDTH = 230 + 4;

// Flyout is a submenu opened on hover or focus, towards the side of the screen with room, and upwards when it would
// run past the bottom of the window. Where it goes is decided as it opens: the menu holding it may still move after
// it mounts (a popover placing itself near the right edge, say).
export function Flyout({icon, label, menuLabel, children}: FlyoutProps) {
    const ref = useRef<HTMLDivElement>(null);
    const menuRef = useRef<HTMLDivElement>(null);
    const [side, setSide] = useState<'left' | 'right'>('right');
    const [up, setUp] = useState(false);
    const place = useCallback(() => {
        const r = ref.current?.getBoundingClientRect();
        if (!r) {
            return;
        }
        const roomRight = window.innerWidth - r.right;
        setSide(roomRight >= FLY_WIDTH + 8 || roomRight >= r.left ? 'right' : 'left');

        // Its height is known once shown (on hover or focus, by CSS): measure it on the next frame.
        requestAnimationFrame(() => {
            const height = menuRef.current?.offsetHeight ?? 0;
            setUp(height > 0 && r.top + height > window.innerHeight - 8);
        });
    }, []);
    useLayoutEffect(place, [place]);
    return (
        <div
            ref={ref}
            className={am('fly', side, {up})}
            onMouseEnter={place}
            onFocus={place}
        >
            <button
                type='button'
                role='menuitem'
                aria-haspopup='menu'
            >
                {icon && <Icon name={icon}/>}
                {label}
                <span className={am('end')}>
                    <Icon
                        name='chev'
                        size='xs'
                    />
                </span>
            </button>
            <div
                ref={menuRef}
                className={am('fly-menu')}
                role='menu'
                aria-label={menuLabel}
            >
                {children}
            </div>
        </div>
    );
}
