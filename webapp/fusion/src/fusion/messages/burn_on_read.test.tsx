// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import {BurnCover, BurnTag} from './burn_on_read';

jest.mock('components/post_view/burn_on_read_expiration_handler', () => () => null);
jest.mock('hooks/useBurnOnReadDeleteModal', () => ({useBurnOnReadDeleteModal: () => ({})}));

// A sent burn-on-read message says how long it lives once opened: its sender's choice, else the server's duration.
describe('fusion/messages/burn_on_read', () => {
    const state = {
        entities: {
            general: {config: {BurnOnReadDurationSeconds: '600', BurnOnReadMaximumDurationSeconds: '86400'}},
            users: {currentUserId: 'reader'},
            posts: {posts: {}},
            channels: {channels: {}, myMembers: {}},
            preferences: {myPreferences: {}},
        },
    } as never;
    const post = (props: Record<string, unknown> = {}) => TestHelper.getPostMock({id: 'p1', user_id: 'sender', channel_id: 'c1', type: 'burn_on_read', props: {expire_at: Date.now() + 3600000, ...props}});

    test('the tag says the duration its sender picked', () => {
        renderWithContext(<BurnTag post={post({read_duration: 15 * 60 * 1000})}/>, state);
        expect(screen.getByText('Burn on read · 15 min')).toBeInTheDocument();
    });

    test('without one, the server\'s duration', () => {
        renderWithContext(<BurnTag post={post()}/>, state);
        expect(screen.getByText('Burn on read · 10 min')).toBeInTheDocument();
    });

    test('a day reads as a day', () => {
        renderWithContext(<BurnTag post={post({read_duration: 24 * 60 * 60 * 1000})}/>, state);
        expect(screen.getByText('Burn on read · 1 day')).toBeInTheDocument();
    });

    test('the cover tells the reader before they open it', () => {
        renderWithContext(<BurnCover post={post({read_duration: 4 * 60 * 60 * 1000})}/>, state);
        expect(screen.getByRole('button')).toHaveTextContent('Click to read — this message burns 4 hours after you open it');
    });
});
