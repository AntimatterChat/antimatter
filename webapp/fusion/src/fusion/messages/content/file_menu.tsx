// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {FileInfo} from '@mattermost/types/files';
import type {Post} from '@mattermost/types/posts';

import {getFilePublicLink} from 'mattermost-redux/actions/files';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getFileDownloadUrl, getFileUrl} from 'mattermost-redux/utils/file_utils';

import {Popover} from 'fusion/components/layer';
import {MenuItem} from 'fusion/components/menu';
import {useToast} from 'fusion/shell/toast_context';
import {permalinkPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';
import {copyToClipboard} from 'utils/utils';

import type {GlobalState} from 'types/store';

type Props = {
    file: FileInfo;
    post: Post;
    anchor?: HTMLElement | null;
    point?: {x: number; y: number};
    onClose: () => void;
};

// FileMenu is the menu of an attachment (its ⋯ button, or a right-click on it), with the classic file menu's actions:
// download it, copy its link (its public link when the server allows them), or open its message in the channel.
export default function FileMenu({file, post, anchor, point, onClose}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const team = useSelector(getCurrentTeam);
    const publicLinks = useSelector((state: GlobalState) => getConfig(state).EnablePublicLink === 'true');

    const copyLink = async () => {
        let link = new URL(getFileUrl(file.id), window.location.origin).href;
        if (publicLinks) {
            const result = await dispatch(getFilePublicLink(file.id)) as {data?: {link: string}};
            link = result.data?.link || link;
        }
        copyToClipboard(link);
        toast(formatMessage({id: 'fusion.files.linkCopied', defaultMessage: 'Link copied'}));
    };

    return (
        <Popover
            anchor={anchor}
            point={point}
            placement={point ? 'point' : 'below'}
            className='plus-pop ch-menu'
            role='menu'
            label={formatMessage({id: 'fusion.files.menu', defaultMessage: 'Options for {name}'}, {name: file.name})}
            onClose={onClose}
        >
            <MenuItem
                icon='upload'
                label={formatMessage({id: 'fusion.files.download', defaultMessage: 'Download'})}
                onClick={() => {
                    const a = document.createElement('a');
                    a.href = getFileDownloadUrl(file.id);
                    a.download = file.name;
                    a.click();
                    onClose();
                }}
            />
            <MenuItem
                icon='link'
                label={publicLinks ? formatMessage({id: 'fusion.files.copyPublicLink', defaultMessage: 'Copy public link'}) : formatMessage({id: 'fusion.files.copyLink', defaultMessage: 'Copy link'})}
                onClick={() => {
                    copyLink();
                    onClose();
                }}
            />
            {team && (
                <MenuItem
                    icon='popout'
                    label={formatMessage({id: 'fusion.files.openInChannel', defaultMessage: 'Open in channel'})}
                    onClick={() => {
                        getHistory().push(permalinkPath(team.name, post.id));
                        onClose();
                    }}
                />
            )}
        </Popover>
    );
}
