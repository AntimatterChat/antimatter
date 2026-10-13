// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useSelector} from 'react-redux';

import {getKnownApp} from 'fusion/apps/known_apps';
import {Flyout} from 'fusion/components/menu';

import type {GlobalState} from 'types/store';
import type {PostDropdownMenuAction} from 'types/store/plugins';

const NO_ACTIONS: PostDropdownMenuAction[] = [];

type Props = {
    postId: string;

    // The flyout of the integrations' single actions, and its menu's accessible name.
    label: string;
    menuLabel: string;

    // Runs an action, then closes the menu holding the entries.
    run: (action: () => void) => () => void;
};

// PluginMenuActions are the entries integrations add to a message's ⋯ menu (registerPostDropdownMenuAction): their
// single actions under one flyout, and each submenu (registerPostDropdownSubMenuAction, such as the Agents' AI
// actions) as its own flyout, with the plugin's app icon.
export default function PluginMenuActions({postId, label, menuLabel, run}: Props) {
    const all = useSelector((state: GlobalState) => state.plugins.components.PostDropdownMenu) || NO_ACTIONS;
    const shown = (a: PostDropdownMenuAction) => !a.filter || a.filter(postId);
    const visible = all.filter(shown);
    const single = visible.filter((a) => !a.subMenu?.length);
    const groups = visible.filter((a) => a.subMenu?.length);

    const item = (action: PostDropdownMenuAction) => (
        <button
            key={action.id}
            type='button'
            role='menuitem'
            onClick={run(() => action.action?.(postId))}
        >
            {action.text}
        </button>
    );

    return (
        <>
            {groups.map((group) => {
                const items = (group.subMenu || []).filter(shown);
                if (!items.length) {
                    return null;
                }
                return (
                    <Flyout
                        key={group.id}
                        icon={getKnownApp(group.pluginId)?.icon || 'plug'}
                        label={group.text}
                        menuLabel={typeof group.text === 'string' ? group.text : label}
                    >
                        {items.map(item)}
                    </Flyout>
                );
            })}
            {single.length > 0 && (
                <Flyout
                    icon='plug'
                    label={label}
                    menuLabel={menuLabel}
                >
                    {single.map(item)}
                </Flyout>
            )}
        </>
    );
}
