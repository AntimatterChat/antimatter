// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {FileInfo} from '@mattermost/types/files';

import {setGlobalItem} from 'actions/storage';
import {updateDraft} from 'actions/views/drafts';
import globalStore from 'stores/redux_store';

import {act, renderHookWithContext} from 'tests/react_testing_utils';

import type {PostDraft} from 'types/store/draft';

import {DRAFT_SAVE_DELAY, useDraftAutosave} from './draft_autosave';

// Saving a draft stores it, as the real action does, without the server.
jest.mock('actions/views/drafts', () => {
    const storage = jest.requireActual('actions/storage');
    return {updateDraft: jest.fn((key: string, value: unknown) => storage.setGlobalItem(key, value))};
});

const fileOf = (id: string) => ({id, name: id + '.mp4'}) as FileInfo;
const storedDraft = (key: string) => (globalStore.getState() as {storage: {storage: Record<string, {value: PostDraft}>}}).storage.storage[key]?.value;

const draftOf = (channelId: string, message: string): PostDraft => ({message, fileInfos: [], uploadsInProgress: [], channelId, rootId: '', createAt: 1, updateAt: 1});

describe('fusion/composer/useDraftAutosave', () => {
    let channelId = 'a';
    const render = (storage: Record<string, unknown> = {}) => renderHookWithContext(
        () => useDraftAutosave(channelId),
        {storage: {initialized: true, storage: Object.fromEntries(Object.entries(storage).map(([k, value]) => [k, {value, timestamp: new Date()}]))}} as never,
    );

    beforeEach(() => {
        jest.useFakeTimers();
        jest.clearAllMocks();
        channelId = 'a';
    });
    afterEach(() => {
        jest.useRealTimers();
    });

    test('saves the draft once the typing pauses, for the Drafts list and the other devices', () => {
        const {result} = render();

        act(() => result.current.save(draftOf('a', 'hel')));
        act(() => result.current.save(draftOf('a', 'hello')));
        expect(result.current.draft.message).toBe('hello');
        expect(updateDraft).not.toHaveBeenCalled();

        act(() => {
            jest.advanceTimersByTime(DRAFT_SAVE_DELAY);
        });
        expect(updateDraft).toHaveBeenCalledTimes(1);
        expect(updateDraft).toHaveBeenCalledWith('draft_a', expect.objectContaining({message: 'hello', show: true}), '', true);
    });

    test('keeps a draft written just before leaving the channel, and shows it on coming back', () => {
        const {result, rerender} = render();
        act(() => result.current.save(draftOf('a', 'half a thought')));

        channelId = 'b';
        rerender();
        expect(updateDraft).toHaveBeenCalledWith('draft_a', expect.objectContaining({message: 'half a thought'}), '', true);
        expect(result.current.draft.message).toBe('');

        channelId = 'a';
        rerender();
        expect(result.current.draft.message).toBe('half a thought');

        // Nothing is saved twice when the timer would have run.
        act(() => {
            jest.advanceTimersByTime(DRAFT_SAVE_DELAY);
        });
        expect(updateDraft).toHaveBeenCalledTimes(1);
    });

    test('saves at once when the page is closed or reloaded', () => {
        const {result} = render();
        act(() => result.current.save(draftOf('a', 'before reload')));

        act(() => {
            window.dispatchEvent(new Event('pagehide'));
        });
        expect(updateDraft).toHaveBeenCalledWith('draft_a', expect.objectContaining({message: 'before reload'}), '', true);
    });

    test('opens with the stored draft of the channel', () => {
        const {result} = render({draft_a: draftOf('a', 'from last time')});

        expect(result.current.draft.message).toBe('from last time');
    });

    test('fills the empty composer with a draft read back later (after a reload, or from the server)', () => {
        const {result} = render();
        expect(result.current.draft.message).toBe('');

        act(() => {
            globalStore.dispatch(setGlobalItem('draft_a', draftOf('a', 'synced from my phone')));
        });
        expect(result.current.draft.message).toBe('synced from my phone');
    });

    test('never replaces what is being written with a stored draft', () => {
        const {result} = render();
        act(() => result.current.save(draftOf('a', 'typing now')));

        act(() => {
            globalStore.dispatch(setGlobalItem('draft_a', draftOf('a', 'older')));
        });
        expect(result.current.draft.message).toBe('typing now');
    });

    test('drops the waiting save of a message just sent', () => {
        const {result} = render();
        act(() => result.current.save(draftOf('a', 'sent')));
        act(() => result.current.cancel());

        act(() => {
            jest.advanceTimersByTime(DRAFT_SAVE_DELAY);
            window.dispatchEvent(new Event('pagehide'));
        });
        expect(updateDraft).not.toHaveBeenCalled();
    });

    // audit-1104 fusion-core-1 / fusion-ui-1: an upload that finished after leaving the conversation was dropped, and
    // one that finished while a save was waiting was overwritten by it a moment later.
    describe('addFiles (finished uploads)', () => {
        test('adds the file to what is shown, and stores it', () => {
            const {result} = render({draft_a: draftOf('a', 'look')});

            act(() => result.current.addFiles([fileOf('f1')]));

            expect(result.current.draft.fileInfos.map((f) => f.id)).toEqual(['f1']);
            expect(storedDraft('draft_a')).toEqual(expect.objectContaining({message: 'look', fileInfos: [expect.objectContaining({id: 'f1'})], show: true}));
        });

        test('a save waiting when the upload finishes keeps the file', () => {
            const {result} = render();
            act(() => result.current.save(draftOf('a', 'a')));

            act(() => result.current.addFiles([fileOf('f1')]));
            act(() => {
                jest.advanceTimersByTime(DRAFT_SAVE_DELAY);
            });

            expect(updateDraft).toHaveBeenLastCalledWith('draft_a', expect.objectContaining({message: 'a', fileInfos: [expect.objectContaining({id: 'f1'})]}), '', true);
            expect(storedDraft('draft_a')).toEqual(expect.objectContaining({message: 'a', fileInfos: [expect.objectContaining({id: 'f1'})]}));
        });

        test('an upload that finishes after the composer closed goes into the stored draft of its conversation', () => {
            const {result, unmount} = render();
            act(() => result.current.save(draftOf('a', 'the video')));
            const addFiles = result.current.addFiles;

            unmount();
            act(() => addFiles([fileOf('f1')]));

            expect(storedDraft('draft_a')).toEqual(expect.objectContaining({message: 'the video', channelId: 'a', fileInfos: [expect.objectContaining({id: 'f1'})], show: true}));
        });

        test('an upload started in another conversation goes into that one, not into the draft shown now', () => {
            const {result, rerender} = render({draft_b: draftOf('b', 'other')});
            const addFiles = result.current.addFiles;

            channelId = 'b';
            rerender();
            act(() => addFiles([fileOf('f1')]));

            expect(result.current.draft.fileInfos).toEqual([]);
            expect(storedDraft('draft_b')?.fileInfos).toEqual([]);
            expect(storedDraft('draft_a')).toEqual(expect.objectContaining({channelId: 'a', fileInfos: [expect.objectContaining({id: 'f1'})]}));
        });
    });
});
