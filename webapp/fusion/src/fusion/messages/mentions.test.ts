// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {mentions} from './mentions';

describe('fusion/messages/mentions', () => {
    const keys = [{key: '@marie'}, {key: '@here'}, {key: 'Marie', caseSensitive: true}, {key: 'deploy'}];

    test.each([
        ['hi @marie', true],
        ['@MARIE, look', true],
        ['thanks @marie.', true],
        ['@here the build is red', true],
        ['ask Marie', true],
        ['ask marie', false],
        ['@marie.dupont is away', false],
        ['@mariea', false],
        ['write to marie@example.com', false],
        ['the deploy failed', true],
        ['redeploy it', false],
        ['`@marie` in code', false],
        ['```\n@here\n```', false],
        ['', false],
    ])('%j mentions: %s', (message, expected) => {
        expect(mentions(message, keys)).toBe(expected);
    });
});
