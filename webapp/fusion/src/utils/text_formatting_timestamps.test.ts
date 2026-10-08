// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {formatText, formatTimestamp} from 'utils/text_formatting';

const emojiMap = {has: () => false, get: () => undefined} as any; // eslint-disable-line @typescript-eslint/no-explicit-any

describe('Antimatter markdown extensions', () => {
    test('dynamic timestamps show in the reader\'s format', () => {
        const at = Date.UTC(2026, 9, 8, 12, 0, 0) / 1000;
        expect(formatTimestamp(at, 'D', Date.now(), 'en-US')).toMatch(/October [78], 2026/);
        expect(formatTimestamp(at - 3600, 'R', at * 1000, 'en-US')).toBe('1 hour ago');
        expect(formatTimestamp(at + (2 * 86400), 'R', at * 1000, 'en-US')).toBe('in 2 days');
    });

    test('a <t:…> token is replaced in messages, and only a valid one', () => {
        const html = formatText('Meet at <t:1791460800:t> please', {}, emojiMap);
        expect(html).toContain('class="markdown__timestamp"');
        expect(html).not.toContain('&lt;t:');
        expect(formatText('not <t:abc:R>', {}, emojiMap)).toContain('&lt;t:abc:R&gt;');
    });

    test('a line starting with -# is subtext', () => {
        const html = formatText('Hello\n-# small print', {}, emojiMap);
        expect(html).toContain('<span class="markdown__subtext">small print</span>');
        expect(formatText('a -# b', {}, emojiMap)).not.toContain('markdown__subtext');
    });
});
