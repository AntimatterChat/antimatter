// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Channel} from '@mattermost/types/channels';

import type {GlobalState} from 'types/store';
import type {ChannelViewPanelRegistration} from 'types/store/plugins';

import {clearLoggedChannelViewPanelErrors, getChannelViewPanel} from './channel_view_panel';

function makeChannel(partial: Partial<Channel> = {}): Channel {
    return {
        id: 'channel-1',
        type: 'O',
        delete_at: 0,
        ...partial,
    } as Channel;
}

function makeState(
    regs: ChannelViewPanelRegistration[] = [],
    channels: Record<string, Channel> = {},
): GlobalState {
    return {
        plugins: {
            components: {
                ChannelViewPanel: regs,
            },
        },
        entities: {
            channels: {
                channels,
            },
        },
    } as unknown as GlobalState;
}

function makeRegistration(partial: Partial<ChannelViewPanelRegistration> = {}): ChannelViewPanelRegistration {
    return {
        id: 'reg-1',
        pluginId: 'test-plugin',
        matcher: () => true,
        component: () => null,
        ...partial,
    } as ChannelViewPanelRegistration;
}

describe('selectors/getChannelViewPanel', () => {
    beforeEach(() => {
        clearLoggedChannelViewPanelErrors();
    });

    it('returns null without registrations, channel ID or channel', () => {
        const channel = makeChannel();
        expect(getChannelViewPanel(makeState([], {[channel.id]: channel}), channel.id)).toBeNull();
        expect(getChannelViewPanel(makeState([makeRegistration()], {[channel.id]: channel}), '')).toBeNull();
        expect(getChannelViewPanel(makeState([makeRegistration()], {}), channel.id)).toBeNull();
    });

    it('returns the first registration whose matcher returns true', () => {
        const channel = makeChannel();
        const first = makeRegistration({id: 'first', matcher: () => false});
        const second = makeRegistration({id: 'second', matcher: (_state, c) => c.id === channel.id});
        const third = makeRegistration({id: 'third'});
        const state = makeState([first, second, third], {[channel.id]: channel});
        expect(getChannelViewPanel(state, channel.id)).toBe(second);
    });

    it('treats a throwing matcher as no-match and logs once per plugin', () => {
        const channel = makeChannel();
        const consoleError = jest.spyOn(console, 'error').mockImplementation(() => {});
        const throwing = makeRegistration({
            id: 'throwing',
            matcher: () => {
                throw new Error('boom');
            },
        });
        const state = makeState([throwing], {[channel.id]: channel});

        expect(getChannelViewPanel(state, channel.id)).toBeNull();
        expect(getChannelViewPanel(state, channel.id)).toBeNull();
        expect(consoleError).toHaveBeenCalledTimes(1);

        consoleError.mockRestore();
    });
});
