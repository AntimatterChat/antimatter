// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {ChannelCategory, ServerCategory} from '@mattermost/types/channel_categories';

import {layoutSidebar} from './server_categories';

const personal = (id: string, type: string, channelIds: string[]) => ({id, type, display_name: id, channel_ids: channelIds}) as unknown as ChannelCategory;
const server = (name: string, locked = false) => ({name, locked}) as ServerCategory;

describe('layoutSidebar', () => {
    const mine = [
        personal('favs', 'favorites', ['fav']),
        personal('channels', 'channels', ['rules', 'general', 'kept', 'plain']),
        personal('stuff', 'custom', ['moved', 'locked-moved']),
    ];
    const mappings = {rules: 'Rules', general: 'Community', moved: 'Community', 'locked-moved': 'Rules', fav: 'Community'};
    const list = [server('Rules', true), server('Community')];

    const layout = (showEmpty = false, servers = list, maps: Record<string, string> = mappings) => layoutSidebar(mine, maps, servers, [], 'me', 'team', showEmpty);
    const ids = (categories: ChannelCategory[], id: string) => categories.find((c) => c.id === id)?.channel_ids;

    test('server categories come first, in their order', () => {
        expect(layout().map((c) => c.id)).toEqual(['managed_Rules', 'managed_Community', 'favs', 'channels', 'stuff']);
    });

    test('a channel moved into your own category or favourites stays there, unless its category is locked', () => {
        const categories = layout();
        expect(ids(categories, 'managed_Community')).toEqual(['general']);
        expect(ids(categories, 'stuff')).toEqual(['moved']);
        expect(ids(categories, 'favs')).toEqual(['fav']);
        expect(ids(categories, 'managed_Rules')).toEqual(['rules', 'locked-moved']);
    });

    test('channels shown above leave your Channels category', () => {
        expect(ids(layout(), 'channels')).toEqual(['kept', 'plain']);
    });

    test('empty server categories show to admins only', () => {
        const servers = [...list, server('Empty')];
        expect(layout(false, servers).some((c) => c.id === 'managed_Empty')).toBe(false);
        expect(layout(true, servers).some((c) => c.id === 'managed_Empty')).toBe(true);
    });

    test('names the list lacks come after it, open; names match ignoring case', () => {
        const categories = layout(false, list, {...mappings, kept: 'Archive', plain: 'community'});
        expect(categories.slice(0, 3).map((c) => c.id)).toEqual(['managed_Rules', 'managed_Community', 'managed_Archive']);
        expect(ids(categories, 'managed_Community')).toEqual(['general', 'plain']);
    });
});
