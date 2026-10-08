// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useDispatch, useSelector} from 'react-redux';

import {savePreferences} from 'mattermost-redux/actions/preferences';
import {get as getPreference} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentUser} from 'mattermost-redux/selectors/entities/users';

import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

// The rows the settings panes are made of.

export function Toggle({title, desc, on, onChange}: {title: string; desc: string; on: boolean; onChange: (on: boolean) => void}) {
    return (
        <div className={am('toggle-row')}>
            <div className={am('txt')}>
                <b>{title}</b>
                <span>{desc}</span>
            </div>
            <button
                type='button'
                className={am('switch')}
                role='switch'
                aria-checked={on}
                aria-label={title}
                onClick={() => onChange(!on)}
            />
        </div>
    );
}

// Choice is a setting row with a few mutually exclusive options, as a segmented control.
export function Choice<T extends string>({title, desc, value, options, onChange}: {title: string; desc?: string; value: T; options: Array<[T, string]>; onChange: (value: T) => void}) {
    return (
        <div className={am('toggle-row', 'choice-row')}>
            <div className={am('txt')}>
                <b>{title}</b>
                {desc && <span>{desc}</span>}
            </div>
            <div
                className={am('seg')}
                role='group'
                aria-label={title}
            >
                {options.map(([key, label]) => (
                    <button
                        key={key}
                        type='button'
                        className={am({on: key === value})}
                        aria-pressed={key === value}
                        onClick={() => key !== value && onChange(key)}
                    >
                        {label}
                    </button>
                ))}
            </div>
        </div>
    );
}

// Action is a setting row whose button opens where the setting is changed.
export function Action({title, desc, label, danger = false, onClick}: {title: string; desc: string; label: string; danger?: boolean; onClick: () => void}) {
    return (
        <div className={am('toggle-row')}>
            <div className={am('txt')}>
                <b>{title}</b>
                <span>{desc}</span>
            </div>
            <button
                type='button'
                className={am('btn', {danger})}
                onClick={onClick}
            >
                {label}
            </button>
        </div>
    );
}

// usePreference reads one of the current user's preferences, and saves it.
export function usePreference(category: string, name: string, fallback: string): [string, (value: string) => void] {
    const dispatch = useDispatch();
    const me = useSelector(getCurrentUser);
    const value = useSelector((state: GlobalState) => getPreference(state, category, name, fallback));
    const save = (next: string) => dispatch(savePreferences(me.id, [{user_id: me.id, category, name, value: next}]));
    return [value, save];
}

// Select is a setting row with a longer list of options, as a menu; a button after it can preview the choice.
export function Select<T extends string>({title, desc, value, options, onChange, after}: {title: string; desc?: string; value: T; options: Array<[T, string]>; onChange: (value: T) => void; after?: React.ReactNode}) {
    return (
        <div className={am('toggle-row', 'choice-row')}>
            <div className={am('txt')}>
                <b>{title}</b>
                {desc && <span>{desc}</span>}
            </div>
            <select
                className={am('select')}
                value={value}
                aria-label={title}
                onChange={(e) => onChange(e.target.value as T)}
            >
                {options.map(([key, label]) => (
                    <option
                        key={key}
                        value={key}
                    >
                        {label}
                    </option>
                ))}
            </select>
            {after}
        </div>
    );
}

// Section titles a group of rows in a pane.
export function Section({title}: {title: string}) {
    return <h3 className={am('set-section')}>{title}</h3>;
}
