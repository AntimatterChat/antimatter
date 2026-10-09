// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {savePreferences} from 'mattermost-redux/actions/preferences';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import UserNote, {CATEGORY_USER_NOTE} from './user_note';

jest.mock('mattermost-redux/actions/preferences', () => ({
    ...jest.requireActual('mattermost-redux/actions/preferences'),
    savePreferences: jest.fn(() => ({type: 'SAVED'})),
    deletePreferences: jest.fn(() => ({type: 'DELETED'})),
}));

const save = savePreferences as jest.Mock;

describe('UserNote', () => {
    const render = () => renderWithContext(<UserNote userId='bob'/>, {entities: {users: {currentUserId: 'me'}, preferences: {myPreferences: {}}}} as never);

    beforeEach(() => save.mockClear());

    test('Enter saves the note', async () => {
        render();
        await userEvent.type(screen.getByRole('textbox', {name: 'Note'}), 'met at FOSDEM{enter}');
        expect(save).toHaveBeenCalledWith('me', [{user_id: 'me', category: CATEGORY_USER_NOTE, name: 'bob', value: 'met at FOSDEM'}]);
    });

    test('closing the card keeps what was typed', async () => {
        const {unmount} = render();
        await userEvent.type(screen.getByRole('textbox', {name: 'Note'}), 'likes tea');
        expect(save).not.toHaveBeenCalled();
        unmount();
        expect(save).toHaveBeenCalledWith('me', [{user_id: 'me', category: CATEGORY_USER_NOTE, name: 'bob', value: 'likes tea'}]);
    });

    test('nothing typed, nothing saved', () => {
        const {unmount} = render();
        unmount();
        expect(save).not.toHaveBeenCalled();
    });
});
