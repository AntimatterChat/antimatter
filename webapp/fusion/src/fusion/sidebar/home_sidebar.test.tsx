// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {RecapStatus} from '@mattermost/types/recaps';

import {LayoutProvider} from 'fusion/shell/layout_context';
import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import HomeSidebar from './home_sidebar';

let mockHideUnreads = false;
jest.mock('fusion/focus/focus', () => ({useHideUnreads: () => mockHideUnreads}));

// Unrelated to the badges under test: stubbed out so this test isn't hostage to their own dependencies.
jest.mock('fusion/calls/calls_on_top', () => ({useCallsOnTop: (channels: unknown) => channels}));
jest.mock('fusion/privacy/message_requests', () => ({RequestsSection: () => null}));
jest.mock('fusion/popovers/collections_popover', () => ({__esModule: true, default: () => null}));
jest.mock('./hidden_section', () => ({__esModule: true, default: () => null}));
jest.mock('./voice_panel', () => ({__esModule: true, default: () => null}));
jest.mock('./group_dm', () => ({useNotesToSelf: () => jest.fn()}));

describe('fusion/sidebar/HomeSidebar', () => {
    const team = TestHelper.getTeamMock({id: 't1', name: 'team'});

    const baseState = {
        entities: {
            users: {currentUserId: 'me'},
            teams: {currentTeamId: 't1', teams: {t1: team}},
            preferences: {myPreferences: {}},
        },
    };

    const render = (extraEntities: Record<string, unknown> = {}) => renderWithContext(
        <LayoutProvider><HomeSidebar/></LayoutProvider>,
        {
            entities: {
                ...baseState.entities,
                ...extraEntities,
            },
        } as never,
    );

    beforeEach(() => {
        mockHideUnreads = false;
    });

    test('shows no badge on Followed threads with nothing unread', () => {
        render();

        const link = screen.getByRole('button', {name: /Followed threads/});
        expect(link).not.toHaveTextContent(/\d/);
    });

    test('shows the number of followed threads with unread replies', () => {
        render({
            threads: {countsIncludingDirect: {t1: {total: 5, total_unread_threads: 3, total_unread_mentions: 0}}},
        });

        const link = screen.getByRole('button', {name: /Followed threads/});
        expect(link).toHaveTextContent('3');
    });

    test('shows the mention count instead, the way the Mentions badge does, when threads have unread mentions', () => {
        render({
            threads: {countsIncludingDirect: {t1: {total: 5, total_unread_threads: 3, total_unread_mentions: 2}}},
        });

        const link = screen.getByRole('button', {name: /Followed threads/});
        expect(link).toHaveTextContent('2');
    });

    test('hides the Followed threads badge while unread badges hide (focus mode)', () => {
        mockHideUnreads = true;
        render({
            threads: {countsIncludingDirect: {t1: {total: 5, total_unread_threads: 3, total_unread_mentions: 2}}},
        });

        const link = screen.getByRole('button', {name: /Followed threads/});
        expect(link).not.toHaveTextContent(/\d/);
    });

    const unreadRecap = {
        id: 'r1',
        user_id: 'me',
        title: 'Daily recap',
        create_at: 1,
        update_at: 1,
        delete_at: 0,
        read_at: 0,
        viewed_at: 0,
        total_message_count: 3,
        status: RecapStatus.COMPLETED,
        bot_id: 'bot',
    };

    test('shows the number of unread recaps', () => {
        render({
            general: {config: {FeatureFlagEnableAIRecaps: 'true'}},
            recaps: {byId: {r1: unreadRecap}, allIds: ['r1']},
        });

        const link = screen.getByRole('link', {name: /Recaps/});
        expect(link).toHaveTextContent('1');
    });

    test('hides the Recaps badge while unread badges hide (focus mode)', () => {
        mockHideUnreads = true;
        render({
            general: {config: {FeatureFlagEnableAIRecaps: 'true'}},
            recaps: {byId: {r1: unreadRecap}, allIds: ['r1']},
        });

        const link = screen.getByRole('link', {name: /Recaps/});
        expect(link).not.toHaveTextContent(/\d/);
    });
});
