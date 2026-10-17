// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import {Popover} from 'fusion/components/layer';
import {MenuItem} from 'fusion/components/menu';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';
import type {ComposerModeRegistration, FileUploadMethodAction} from 'types/store/plugins';

import {ComposerModeItems} from './composer_modes';
import PluginMenuItems from './plugin_menu_items';

const NO_METHODS: FileUploadMethodAction[] = [];
const NO_MODES: ComposerModeRegistration[] = [];

type Props = {
    anchor: HTMLElement | null;
    channelId: string;
    rootId: string;

    // Opens the file picker; uploads files an app's upload method picked.
    onPickFile: () => void;
    onUpload: (files: FileList | File[]) => void;
    onClose: () => void;

    // Apps' composer modes (a poll…), which the composer switches to.
    modes?: ComposerModeRegistration[];
    onMode?: (id: string) => void;
};

// PlusMenu is the composer's "+" menu: only what adds something to the message (a file, and what apps add: a
// picture from elsewhere, a poll, a location…). Scheduling is the send button's chevron (and the formatting bar's
// clock); slash commands start by typing /.
export default function PlusMenu({anchor, channelId, rootId, onPickFile, onUpload, onClose, modes = NO_MODES, onMode}: Props) {
    const {formatMessage} = useIntl();
    const uploadMethods = useSelector((state: GlobalState) => state.plugins.components.FileUploadMethod) || NO_METHODS;

    return (
        <Popover
            anchor={anchor}
            placement='above'
            className='plus-pop'
            role='menu'
            label={formatMessage({id: 'fusion.composer.plusMenu', defaultMessage: 'Add to message'})}
            onClose={onClose}
        >
            <MenuItem
                icon='upload'
                label={formatMessage({id: 'fusion.composer.upload', defaultMessage: 'Upload a file'})}
                onClick={() => {
                    onClose();
                    onPickFile();
                }}
            />
            {uploadMethods.map((method) => (
                <button
                    key={method.id}
                    type='button'
                    role='menuitem'
                    onClick={() => {
                        onClose();
                        method.action?.((files: FileList | File[]) => onUpload(files));
                    }}
                >
                    <span className={am('plugin-ic')}>{method.icon}</span>
                    {method.text}
                </button>
            ))}
            {onMode && (
                <ComposerModeItems
                    modes={modes}
                    onPick={(id) => {
                        onClose();
                        onMode(id);
                    }}
                />
            )}
            <PluginMenuItems
                channelId={channelId}
                rootId={rootId}
                onClose={onClose}
            />
        </Popover>
    );
}
