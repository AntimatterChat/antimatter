// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useSelector} from 'react-redux';

import {Flyout, MenuHeading} from 'fusion/components/menu';

import type {GlobalState} from 'types/store';
import type {PostDropdownMenuAction} from 'types/store/plugins';

const NO_ACTIONS: PostDropdownMenuAction[] = [];

type Props = {
    postId: string;

    // The flyout holding the integrations' actions ("More actions"), and its menu's accessible name.
    label: string;
    menuLabel: string;

    // Runs an action, then closes the menu holding the entries.
    run: (action: () => void) => () => void;
};

// PluginMenuActions are the entries integrations add to a message's ⋯ menu (registerPostDropdownMenuAction), all
// under one "More actions" flyout: no app gets an entry of its own in the menu. The single actions come first, then
// each submenu (registerPostDropdownSubMenuAction, such as the Agents' AI actions) as a headed group.
export default function PluginMenuActions({postId, label, menuLabel, run}: Props) {
    const all = useSelector((state: GlobalState) => state.plugins.components.PostDropdownMenu) || NO_ACTIONS;
    const shown = (a: PostDropdownMenuAction) => !a.filter || a.filter(postId);
    const visible = all.filter(shown);
    const single = visible.filter((a) => !a.subMenu?.length);
    const groups = visible.
        filter((a) => a.subMenu?.length).
        map((group) => ({group, items: (group.subMenu || []).filter(shown)})).
        filter(({items}) => items.length > 0);

    if (!single.length && !groups.length) {
        return null;
    }

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
        <Flyout
            icon='plug'
            label={label}
            menuLabel={menuLabel}
        >
            {single.map(item)}
            {groups.map(({group, items}) => (
                <React.Fragment key={group.id}>
                    <MenuHeading>{group.text}</MenuHeading>
                    {items.map(item)}
                </React.Fragment>
            ))}
        </Flyout>
    );
}
