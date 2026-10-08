// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {FileInfo} from '@mattermost/types/files';
import type {Post} from '@mattermost/types/posts';

import {Posts} from 'mattermost-redux/constants';
import {makeGetFilesForPost} from 'mattermost-redux/selectors/entities/files';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getFileDownloadUrl, getFilePreviewUrl, getFileUrl, getFormattedFileSize, sortFileInfos} from 'mattermost-redux/utils/file_utils';

import FilePreviewModal from 'components/file_preview_modal';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';
import {openDialog} from 'fusion/utils/modals';
import {ModalIdentifiers} from 'utils/constants';

import type {GlobalState} from 'types/store';

import FileMenu from './file_menu';
import {imageCaption, useLightbox} from './lightbox';

function isImage(file: FileInfo, svgs: boolean) {
    const ext = (file.extension || '').toLowerCase();
    if (ext === 'svg') {
        return svgs;
    }
    return (file.mime_type || '').startsWith('image/') && (file.has_preview_image || ext === 'gif');
}

// The picture shown for an image: its preview, or the file itself for animated GIFs and SVGs.
function imageSrc(file: FileInfo) {
    const ext = (file.extension || '').toLowerCase();
    return file.has_preview_image && ext !== 'gif' ? getFilePreviewUrl(file.id) : getFileUrl(file.id);
}

// Files draws a message's uploads as the mockup does: images as .img-att tiles that open the lightbox, with their name
// and size in the tooltip rather than over the picture, other files as .attach chips (extension badge, name,
// "412 KB · PDF") that open the classic file preview.
export default function Files({post}: {post: Post}) {
    const {formatMessage, locale} = useIntl();
    const dispatch = useDispatch();
    const getFiles = useMemo(() => makeGetFilesForPost(), []);
    const stored = useSelector((state: GlobalState) => getFiles(state, post.id));
    const svgs = useSelector((state: GlobalState) => getConfig(state).EnableSVGs === 'true');
    const [lightbox, openImage] = useLightbox();

    // The menu of an attachment: from its ⋯ button (anchor) or a right-click (point).
    const [menu, setMenu] = useState<{file: FileInfo; anchor?: HTMLElement; point?: {x: number; y: number}} | null>(null);
    const contextMenu = (file: FileInfo) => (e: React.MouseEvent) => {
        e.preventDefault();
        setMenu({file, point: {x: e.clientX, y: e.clientY}});
    };
    const moreButton = (file: FileInfo) => (
        <span
            role='button'
            tabIndex={0}
            className={am('att-more')}
            title={formatMessage({id: 'fusion.files.more', defaultMessage: 'More actions'})}
            aria-label={formatMessage({id: 'fusion.files.menu', defaultMessage: 'Options for {name}'}, {name: file.name})}
            onClick={(e) => {
                e.stopPropagation();
                setMenu({file, anchor: e.currentTarget});
            }}
            onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    e.stopPropagation();
                    setMenu({file, anchor: e.currentTarget});
                }
            }}
        >
            <Icon
                name='dots'
                size='sm'
            />
        </span>
    );

    // Burn-on-read messages carry their files in the post, not in the store.
    const files = sortFileInfos(post.type === Posts.POST_TYPES.BURN_ON_READ && post.metadata?.files ? post.metadata.files : stored, locale);
    if (!files.length) {
        return null;
    }

    const preview = (index: number) => dispatch(openDialog(ModalIdentifiers.FILE_PREVIEW_MODAL, FilePreviewModal, {postId: post.id, post, fileInfos: files, startIndex: index}));

    return (
        <div className={am('files')}>
            {files.map((file, index) => {
                const size = getFormattedFileSize(file.size);
                if (isImage(file, svgs)) {
                    const image = {src: imageSrc(file), name: file.name, size, width: file.width, height: file.height, download: getFileDownloadUrl(file.id)};
                    const ratio = file.width && file.height ? `${file.width} / ${file.height}` : undefined;
                    return (
                        <button
                            key={file.id}
                            type='button'
                            className={am('img-att')}
                            style={ratio ? {aspectRatio: ratio} : undefined}
                            aria-label={formatMessage({id: 'fusion.files.openImage', defaultMessage: 'Open image {name}'}, {name: file.name})}
                            title={imageCaption({...image, width: undefined, height: undefined})}
                            onClick={() => openImage(image)}
                            onContextMenu={contextMenu(file)}
                        >
                            <img
                                src={image.src}
                                alt=''
                                loading='lazy'
                            />
                            {moreButton(file)}
                        </button>
                    );
                }
                const ext = (file.extension || '').toUpperCase().slice(0, 4) || formatMessage({id: 'fusion.files.file', defaultMessage: 'FILE'});
                return (
                    <button
                        key={file.id}
                        type='button'
                        className={am('attach')}
                        aria-label={formatMessage({id: 'fusion.files.open', defaultMessage: 'Open {name}'}, {name: file.name})}
                        onClick={() => preview(index)}
                        onContextMenu={contextMenu(file)}
                    >
                        <span className={am('file')}>{ext}</span>
                        <span className={am('attach-meta')}>
                            <b>{file.name}</b>
                            <span>{file.extension ? `${size} · ${file.extension.toUpperCase()}` : size}</span>
                        </span>
                        {moreButton(file)}
                    </button>
                );
            })}
            {lightbox}
            {menu && (
                <FileMenu
                    file={menu.file}
                    post={post}
                    anchor={menu.anchor}
                    point={menu.point}
                    onClose={() => setMenu(null)}
                />
            )}
        </div>
    );
}
