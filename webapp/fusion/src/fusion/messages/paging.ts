// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {getLatestPostId, getOldestPostId} from 'utils/post_utils';

// pageEdges gives the posts a channel's list pages from, older and newer, out of its prepared rows (newest first).
// They are always real posts: a folded row of join and leave messages ("user-activity-…") has an id the server
// doesn't know, and asking for the posts before it never ends the paging (the answer is dropped, or refused).
export function pageEdges(items: string[]): {oldest: string; newest: string} {
    return {oldest: getOldestPostId(items), newest: getLatestPostId(items)};
}
