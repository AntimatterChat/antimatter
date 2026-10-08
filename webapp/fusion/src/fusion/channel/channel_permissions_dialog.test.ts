// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {lineState, setLine} from './channel_permissions_dialog';

describe('channel permission lines', () => {
    const both = ['delete_others_posts', 'edit_others_posts'];

    test('a line is denied or allowed only when all its permissions are', () => {
        expect(lineState({allow: [], deny: both}, both)).toBe('deny');
        expect(lineState({allow: both, deny: []}, both)).toBe('allow');
        expect(lineState({allow: ['edit_others_posts'], deny: []}, both)).toBe('inherit');
    });

    test('setting a line moves its permissions between allow and deny, leaving the others', () => {
        const start = {allow: ['create_post'], deny: ['upload_file']};
        expect(setLine(start, both, 'deny')).toEqual({allow: ['create_post'], deny: ['upload_file', ...both]});
        const denied = setLine(start, both, 'deny');
        expect(setLine(denied, both, 'allow')).toEqual({allow: ['create_post', ...both], deny: ['upload_file']});
        expect(setLine(denied, both, 'inherit')).toEqual(start);
    });
});
