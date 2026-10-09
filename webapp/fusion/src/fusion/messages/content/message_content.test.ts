// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {usesBlockRenderer} from './message_content';

describe('usesBlockRenderer', () => {
    test('leaves classic attachments to Fusion\'s cards', () => {
        expect(usesBlockRenderer({attachments: [{title: 'Build passed'}]})).toBe(false);
        expect(usesBlockRenderer({})).toBe(false);
        expect(usesBlockRenderer(undefined)).toBe(false);
    });

    test('gives blocks to their own renderer, with or without attachments', () => {
        expect(usesBlockRenderer({mm_blocks: [{type: 'text'}]})).toBe(true);
        expect(usesBlockRenderer({blocks: [{type: 'section'}], attachments: [{title: 'x'}]})).toBe(true);
        expect(usesBlockRenderer({cards: [{}]})).toBe(true);
        expect(usesBlockRenderer({mm_blocks: []})).toBe(false);
    });
});
