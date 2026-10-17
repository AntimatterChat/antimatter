// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, within} from 'tests/react_testing_utils';

import CommandArgsBar, {ArgChips} from './command_args_bar';

const commands = [{trigger: 'invite', hint: '@[username]... #[channel]...', description: 'Invite people'}];

describe('fusion/composer/CommandArgsBar', () => {
    test('shows the command and each argument as a block, the one being typed marked', () => {
        renderWithContext(
            <CommandArgsBar
                pretext='/invite @bob #be'
                commands={commands}
            />,
        );
        const bar = screen.getByRole('group', {name: 'Arguments of /invite'});
        expect(within(bar).getByText('/invite')).toBeInTheDocument();
        expect(within(bar).getByText('@username…').parentElement).toHaveTextContent('@username…@bob');
        const current = bar.querySelector('[aria-current="true"]');
        expect(current).toHaveTextContent('#channel…#be');
    });

    test('shows nothing outside a command\'s arguments', () => {
        const {container} = renderWithContext(
            <CommandArgsBar
                pretext='/invite'
                commands={commands}
            />,
        );
        expect(container).toBeEmptyDOMElement();
    });

    test('a suggestion\'s hint is drawn as blocks', () => {
        const {container} = renderWithContext(<ArgChips hint='@[username]... ~[channel]...'/>);
        expect([...container.querySelectorAll('.am-arg')].map((c) => c.textContent)).toEqual(['@username…', '#channel…']);
    });
});
