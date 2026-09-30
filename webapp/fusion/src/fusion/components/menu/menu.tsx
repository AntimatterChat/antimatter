// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useLayoutEffect, useRef, useState} from 'react';

import Icon from 'fusion/components/icon';
import type {IconName} from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

type ItemProps = {
    icon?: IconName;
    label: React.ReactNode;
    sub?: React.ReactNode;
    danger?: boolean;
    accent?: boolean;
    checked?: boolean;
    role?: 'menuitem' | 'menuitemradio' | 'menuitemcheckbox';
    disabled?: boolean;
    onClick?: (e: React.MouseEvent) => void;
    children?: React.ReactNode;
};

// MenuItem is a row of a .am-plus-pop menu.
export function MenuItem({icon, label, sub, danger, accent, checked, role = 'menuitem', disabled, onClick, children}: ItemProps) {
    return (
        <button
            type='button'
            role={role}
            aria-checked={role === 'menuitem' ? undefined : Boolean(checked)}
            className={am({danger, accent, cur: role === 'menuitemradio' && checked})}
            disabled={disabled}
            onClick={onClick}
        >
            {icon && <Icon name={icon}/>}
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

// Flyout is a submenu opened on hover or focus, towards the side of the screen with room.
export function Flyout({icon, label, menuLabel, children}: FlyoutProps) {
    const ref = useRef<HTMLDivElement>(null);
    const [side, setSide] = useState<'left' | 'right'>('right');
    useLayoutEffect(() => {
        const pop = ref.current?.closest('.am-pop');
        if (pop) {
            const r = pop.getBoundingClientRect();
            setSide(r.right + 240 > window.innerWidth ? 'left' : 'right');
        }
    }, []);
    return (
        <div
            ref={ref}
            className={am('fly', side)}
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
                className={am('fly-menu')}
                role='menu'
                aria-label={menuLabel}
            >
                {children}
            </div>
        </div>
    );
}
