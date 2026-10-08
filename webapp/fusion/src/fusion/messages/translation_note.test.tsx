// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {fireEvent, screen} from '@testing-library/react';
import React from 'react';

import type {Post} from '@mattermost/types/posts';

import {renderWithContext} from 'tests/react_testing_utils';

import TranslationNote from './translation_note';

const post = (state: string, sourceLang?: string) => ({
    id: 'p1',
    type: '',
    message: 'Bonjour',
    metadata: {translations: {en: {state, source_lang: sourceLang, object: {message: 'Hello'}}}},
}) as unknown as Post;

describe('TranslationNote', () => {
    test('says from what language a message was translated, and switches to the original', () => {
        const onToggle = jest.fn();
        renderWithContext(
            <TranslationNote
                post={post('ready', 'fr')}
                showOriginal={false}
                onToggle={onToggle}
            />,
        );
        expect(screen.getByText('Translated from French')).toBeInTheDocument();
        fireEvent.click(screen.getByText('Show the original'));
        expect(onToggle).toHaveBeenCalled();
    });

    test('offers the translation back while showing the original', () => {
        renderWithContext(
            <TranslationNote
                post={post('ready', 'fr')}
                showOriginal={true}
                onToggle={jest.fn()}
            />,
        );
        expect(screen.getByText('Original message')).toBeInTheDocument();
        expect(screen.getByText('Show the translation')).toBeInTheDocument();
    });

    test('says when a message is being translated, and nothing when it was not', () => {
        const {rerender} = renderWithContext(
            <TranslationNote
                post={post('processing')}
                showOriginal={false}
                onToggle={jest.fn()}
            />,
        );
        expect(screen.getByText('Translating…')).toBeInTheDocument();
        rerender(
            <TranslationNote
                post={post('skipped')}
                showOriginal={false}
                onToggle={jest.fn()}
            />,
        );
        expect(screen.queryByText(/Translat/)).not.toBeInTheDocument();
    });
});
