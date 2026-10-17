// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {defineMessages} from 'react-intl';
import type {IntlShape} from 'react-intl';

import type {ChannelCategory} from '@mattermost/types/channel_categories';

// The server names the default categories in English ("Favourites", "Channels", "Direct Messages") and keeps that
// name, so Fusion shows its own label for them, in the user's language, as the classic sidebar does. The categories
// people make, and the admins' server categories, are shown by the name they were given.
const defaultCategoryNames = defineMessages({
    favorites: {id: 'sidebar_left.sidebar_channel_menu.favorites', defaultMessage: 'Favourites'},
    channels: {id: 'sidebar_left.sidebar_channel_menu.channels', defaultMessage: 'Channels'},
    direct_messages: {id: 'fusion.home.title', defaultMessage: 'Direct messages'},
});

// categoryName is the name to show for a category: the translated label of a default one, else its own name. Code
// that matches categories by name (server categories) keeps using display_name.
export function categoryName(intl: Pick<IntlShape, 'formatMessage'>, category: Pick<ChannelCategory, 'type' | 'display_name'>): string {
    const message = defaultCategoryNames[category.type as keyof typeof defaultCategoryNames];
    return message ? intl.formatMessage(message) : category.display_name;
}
