// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import type {Dispatch, SetStateAction} from 'react';
import {useDispatch, useSelector, useStore} from 'react-redux';

import type {FileInfo} from '@mattermost/types/files';

import {updateDraft} from 'actions/views/drafts';
import {makeGetDraft} from 'selectors/drafts';

import {StoragePrefixes} from 'utils/constants';

import type {GlobalState} from 'types/store';
import type {PostDraft} from 'types/store/draft';

// How long after the last keystroke a draft is saved.
export const DRAFT_SAVE_DELAY = 500;

export function isDraftEmpty(draft: Pick<PostDraft, 'message' | 'fileInfos'>): boolean {
    return !draft.message && !draft.fileInfos?.length;
}

type DraftAutosave = {

    /** The draft shown in the composer. */
    draft: PostDraft;

    /** Shows a draft without saving it (sending it, putting back one that couldn't be sent). */
    setDraft: Dispatch<SetStateAction<PostDraft>>;

    /** Shows a changed draft and saves it a moment later, once the typing pauses. */
    save: (next: PostDraft) => void;

    /** Drops the save still waiting (the message was just sent). */
    cancel: () => void;

    /**
     * Adds finished uploads to the draft they were started in, also when the composer has left that conversation
     * (or closed) since: they're added to its stored draft then. Keep the function from the time of the upload.
     */
    addFiles: (fileInfos: FileInfo[]) => void;

    /** The draft's key in the stored drafts. */
    key: string;
};

// useDraftAutosave keeps the draft of a conversation's composer and saves it, in this browser and, when drafts are
// synced, on the server, so that it's there when you come back: after switching conversations, closing the window
// or reloading the page, or on another device. A save still waiting when you leave the conversation or the page is
// made at once. A draft that turns up after the composer opened (the stored drafts are read back after a reload, the
// server's arrive later) fills it while it's empty.
export function useDraftAutosave(channelId: string, rootId = ''): DraftAutosave {
    const dispatch = useDispatch();
    const store = useStore<GlobalState>();
    const getDraft = useMemo(() => makeGetDraft(), []);
    const storedDraft = useSelector((state: GlobalState) => getDraft(state, channelId, rootId));
    const [draft, setDraft] = useState<PostDraft>(storedDraft);
    const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
    const pending = useRef<(() => void) | null>(null);
    const key = rootId ? StoragePrefixes.COMMENT_DRAFT + rootId : StoragePrefixes.DRAFT + channelId;

    // What addFiles needs to know when an upload finishes: the draft shown now (state set since the last render
    // included), which conversation it belongs to, and whether the composer is still there.
    const shown = useRef({draft, key, mounted: false});
    shown.current.draft = draft;
    shown.current.key = key;
    useEffect(() => {
        shown.current.mounted = true;
        return () => {
            shown.current.mounted = false;
        };
    }, []);

    const flush = useCallback(() => {
        clearTimeout(timer.current);
        const run = pending.current;
        pending.current = null;
        run?.();
    }, []);

    const cancel = useCallback(() => {
        clearTimeout(timer.current);
        pending.current = null;
    }, []);

    // Saved with show, as the classic web app does, so it's listed in Drafts.
    const save = useCallback((next: PostDraft) => {
        setDraft(next);
        shown.current.draft = next;
        clearTimeout(timer.current);
        pending.current = () => dispatch(updateDraft(key, {...next, show: true}, rootId, true));
        timer.current = setTimeout(flush, DRAFT_SAVE_DELAY);
    }, [dispatch, flush, key, rootId]);

    // Leaving the conversation saves what's waiting, under the conversation it was written in.
    useEffect(() => flush, [channelId, rootId, flush]);

    // Switch drafts with the conversation.
    useEffect(() => {
        setDraft(storedDraft);

        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [channelId, rootId]);

    // So does closing or reloading the page.
    useEffect(() => {
        window.addEventListener('pagehide', flush);
        window.addEventListener('beforeunload', flush);
        return () => {
            window.removeEventListener('pagehide', flush);
            window.removeEventListener('beforeunload', flush);
        };
    }, [flush]);

    // A stored draft that arrives while the composer is empty and nothing is being saved fills it: never over what's
    // being written.
    useEffect(() => {
        if (!pending.current && isDraftEmpty(draft) && !isDraftEmpty(storedDraft)) {
            setDraft(storedDraft);
        }

        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [storedDraft]);

    // An upload can finish after the person has left the conversation (the composer is keyed per conversation, so it
    // is gone by then): the file goes into that conversation's stored draft, as in the classic web app, instead of
    // being dropped with the composer's state. While the draft is shown, it goes into what's shown, and into the save
    // still waiting, which would otherwise store the draft from before the file a moment later.
    const addFiles = useCallback((fileInfos: FileInfo[]) => {
        if (shown.current.mounted && shown.current.key === key) {
            const next = {...shown.current.draft, channelId, rootId, fileInfos: [...shown.current.draft.fileInfos, ...fileInfos]};
            shown.current.draft = next;
            setDraft(next);
            if (pending.current) {
                pending.current = () => dispatch(updateDraft(key, {...next, show: true}, rootId, true));
            }
            dispatch(updateDraft(key, {...next, show: true}, rootId, true));
            return;
        }
        const stored = getDraft(store.getState(), channelId, rootId);
        const next = {...stored, channelId, rootId, createAt: stored.createAt || Date.now(), fileInfos: [...stored.fileInfos, ...fileInfos]};
        dispatch(updateDraft(key, {...next, show: true}, rootId, true));
    }, [channelId, dispatch, getDraft, key, rootId, store]);

    return {draft, setDraft, save, cancel, key, addFiles};
}
