// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {next} from './nav_shortcuts';

describe('next', () => {
    test('goes round the list both ways', () => {
        expect(next(['a', 'b', 'c'], 'b', 1)).toBe('c');
        expect(next(['a', 'b', 'c'], 'c', 1)).toBe('a');
        expect(next(['a', 'b', 'c'], 'a', -1)).toBe('c');
    });

    test('starts at an end when the current one is not in the list', () => {
        expect(next(['a', 'b'], 'x', 1)).toBe('a');
        expect(next(['a', 'b'], 'x', -1)).toBe('b');
        expect(next([], 'x', 1)).toBeUndefined();
    });
});
