// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {GlobalState} from 'types/store';
import type {EmojiPickerTabRegistration} from 'types/store/plugins';

import {clearLoggedEmojiPickerTabErrors, getEmojiPickerTabs, isGifPickerReplaced} from './emoji_picker_tabs';

function makeState(regs: EmojiPickerTabRegistration[] = []): GlobalState {
    return {
        plugins: {
            components: {
                EmojiPickerTab: regs,
            },
        },
    } as unknown as GlobalState;
}

function makeTab(partial: Partial<EmojiPickerTabRegistration> = {}): EmojiPickerTabRegistration {
    return {
        id: 'tab-1',
        pluginId: 'test-plugin',
        label: 'Tab',
        order: 0,
        component: () => null,
        shouldRender: () => true,
        replacesGifPicker: false,
        ...partial,
    };
}

describe('selectors/getEmojiPickerTabs', () => {
    beforeEach(() => {
        clearLoggedEmojiPickerTabErrors();
    });

    it('returns nothing without registrations or channel ID', () => {
        expect(getEmojiPickerTabs(makeState(), {channelId: 'channel'})).toEqual([]);
        expect(getEmojiPickerTabs(makeState([makeTab()]), {channelId: ''})).toEqual([]);
    });

    it('sorts the tabs by order and keeps the registration order otherwise', () => {
        const stickers = makeTab({id: 'stickers', order: 2});
        const gifs = makeTab({id: 'gifs', order: 1});
        const other = makeTab({id: 'other', order: 1});
        const regs = [stickers, gifs, other];

        expect(getEmojiPickerTabs(makeState(regs), {channelId: 'channel'})).toEqual([gifs, other, stickers]);
        expect(regs).toEqual([stickers, gifs, other]);
    });

    it('keeps the tabs whose shouldRender returns true for the channel and thread', () => {
        const inThreads = makeTab({id: 'threads', shouldRender: (_state, ctx) => Boolean(ctx.rootId)});
        const inChannel = makeTab({id: 'channel', shouldRender: (_state, ctx) => ctx.channelId === 'channel' && !ctx.rootId});
        const state = makeState([inThreads, inChannel]);

        expect(getEmojiPickerTabs(state, {channelId: 'channel'})).toEqual([inChannel]);
        expect(getEmojiPickerTabs(state, {channelId: 'channel', rootId: 'root'})).toEqual([inThreads]);
    });

    it('passes the state to shouldRender', () => {
        const shouldRender = jest.fn(() => true);
        const state = makeState([makeTab({shouldRender})]);
        getEmojiPickerTabs(state, {channelId: 'channel', rootId: 'root'});
        expect(shouldRender).toHaveBeenCalledWith(state, {channelId: 'channel', rootId: 'root'});
    });

    it('hides a tab whose shouldRender throws and logs once per plugin', () => {
        const consoleError = jest.spyOn(console, 'error').mockImplementation(() => {});
        const throwing = makeTab({
            id: 'throwing',
            shouldRender: () => {
                throw new Error('boom');
            },
        });
        const fine = makeTab({id: 'fine', pluginId: 'other-plugin'});
        const state = makeState([throwing, fine]);

        expect(getEmojiPickerTabs(state, {channelId: 'channel'})).toEqual([fine]);
        expect(getEmojiPickerTabs(state, {channelId: 'channel'})).toEqual([fine]);
        expect(consoleError).toHaveBeenCalledTimes(1);

        consoleError.mockRestore();
    });
});

describe('selectors/isGifPickerReplaced', () => {
    beforeEach(() => {
        clearLoggedEmojiPickerTabErrors();
    });

    it('is false without a tab replacing the GIF picker', () => {
        expect(isGifPickerReplaced(makeState(), {channelId: 'channel'})).toBe(false);
        expect(isGifPickerReplaced(makeState([makeTab()]), {channelId: 'channel'})).toBe(false);
    });

    it('is true when a replacing tab renders in the message box', () => {
        const gifs = makeTab({replacesGifPicker: true, shouldRender: (_state, ctx) => ctx.channelId === 'gifs'});
        const state = makeState([makeTab(), gifs]);

        expect(isGifPickerReplaced(state, {channelId: 'gifs'})).toBe(true);
        expect(isGifPickerReplaced(state, {channelId: 'other'})).toBe(false);
    });

    it('is false when the shouldRender of the replacing tab throws', () => {
        const consoleError = jest.spyOn(console, 'error').mockImplementation(() => {});
        const state = makeState([makeTab({
            replacesGifPicker: true,
            shouldRender: () => {
                throw new Error('boom');
            },
        })]);

        expect(isGifPickerReplaced(state, {channelId: 'channel'})).toBe(false);

        consoleError.mockRestore();
    });
});
