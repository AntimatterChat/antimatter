// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect, useRef} from 'react';
import {useSelector} from 'react-redux';
import {useLocation, useRouteMatch} from 'react-router-dom';

import {getPost, getRecentPostsChunkInChannel} from 'mattermost-redux/selectors/entities/posts';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import type {PermalinkAnchorState} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

export type PermalinkAnchor = {

    // The message the address names: it's highlighted.
    linkedPostId?: string;

    // The message the conversation's list opens around: the linked one, and still that one once the address drops
    // it a few seconds later (it's then in the address's state), so the list neither reloads nor jumps to the newest
    // messages under the reader.
    anchorPostId?: string;
};

// usePermalinkAnchor is the message a link opened in the conversation shown. While a permalink resolves to its
// conversation, the conversation shown keeps its own. Sending a message while the list is around an older message
// brings back the newest messages, with the one sent.
export function usePermalinkAnchor(channelId: string | undefined, resolvingPermalink: boolean): PermalinkAnchor {
    const match = useRouteMatch<{postid?: string}>();
    const location = useLocation<PermalinkAnchorState | undefined>();
    const shown = useRef<{channelId?: string; postId?: string}>({});

    let linkedPostId: string | undefined;
    let anchorPostId: string | undefined;
    if (resolvingPermalink) {
        anchorPostId = shown.current.channelId === channelId ? shown.current.postId : undefined;
    } else {
        linkedPostId = match.params.postid;
        anchorPostId = linkedPostId || location.state?.permalinkAnchor || undefined;
        shown.current = {channelId, postId: anchorPostId};
    }

    // The newest message of the conversation, when the reader sent it (it keeps its pending id once sent).
    const mine = useSelector((state: GlobalState) => {
        const id = channelId ? getRecentPostsChunkInChannel(state, channelId)?.order[0] : undefined;
        const post = id ? getPost(state, id) : undefined;
        return post && post.user_id === getCurrentUserId(state) ? post.pending_post_id || post.id : undefined;
    });
    const kept = anchorPostId && !linkedPostId ? anchorPostId : undefined;
    const last = useRef({channelId, mine});
    useEffect(() => {
        const previous = last.current;
        last.current = {channelId, mine};
        if (kept && mine && previous.channelId === channelId && previous.mine !== mine) {
            getHistory().replace(location.pathname);
        }
    }, [channelId, mine, kept, location.pathname]);

    return {linkedPostId, anchorPostId};
}
