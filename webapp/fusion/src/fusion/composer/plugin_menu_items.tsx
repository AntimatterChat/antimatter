// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {shallowEqual, useSelector} from 'react-redux';

import {getComposerMenuItems} from 'selectors/composer_menu';

import {isIconName} from 'fusion/components/icon';
import {MenuItem} from 'fusion/components/menu';

import type {GlobalState} from 'types/store';

type Props = {
    channelId: string;
    rootId: string;

    // Closes the "+" menu before the item runs.
    onClose: () => void;
};

// PluginMenuItems are the items plugins add to the composer's "+" menu (registerComposerMenuItem), e.g. "Create a
// poll". A plugin's item uses the Fusion icon it names (fusionIcon), else its own icon.
export default function PluginMenuItems({channelId, rootId, onClose}: Props) {
    const ctx = {channelId, rootId: rootId || undefined};
    const items = useSelector((state: GlobalState) => getComposerMenuItems(state, ctx), shallowEqual);

    return (
        <>
            {items.map((item) => {
                const fusionIcon = item.fusionIcon && isIconName(item.fusionIcon) ? item.fusionIcon : undefined;
                return (
                    <MenuItem
                        key={item.id}
                        icon={fusionIcon}
                        label={fusionIcon ? item.text : (
                            <>
                                {item.icon}
                                <span>{item.text}</span>
                            </>
                        )}
                        onClick={() => {
                            onClose();
                            item.action(ctx);
                        }}
                    />
                );
            })}
        </>
    );
}
