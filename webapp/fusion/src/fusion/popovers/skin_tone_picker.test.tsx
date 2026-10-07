// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {setUserSkinTone} from 'actions/emoji_actions';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import SkinTonePicker from './skin_tone_picker';

jest.mock('actions/emoji_actions', () => ({
    setUserSkinTone: jest.fn(() => ({type: 'SKIN'})),
}));
jest.mock('components/emoji/render_emoji', () => ({
    __esModule: true,
    default: ({emojiName}: {emojiName: string}) => <span>{emojiName}</span>,
}));

describe('fusion/popovers/SkinTonePicker', () => {
    const render = (tone?: string) => renderWithContext(
        <SkinTonePicker/>,
        {entities: {users: {currentUserId: 'me'}, preferences: {myPreferences: tone ? {'emoji--emoji_skintone': {category: 'emoji', name: 'emoji_skintone', user_id: 'me', value: tone}} : {}}}} as never,
    );

    test('shows your skin tone, and saves the one you pick', async () => {
        render('1F3FD');

        await userEvent.click(screen.getByRole('button', {name: 'Skin tone: Medium skin tone'}));
        expect(screen.getByRole('radio', {name: 'Medium skin tone'})).toHaveAttribute('aria-checked', 'true');

        await userEvent.click(screen.getByRole('radio', {name: 'Dark skin tone'}));
        expect(setUserSkinTone).toHaveBeenCalledWith('1F3FF');
        expect(screen.getByRole('button', {name: /^Skin tone/})).toBeInTheDocument();
    });

    test('starts from the default tone', () => {
        render();

        expect(screen.getByRole('button', {name: 'Skin tone: Default skin tone'})).toBeInTheDocument();
    });
});
