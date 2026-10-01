// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useSelector} from 'react-redux';

import {PlusIcon} from '@mattermost/compass-icons/components';

import {getComposerMenuItems} from 'selectors/composer_menu';

import * as Menu from 'components/menu';

import type {GlobalState} from 'types/store';
import type {ComposerMenuItemRegistration} from 'types/store/plugins';

type Props = {
    channelId: string;
    rootId: string;
};

// ComposerMenu is the message box's "+" menu of plugin items (registerComposerMenuItem), e.g.
// "Create a poll". It's only shown when a plugin has an item for this channel or thread.
export default function ComposerMenu({channelId, rootId}: Props) {
    const {formatMessage} = useIntl();
    const items = useSelector((state: GlobalState) => getComposerMenuItems(state, {channelId, rootId: rootId || undefined}), shallowEqual);

    const run = useCallback((item: ComposerMenuItemRegistration) => {
        item.action({channelId, rootId: rootId || undefined});
    }, [channelId, rootId]);

    if (!items.length) {
        return null;
    }

    const label = formatMessage({id: 'advanced_text_editor.composer_menu', defaultMessage: 'Add to message'});
    const key = rootId || channelId;
    return (
        <Menu.Container
            menuButton={{
                id: `composerMenuButton_${key}`,
                class: 'style--none AdvancedTextEditor__action-button',
                'aria-label': label,
                children: (
                    <PlusIcon
                        size={18}
                        color='currentColor'
                    />
                ),
            }}
            menuButtonTooltip={{text: label}}
            menu={{
                id: `composerMenu_${key}`,
                'aria-label': label,
            }}
            anchorOrigin={{vertical: 'top', horizontal: 'left'}}
            transformOrigin={{vertical: 'bottom', horizontal: 'left'}}
        >
            {items.map((item) => (
                <Menu.Item
                    key={item.id}
                    id={`composerMenuItem_${item.id}`}
                    leadingElement={item.icon}
                    labels={<span>{item.text}</span>}
                    onClick={() => run(item)}
                />
            ))}
        </Menu.Container>
    );
}
