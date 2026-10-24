// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {FileInfo} from '@mattermost/types/files';
import type {Post} from '@mattermost/types/posts';

import {getFilePreviewUrl, getFileUrl} from 'mattermost-redux/utils/file_utils';

import {isImage} from 'fusion/messages/content/files';

// Shared by the topics view (fusion/forum/topics_view.tsx) and the image board view
// (fusion/forum/imageboard_view.tsx): the same "first image" and thumbnail rules, so the two views agree on what
// counts as a topic's (or a reply's) picture.
export function firstImageFile(post: Post): FileInfo | undefined {
    return post.metadata?.files?.find((f) => isImage(f, false));
}

export function thumbSrc(file: FileInfo): string {
    const ext = (file.extension || '').toLowerCase();
    return file.has_preview_image && ext !== 'gif' ? getFilePreviewUrl(file.id) : getFileUrl(file.id);
}
