// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import Typing from './typing';

describe('fusion/composer/Typing', () => {
    const names = ['alice', 'bob', 'carol', 'dave'];
    const profiles = Object.fromEntries(names.map((n) => [n, TestHelper.getUserMock({id: n, username: n})]));

    const render = (typing: string[], rootId = '') => renderWithContext(
        <Typing
            channelId='ch'
            rootId={rootId}
        />,
        {
            entities: {
                users: {currentUserId: 'me', profiles},
                typing: {[rootId ? `ch${rootId}` : 'ch']: Object.fromEntries(typing.map((id) => [id, {now: Date.now(), expires: Date.now() + 5000}]))},
            },
        } as never,
    );

    test('says nothing when nobody is typing', () => {
        const {container} = render([]);
        expect(container.textContent).toBe('');
    });

    test('names one, two or three people typing', () => {
        render(['alice']);
        expect(screen.getByText('alice is typing…')).toBeInTheDocument();
    });

    test('names up to three people', () => {
        render(['alice', 'bob', 'carol']);
        expect(screen.getByText('alice, bob, and carol are typing…')).toBeInTheDocument();
    });

    test('says several people are typing beyond three', () => {
        render(names);
        expect(screen.getByText('Several people are typing…')).toBeInTheDocument();
    });
});
