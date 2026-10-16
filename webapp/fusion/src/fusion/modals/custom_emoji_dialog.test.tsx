// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {createCustomEmoji, deleteCustomEmoji, searchCustomEmojis} from 'mattermost-redux/actions/emojis';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import CustomEmojiDialog, {nameFromFile} from './custom_emoji_dialog';

let mockPerms = new Set<string>();
jest.mock('mattermost-redux/selectors/entities/roles', () => ({
    ...jest.requireActual('mattermost-redux/selectors/entities/roles'),
    haveICurrentTeamPermission: jest.fn((state: unknown, permission: string) => mockPerms.has(permission)),
}));
jest.mock('mattermost-redux/actions/emojis', () => ({
    createCustomEmoji: jest.fn(() => ({type: 'MOCK_CREATE_EMOJI'})),
    deleteCustomEmoji: jest.fn(() => ({type: 'MOCK_DELETE_EMOJI'})),
    getCustomEmojis: jest.fn(() => ({type: 'MOCK_GET_EMOJIS'})),
    searchCustomEmojis: jest.fn(() => ({type: 'MOCK_SEARCH_EMOJIS'})),
}));
const mockEmojiMap = {hasSystemEmoji: (name: string) => name === 'smile', has: (name: string) => name === 'blobcat' || name === 'smile'};
jest.mock('selectors/emojis', () => ({getEmojiMap: () => mockEmojiMap}));

const emoji = (id: string, name: string, creator: string) => ({id, name, creator_id: creator, create_at: 1, update_at: 1, delete_at: 0, category: 'custom'});
const state = {
    entities: {
        general: {config: {EnableCustomEmoji: 'true'}},
        users: {
            currentUserId: 'me',
            profiles: {
                me: TestHelper.getUserMock({id: 'me', username: 'me'}),
                nina: TestHelper.getUserMock({id: 'nina', username: 'nina', first_name: '', last_name: '', nickname: ''}),
            },
        },
        emojis: {customEmoji: {e1: emoji('e1', 'blobcat', 'nina'), e2: emoji('e2', 'approved', 'me')}},
        preferences: {myPreferences: {}},
    },
};

const image = (name = 'Party Parrot.gif', size = 1000) => {
    const file = new File(['x'], name, {type: 'image/gif'});
    Object.defineProperty(file, 'size', {value: size});
    return file;
};

describe('fusion/modals/CustomEmojiDialog', () => {
    beforeAll(() => {
        URL.createObjectURL = jest.fn(() => 'blob:preview');
        URL.revokeObjectURL = jest.fn();
    });

    beforeEach(() => {
        mockPerms = new Set();
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const render = (tab: 'add' | 'all' = 'add') => {
        const onExited = jest.fn();
        renderWithContext(
            <CustomEmojiDialog
                tab={tab}
                onExited={onExited}
            />,
            state as never,
        );
        return onExited;
    };
    const choose = (file: File) => userEvent.upload(document.querySelector('input[type="file"]') as HTMLInputElement, file);
    const nameInput = () => screen.getByPlaceholderText('e.g. party_parrot');

    test('names an emoji after its image, adds it and is ready for the next', async () => {
        render();
        const add = screen.getByRole('button', {name: 'Add emoji'});
        expect(add).toBeDisabled();

        await choose(image());
        expect(nameInput()).toHaveValue('party_parrot');
        expect(screen.getByText('Party Parrot.gif')).toBeInTheDocument();
        await userEvent.click(add);

        expect(createCustomEmoji).toHaveBeenCalledWith({name: 'party_parrot', creator_id: 'me'}, expect.any(File));
        expect(await screen.findByRole('status')).toHaveTextContent(':party_parrot: is ready to use.');
        expect(nameInput()).toHaveValue('');
        expect(screen.getByRole('button', {name: 'Done'})).toBeInTheDocument();
    });

    test('takes a name typed between colons', async () => {
        render();
        await choose(image());
        await userEvent.clear(nameInput());
        await userEvent.type(nameInput(), ':Blob_Dance:');
        await userEvent.click(screen.getByRole('button', {name: 'Add emoji'}));
        expect(createCustomEmoji).toHaveBeenCalledWith({name: 'blob_dance', creator_id: 'me'}, expect.any(File));
    });

    test.each([
        ['party parrot', 'A name can only have lowercase letters, numbers, and - + _'],
        ['smile', 'An emoji already has this name.'],
        ['blobcat', 'A custom emoji already has this name.'],
    ])('refuses the name "%s"', async (name, why) => {
        render();
        await choose(image());
        await userEvent.clear(nameInput());
        await userEvent.type(nameInput(), name);
        await userEvent.click(screen.getByRole('button', {name: 'Add emoji'}));
        expect(screen.getByRole('alert')).toHaveTextContent(why);
        expect(createCustomEmoji).not.toHaveBeenCalled();
    });

    test('refuses an image over 1 MB', async () => {
        render();
        await choose(image('big.png', (1 << 20) + 1));
        await userEvent.click(screen.getByRole('button', {name: 'Add emoji'}));
        expect(screen.getByRole('alert')).toHaveTextContent('The image must be 1 MB at most.');
        expect(createCustomEmoji).not.toHaveBeenCalled();
    });

    test('tells why the server refused it', async () => {
        jest.mocked(createCustomEmoji).mockReturnValueOnce((() => ({error: {message: 'Emoji limit reached'}})) as never);
        render();
        await choose(image());
        await userEvent.click(screen.getByRole('button', {name: 'Add emoji'}));
        expect(await screen.findByRole('alert')).toHaveTextContent('Emoji limit reached');
    });

    test('lists the server\'s custom emoji by name, with who added them', async () => {
        render('all');
        const rows = screen.getAllByRole('listitem');
        expect(rows.map((r) => r.textContent)).toEqual([':approved:me', ':blobcat:nina']);
    });

    test('without permissions, nobody deletes emoji', () => {
        render('all');
        expect(screen.queryByRole('button', {name: /^Delete :/})).not.toBeInTheDocument();
    });

    test('deleting one\'s own emoji takes its permission, and others\' theirs', () => {
        mockPerms = new Set(['delete_emojis']);
        render('all');
        expect(screen.getByRole('button', {name: 'Delete :approved:'})).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Delete :blobcat:'})).not.toBeInTheDocument();
    });

    test('someone allowed to delete others\' emoji deletes them, after confirming', async () => {
        mockPerms = new Set(['delete_others_emojis']);
        render('all');
        expect(screen.queryByRole('button', {name: 'Delete :approved:'})).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('button', {name: 'Delete :blobcat:'}));
        await userEvent.click(screen.getByRole('button', {name: 'Keep'}));
        expect(deleteCustomEmoji).not.toHaveBeenCalled();

        await userEvent.click(screen.getByRole('button', {name: 'Delete :blobcat:'}));
        await userEvent.click(screen.getByRole('button', {name: 'Delete'}));
        expect(deleteCustomEmoji).toHaveBeenCalledWith('e1');
    });

    test('searches, also on the server', async () => {
        render('all');
        await userEvent.type(screen.getByRole('textbox', {name: 'Search custom emoji'}), ':blob');
        expect(screen.getAllByRole('listitem').map((r) => r.textContent)).toEqual([':blobcat:nina']);
        await waitFor(() => expect(searchCustomEmojis).toHaveBeenCalledWith('blob', {}, true));

        await userEvent.clear(screen.getByRole('textbox', {name: 'Search custom emoji'}));
        await userEvent.type(screen.getByRole('textbox', {name: 'Search custom emoji'}), 'zzz');
        expect(screen.getByText('No custom emoji matches.')).toBeInTheDocument();
    });

    test('switches between adding and the list', async () => {
        render();
        expect(screen.getByRole('tab', {name: 'Add'})).toHaveAttribute('aria-selected', 'true');
        await userEvent.click(screen.getByRole('tab', {name: 'All custom emoji'}));
        expect(screen.getByRole('textbox', {name: 'Search custom emoji'})).toBeInTheDocument();
    });
});

describe('nameFromFile', () => {
    test('makes an emoji name of a file name', () => {
        expect(nameFromFile('Party Parrot.gif')).toBe('party_parrot');
        expect(nameFromFile('__thumbs-up+1!!.png')).toBe('thumbs-up+1');
        expect(nameFromFile('a'.repeat(80) + '.png')).toHaveLength(64);
    });
});
