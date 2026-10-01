// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {GlobalState} from 'types/store';
import type {ComposerMenuItemRegistration} from 'types/store/plugins';

import {clearLoggedComposerMenuItemErrors, getComposerMenuItems} from './composer_menu';

function makeState(regs: ComposerMenuItemRegistration[] = []): GlobalState {
    return {
        plugins: {
            components: {
                ComposerMenuItem: regs,
            },
        },
    } as unknown as GlobalState;
}

function makeItem(partial: Partial<ComposerMenuItemRegistration> = {}): ComposerMenuItemRegistration {
    return {
        id: 'item-1',
        pluginId: 'test-plugin',
        text: 'Item',
        icon: null,
        action: jest.fn(),
        shouldRender: () => true,
        ...partial,
    };
}

describe('selectors/getComposerMenuItems', () => {
    beforeEach(() => {
        clearLoggedComposerMenuItemErrors();
    });

    it('returns nothing without registrations or channel ID', () => {
        expect(getComposerMenuItems(makeState(), {channelId: 'channel'})).toEqual([]);
        expect(getComposerMenuItems(makeState([makeItem()]), {channelId: ''})).toEqual([]);
    });

    it('returns the registrations themselves when all of them render', () => {
        const regs = [makeItem({id: 'a'}), makeItem({id: 'b'})];
        const state = makeState(regs);
        expect(getComposerMenuItems(state, {channelId: 'channel'})).toBe(regs);
    });

    it('keeps the items whose shouldRender returns true for the channel and thread', () => {
        const inThreads = makeItem({id: 'threads', shouldRender: (_state, ctx) => Boolean(ctx.rootId)});
        const inChannel = makeItem({id: 'channel', shouldRender: (_state, ctx) => ctx.channelId === 'channel' && !ctx.rootId});
        const state = makeState([inThreads, inChannel]);

        expect(getComposerMenuItems(state, {channelId: 'channel'})).toEqual([inChannel]);
        expect(getComposerMenuItems(state, {channelId: 'channel', rootId: 'root'})).toEqual([inThreads]);
    });

    it('passes the state to shouldRender', () => {
        const shouldRender = jest.fn(() => true);
        const state = makeState([makeItem({shouldRender})]);
        getComposerMenuItems(state, {channelId: 'channel', rootId: 'root'});
        expect(shouldRender).toHaveBeenCalledWith(state, {channelId: 'channel', rootId: 'root'});
    });

    it('hides an item whose shouldRender throws and logs once per plugin', () => {
        const consoleError = jest.spyOn(console, 'error').mockImplementation(() => {});
        const throwing = makeItem({
            id: 'throwing',
            shouldRender: () => {
                throw new Error('boom');
            },
        });
        const fine = makeItem({id: 'fine', pluginId: 'other-plugin'});
        const state = makeState([throwing, fine]);

        expect(getComposerMenuItems(state, {channelId: 'channel'})).toEqual([fine]);
        expect(getComposerMenuItems(state, {channelId: 'channel'})).toEqual([fine]);
        expect(consoleError).toHaveBeenCalledTimes(1);

        consoleError.mockRestore();
    });
});
