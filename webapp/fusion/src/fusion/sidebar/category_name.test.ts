// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import fs from 'fs';
import path from 'path';

import {createIntl} from 'react-intl';

import type {ChannelCategory} from '@mattermost/types/channel_categories';

import {categoryName} from './category_name';

// The tests mock every i18n/*.json import: read the real French catalog.
const frMessages: Record<string, string> = JSON.parse(fs.readFileSync(path.join(__dirname, '..', '..', 'i18n', 'fr.json'), 'utf8'));

const category = (type: ChannelCategory['type'], displayName: string) => ({type, display_name: displayName});

describe('fusion/sidebar/categoryName', () => {
    const fr = createIntl({locale: 'fr', messages: frMessages});
    const en = createIntl({locale: 'en'});

    test('the default categories have their names in the user\'s language, not the one the server stored', () => {
        expect(categoryName(fr, category('favorites', 'Favourites'))).toBe('Favoris');
        expect(categoryName(fr, category('favorites', 'Favorites'))).toBe('Favoris');
        expect(categoryName(fr, category('channels', 'Channels'))).toBe('Canaux');
        expect(categoryName(fr, category('direct_messages', 'Direct Messages'))).toBe('Messages personnels');
    });

    test('custom and server categories keep their own names', () => {
        expect(categoryName(fr, category('custom', 'Work'))).toBe('Work');
        expect(categoryName(fr, category('custom', 'Favorites'))).toBe('Favorites');
        expect(categoryName(fr, category('managed', 'Staff'))).toBe('Staff');
    });

    test('in English, Favourites is spelt the British way', () => {
        expect(categoryName(en, category('favorites', 'Favorites'))).toBe('Favourites');
        expect(categoryName(en, category('channels', 'Channels'))).toBe('Channels');
        expect(categoryName(en, category('direct_messages', 'Direct Messages'))).toBe('Direct messages');
    });
});
