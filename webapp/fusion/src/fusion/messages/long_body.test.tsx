// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import LongBody, {LONG_MESSAGE_HEIGHT} from './long_body';

describe('fusion/messages/LongBody', () => {
    const setHeight = (height: number) => Object.defineProperty(HTMLElement.prototype, 'scrollHeight', {configurable: true, get: () => height});

    afterEach(() => {
        delete (HTMLElement.prototype as {scrollHeight?: number}).scrollHeight;
    });

    test('leaves short messages alone', () => {
        setHeight(120);
        renderWithContext(
            <LongBody
                postId='short'
                className='am-body'
            >
                <p>{'Short'}</p>
            </LongBody>,
        );

        expect(screen.queryByRole('button')).not.toBeInTheDocument();
        expect(screen.getByText('Short').parentElement).not.toHaveClass('am-folded');
    });

    test('folds long messages, and shows them in full on demand', async () => {
        setHeight(LONG_MESSAGE_HEIGHT * 3);
        renderWithContext(
            <LongBody
                postId='long'
                className='am-body'
            >
                <p>{'Long'}</p>
            </LongBody>,
        );

        const body = screen.getByText('Long').parentElement!;
        expect(body).toHaveClass('am-folded');
        expect(body).toHaveStyle({maxHeight: `${LONG_MESSAGE_HEIGHT}px`});

        await userEvent.click(screen.getByRole('button', {name: 'Show more'}));
        expect(body).not.toHaveClass('am-folded');

        await userEvent.click(screen.getByRole('button', {name: 'Show less'}));
        expect(body).toHaveClass('am-folded');
    });
});
