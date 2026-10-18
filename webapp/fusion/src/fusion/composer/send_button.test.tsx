// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import SendButton from './send_button';

describe('fusion/composer/SendButton', () => {
    test('schedules from the chevron next to it ("Send later")', async () => {
        const onLater = jest.fn();
        renderWithContext(
            <form onSubmit={(e) => e.preventDefault()}>
                <SendButton
                    disabled={false}
                    onLater={onLater}
                />
            </form>,
        );
        expect(screen.getByRole('button', {name: 'Send'})).toBeInTheDocument();
        const later = screen.getByRole('button', {name: 'Send later'});
        expect(later).toHaveAttribute('type', 'button');
        expect(later).toHaveAttribute('aria-haspopup', 'menu');

        await userEvent.click(later);
        expect(onLater).toHaveBeenCalledTimes(1);
    });

    test('has no chevron when scheduled messages are off', () => {
        renderWithContext(<SendButton disabled={false}/>);
        expect(screen.getByRole('button', {name: 'Send'})).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Send later'})).not.toBeInTheDocument();
    });

    test('is disabled while files upload', () => {
        renderWithContext(
            <SendButton
                disabled={true}
                onLater={jest.fn()}
            />,
        );
        expect(screen.getByRole('button', {name: 'Send'})).toBeDisabled();
        expect(screen.getByRole('button', {name: 'Send later'})).toBeDisabled();
    });

    test('says what a composer mode sends', () => {
        renderWithContext(
            <SendButton
                disabled={false}
                label='Post poll'
            />,
        );
        expect(screen.getByRole('button', {name: 'Post poll'})).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Send'})).not.toBeInTheDocument();
    });
});
