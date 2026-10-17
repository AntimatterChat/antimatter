// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect} from 'react';
import {useDispatch, useSelector} from 'react-redux';

import {syncedDraftsAreAllowedAndEnabled} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';

import {getDrafts} from 'actions/views/drafts';
import {getGlobalItem} from 'selectors/storage';

import {StoragePrefixes} from 'utils/constants';

import type {GlobalState} from 'types/store';
import type {PostDraft} from 'types/store/draft';

// hasChannelDraft tells whether a message to the channel (not a reply in one of its threads) waits in its draft:
// written here and kept in this browser, or synced from another device.
export function hasChannelDraft(state: GlobalState, channelId: string): boolean {
    const draft = getGlobalItem<Partial<PostDraft> | null>(state, StoragePrefixes.DRAFT + channelId, null);
    return Boolean(draft && (draft.message?.trim() || draft.fileInfos?.length));
}

// useSyncedDrafts reads the drafts kept on the server for the current team, when drafts are synced, so that a
// message started on another device (or before this browser's storage was cleared) can be finished here. The classic
// web app does this from its Drafts link, which the Fusion sidebar doesn't have.
export function useSyncedDrafts() {
    const dispatch = useDispatch();
    const teamId = useSelector(getCurrentTeamId);
    const synced = useSelector(syncedDraftsAreAllowedAndEnabled);
    useEffect(() => {
        if (teamId && synced) {
            dispatch(getDrafts(teamId));
        }
    }, [dispatch, teamId, synced]);
}
