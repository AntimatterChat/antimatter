// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {plainText} from './plain_text';

describe('fusion/utils/plain_text', () => {
    test.each([
        ['**bold** and _it_ and ~~gone~~', 'bold and it and gone'],
        ['see [the docs](https://example.com) now', 'see the docs now'],
        ['# Title\n> quoted\n- item', 'Title quoted item'],
        ['before ```js\nconst a = 1;\n``` after', 'before [code] after'],
        ['`code` stays', 'code stays'],
        ['snake_case_name stays', 'snake_case_name stays'],
    ])('%j', (input, expected) => {
        expect(plainText(input)).toBe(expected);
    });

    test('cuts long text', () => {
        expect(plainText('a'.repeat(400), 10)).toBe('aaaaaaaaaa');
    });
});
