// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {DeepPartial} from '@mattermost/types/utilities';

import {renderWithContext, screen} from 'tests/react_testing_utils';

import type {GlobalState} from 'types/store';

import ExternalLink from '.';

describe('components/external_link', () => {
    const initialState: DeepPartial<GlobalState> = {
        entities: {
            general: {
                config: {},
            },
            users: {
                currentUserId: 'currentUserId',
            },
        },
    };

    it('should render external link with correct attributes', () => {
        renderWithContext(
            <ExternalLink
                location='test'
                href='https://antimatter.example'
            >
                {'Click Me'}
            </ExternalLink>,
            initialState,
        );

        const linkElement = screen.getByRole('link', {name: 'Click Me'});

        expect(linkElement).toBeInTheDocument();
        expect(linkElement).toHaveAttribute('target', '_blank');
        expect(linkElement).toHaveAttribute('rel', 'noopener noreferrer');
        expect(linkElement).toHaveAttribute('href', expect.stringContaining('https://antimatter.example'));
    });

    it("should not attach parameters if href isn't *.mattermost.com enabled", () => {
        const state = {
            ...initialState,
            entities: {
                ...initialState.entities,
                general: {
                    ...initialState?.entities?.general,
                    config: {
                        DiagnosticsEnabled: 'true',
                    },
                },
            },
        };
        renderWithContext(
            <ExternalLink
                location='test'
                href='https://google.com'
            >
                {'Click Me'}
            </ExternalLink>,
            state,
        );

        expect(screen.queryByText('Click Me')).not.toHaveAttribute(
            'href',
            'utm_source=mattermost&utm_medium=in-product&utm_content=&uid=currentUserId&sid=',
        );
    });

    it('should be able to override target, rel', () => {
        const state = {
            ...initialState,
            entities: {
                ...initialState.entities,
                general: {
                    ...initialState?.entities?.general,
                    config: {
                        DiagnosticsEnabled: 'true',
                    },
                },
            },
        };
        renderWithContext(
            <ExternalLink
                target='test'
                rel='test'
                href='https://google.com'
                location='test'
            >{'Click Me'}</ExternalLink>,
            state,
        );

        expect(screen.queryByText('Click Me')).toHaveAttribute(
            'target',
            expect.stringMatching(
                'test',
            ),
        );
        expect(screen.queryByText('Click Me')).toHaveAttribute(
            'rel',
            expect.stringMatching('test'),
        );
    });


    it('shows links to mattermost.com as text', () => {
        renderWithContext(
            <ExternalLink
                location='test'
                href='https://docs.mattermost.com/guide'
            >
                {'Read the guide'}
            </ExternalLink>,
            initialState,
        );

        expect(screen.getByText('Read the guide')).toBeInTheDocument();
        expect(screen.queryByRole('link')).toBeNull();
    });
});
