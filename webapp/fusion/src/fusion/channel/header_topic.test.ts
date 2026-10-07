// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {createIntl} from 'react-intl';

import {lastOnlineAgo} from './header_topic';

describe('fusion/channel/lastOnlineAgo', () => {
    const intl = createIntl({locale: 'en', messages: {}});
    const now = Date.UTC(2026, 9, 8, 12, 0);

    test('counts in the largest whole unit', () => {
        expect(lastOnlineAgo(intl, now - (6 * 60000), now)).toBe('6 minutes ago');
        expect(lastOnlineAgo(intl, now - (3 * 3600000), now)).toBe('3 hours ago');
        expect(lastOnlineAgo(intl, now - (2 * 86400000), now)).toBe('2 days ago');
        expect(lastOnlineAgo(intl, now - 86400000, now)).toBe('yesterday');
    });
});
