// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {GlobalState} from 'types/store';
import type {ComposerModeRegistration} from 'types/store/plugins';

import {clearLoggedComposerModeErrors, getComposerModes} from './composer_modes';

function makeState(regs: ComposerModeRegistration[] = []): GlobalState {
    return {plugins: {components: {ComposerMode: regs}}} as unknown as GlobalState;
}

function makeMode(partial: Partial<ComposerModeRegistration> = {}): ComposerModeRegistration {
    return {
        id: 'mode-1',
        pluginId: 'test-plugin',
        label: 'Poll',
        component: () => null,
        shouldRender: () => true,
        ...partial,
    };
}

describe('selectors/getComposerModes', () => {
    beforeEach(() => {
        clearLoggedComposerModeErrors();
    });

    it('returns nothing without registrations or channel ID', () => {
        expect(getComposerModes(makeState(), {channelId: 'channel'})).toEqual([]);
        expect(getComposerModes(makeState([makeMode()]), {channelId: ''})).toEqual([]);
    });

    it('returns the registrations themselves when all of them render', () => {
        const regs = [makeMode({id: 'a'}), makeMode({id: 'b'})];
        expect(getComposerModes(makeState(regs), {channelId: 'channel'})).toBe(regs);
    });

    it('keeps the modes whose shouldRender returns true for the channel and thread', () => {
        const inThreads = makeMode({id: 'threads', shouldRender: (_state, ctx) => Boolean(ctx.rootId)});
        const inChannel = makeMode({id: 'channel', shouldRender: (_state, ctx) => !ctx.rootId});
        const state = makeState([inThreads, inChannel]);

        expect(getComposerModes(state, {channelId: 'channel'})).toEqual([inChannel]);
        expect(getComposerModes(state, {channelId: 'channel', rootId: 'root'})).toEqual([inThreads]);
    });

    it('hides a mode whose shouldRender throws and logs once per plugin', () => {
        const consoleError = jest.spyOn(console, 'error').mockImplementation(() => {});
        const throwing = makeMode({
            id: 'throwing',
            shouldRender: () => {
                throw new Error('boom');
            },
        });
        const fine = makeMode({id: 'fine', pluginId: 'other-plugin'});
        const state = makeState([throwing, fine]);

        expect(getComposerModes(state, {channelId: 'channel'})).toEqual([fine]);
        expect(getComposerModes(state, {channelId: 'channel'})).toEqual([fine]);
        expect(consoleError).toHaveBeenCalledTimes(1);

        consoleError.mockRestore();
    });
});
