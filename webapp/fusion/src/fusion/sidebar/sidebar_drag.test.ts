// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {ChannelCategory} from '@mattermost/types/channel_categories';

import {canDropChannel, categoryDropIndex, channelDropIndex} from './sidebar_drag';

describe('fusion/sidebar/sidebar_drag', () => {
    const category = (type: ChannelCategory['type']) => ({id: type, type} as ChannelCategory);
    const channel = (directMessage = false) => ({kind: 'channel' as const, channelId: 'b', categoryId: 'channels', directMessage});

    test('lets channels go where the classic sidebar does', () => {
        expect(canDropChannel(channel(), category('channels'))).toBe(true);
        expect(canDropChannel(channel(), category('custom'))).toBe(true);
        expect(canDropChannel(channel(), category('direct_messages'))).toBe(false);
        expect(canDropChannel(channel(true), category('channels'))).toBe(false);
        expect(canDropChannel(channel(true), category('favorites'))).toBe(true);
        expect(canDropChannel({kind: 'category', categoryId: 'x'}, category('custom'))).toBe(false);
        expect(canDropChannel(null, category('custom'))).toBe(false);
    });

    test('places a channel before or after another, within its category or from another', () => {
        const ids = ['a', 'b', 'c', 'd'];

        // b moving down after c, or up before a; staying put is no move.
        expect(channelDropIndex(ids, 'b', 'c', true)).toBe(2);
        expect(channelDropIndex(ids, 'b', 'a', false)).toBe(0);
        expect(channelDropIndex(ids, 'b', 'c', false)).toBe(-1);
        expect(channelDropIndex(ids, 'b', 'a', true)).toBe(-1);

        // From another category, nothing is taken out first; on the header, first.
        expect(channelDropIndex(['x', 'y'], 'b', 'y', true)).toBe(2);
        expect(channelDropIndex(['x', 'y'], 'b', null, false)).toBe(0);
    });

    test('places a category before or after another', () => {
        const order = ['favorites', 'one', 'two', 'channels', 'direct_messages'];

        expect(categoryDropIndex(order, 'one', 'channels', true)).toBe(3);
        expect(categoryDropIndex(order, 'two', 'favorites', false)).toBe(0);
        expect(categoryDropIndex(order, 'one', 'two', false)).toBe(-1);
    });
});
