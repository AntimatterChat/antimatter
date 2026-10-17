// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import {RowContent} from './autocomplete';

jest.mock('fusion/components/avatar', () => () => null);

// Agents come in the @ suggestions with the people (the server lists the agents the person may talk to): tagged AGENT,
// where a bot says "bot".
describe('fusion/composer/autocomplete: agents', () => {
    test('an agent from the agents\' list is tagged AGENT', () => {
        const coder = {...TestHelper.getUserMock({id: 'coder', username: 'coder', first_name: 'Coder', is_bot: true}), isAgent: true};
        renderWithContext(
            <RowContent
                kind='mention'
                row={{term: '@coder', item: coder}}
            />,
        );
        expect(screen.getByText('AGENT')).toBeInTheDocument();
        expect(screen.queryByText(/· bot/)).not.toBeInTheDocument();
    });

    test('a user marked is_agent is tagged AGENT too', () => {
        const researcher = TestHelper.getUserMock({id: 'r', username: 'researcher', first_name: 'Researcher', is_bot: true, is_agent: true});
        renderWithContext(
            <RowContent
                kind='mention'
                row={{term: '@researcher', item: researcher}}
            />,
        );
        expect(screen.getByText('AGENT')).toBeInTheDocument();
    });

    test('a plain bot says bot, without the tag', () => {
        const bot = TestHelper.getUserMock({id: 'b', username: 'release', first_name: 'Release', is_bot: true});
        renderWithContext(
            <RowContent
                kind='mention'
                row={{term: '@release', item: bot}}
            />,
        );
        expect(screen.queryByText('AGENT')).not.toBeInTheDocument();
        expect(screen.getByText(/· bot/)).toBeInTheDocument();
    });
});
