// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import FusionErrorBoundary, {PageError} from './error_boundary';

function Boom({fail}: {fail: boolean}): JSX.Element {
    if (fail) {
        throw new Error('boom');
    }
    return <span>{'drawn'}</span>;
}

// An error while drawing one part of the Fusion UI stays in that part instead of blanking the whole window.
describe('fusion/components/FusionErrorBoundary', () => {
    let consoleError: jest.SpyInstance;
    beforeEach(() => {
        // React reports the caught error on the console: expected here.
        consoleError = jest.spyOn(console, 'error').mockImplementation(() => {});
    });
    afterEach(() => {
        consoleError.mockRestore();
    });

    test('shows its fallback in place of a child that throws, and its siblings stay', () => {
        renderWithContext(
            <>
                <FusionErrorBoundary fallback={<span>{'fallback'}</span>}>
                    <Boom fail={true}/>
                </FusionErrorBoundary>
                <span>{'sibling'}</span>
            </>,
        );
        expect(screen.getByText('fallback')).toBeInTheDocument();
        expect(screen.getByText('sibling')).toBeInTheDocument();
        expect(screen.queryByText('drawn')).not.toBeInTheDocument();
    });

    test('draws again once its reset key changes', () => {
        const {rerender} = renderWithContext(
            <FusionErrorBoundary
                fallback={<span>{'fallback'}</span>}
                resetKey='v1'
            >
                <Boom fail={true}/>
            </FusionErrorBoundary>,
        );
        expect(screen.getByText('fallback')).toBeInTheDocument();

        // Same key: still the fallback, even with a child that would now draw.
        rerender(
            <FusionErrorBoundary
                fallback={<span>{'fallback'}</span>}
                resetKey='v1'
            >
                <Boom fail={false}/>
            </FusionErrorBoundary>,
        );
        expect(screen.getByText('fallback')).toBeInTheDocument();

        rerender(
            <FusionErrorBoundary
                fallback={<span>{'fallback'}</span>}
                resetKey='v2'
            >
                <Boom fail={false}/>
            </FusionErrorBoundary>,
        );
        expect(screen.getByText('drawn')).toBeInTheDocument();
    });

    test('the page fallback offers to reload', async () => {
        const reload = jest.fn();
        const location = window.location;
        Object.defineProperty(window, 'location', {configurable: true, value: {...location, reload}});
        try {
            renderWithContext(<PageError/>);
            expect(screen.getByRole('alert')).toHaveTextContent('Something went wrong.');
            await userEvent.click(screen.getByRole('button', {name: 'Reload'}));
            expect(reload).toHaveBeenCalled();
        } finally {
            Object.defineProperty(window, 'location', {configurable: true, value: location});
        }
    });
});
