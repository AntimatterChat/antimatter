// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {createCustomEmoji, deleteCustomEmoji, getCustomEmojiByName, getCustomEmojis, searchCustomEmojis} from 'mattermost-redux/actions/emojis';

import {act, fireEvent, renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import {AddCustomEmojiView, ManageCustomEmojiView, MAX_BYTES, cleanName, imageProblem, nameFromFile, nameProblem} from './custom_emoji_views';

jest.mock('mattermost-redux/actions/emojis', () => ({
    createCustomEmoji: jest.fn(),
    deleteCustomEmoji: jest.fn(() => ({type: 'MOCK_DELETE_EMOJI', data: true})),
    getCustomEmojiByName: jest.fn(() => ({type: 'MOCK_EMOJI_BY_NAME', error: {status_code: 404}})),
    getCustomEmojis: jest.fn(() => ({type: 'MOCK_GET_EMOJIS', data: []})),
    searchCustomEmojis: jest.fn(() => ({type: 'MOCK_SEARCH_EMOJIS', data: []})),
}));

const emoji = (id: string, name: string, creator: string, at = Date.UTC(2026, 9, 12)) => ({id, name, creator_id: creator, create_at: at, update_at: at, delete_at: 0, category: 'custom' as const});

// The person's permissions, by role, as the server grants them: members may add emoji and delete their own; deleting
// others' takes delete_others_emojis too.
const MEMBER = ['create_emojis', 'delete_emojis'];
const ADMIN = ['create_emojis', 'delete_emojis', 'delete_others_emojis'];

function stateWith({perms = MEMBER, enabled = true, emojis = [emoji('e1', 'blobcat', 'nina'), emoji('e2', 'approved', 'me')]} = {}) {
    return {
        entities: {
            general: {config: {EnableCustomEmoji: enabled ? 'true' : 'false'}},
            users: {
                currentUserId: 'me',
                profiles: {
                    me: TestHelper.getUserMock({id: 'me', username: 'me', roles: 'system_user'}),
                    nina: TestHelper.getUserMock({id: 'nina', username: 'nina', first_name: '', last_name: '', nickname: ''}),
                },
            },
            roles: {roles: {system_user: {id: 'r1', name: 'system_user', permissions: perms}}},
            teams: {currentTeamId: 'team1', teams: {team1: TestHelper.getTeamMock({id: 'team1', name: 'lab'})}, myMembers: {}},
            emojis: {customEmoji: Object.fromEntries(emojis.map((e) => [e.id, e]))},
            preferences: {myPreferences: {}},
        },
    };
}

const image = (name = 'Party Parrot.gif', size = 1000, type = 'image/gif') => {
    const file = new File(['x'], name, {type});
    Object.defineProperty(file, 'size', {value: size});
    return file;
};

const fileInput = () => document.querySelector('input[type="file"]') as HTMLInputElement;
const nameInput = () => screen.getByRole('textbox', {name: 'Name'});

describe('custom emoji names and images, as the server checks them', () => {
    const map = {hasSystemEmoji: (n: string) => n === 'smile', has: (n: string) => n === 'smile' || n === 'blobcat'};

    test('names come from file names, lowercase without colons', () => {
        expect(nameFromFile('Party Parrot.gif')).toBe('party_parrot');
        expect(nameFromFile('-wave-.png')).toBe('wave');
        expect(cleanName(' :Thumbs_Up: ')).toBe('thumbs_up');
    });

    test('names take lowercase letters, numbers and - + _, up to 64 characters', () => {
        expect(nameProblem('', map)).toEqual({kind: 'empty'});
        expect(nameProblem('party parrot!', map)).toEqual({kind: 'chars', chars: ' !'});
        expect(nameProblem('a'.repeat(65), map)).toEqual({kind: 'long'});
        expect(nameProblem('a'.repeat(64), map)).toBeNull();
        expect(nameProblem('c++_-1', map)).toBeNull();
    });

    test('names of standard and custom emoji are taken', () => {
        expect(nameProblem('smile', map)).toEqual({kind: 'system'});
        expect(nameProblem('blobcat', map)).toEqual({kind: 'taken'});
    });

    test('images are PNG, GIF, JPEG or WebP of 512 KB at most', () => {
        expect(imageProblem(image('a.png', 10, 'image/png'))).toBeNull();
        expect(imageProblem(image('a.webp', 10, 'image/webp'))).toBeNull();
        expect(imageProblem(image('a.svg', 10, 'image/svg+xml'))).toEqual({kind: 'type'});
        expect(imageProblem(image('a.gif', MAX_BYTES))).toBeNull();
        expect(imageProblem(image('a.gif', MAX_BYTES + 1))).toEqual({kind: 'size'});
    });
});

describe('fusion/popovers/AddCustomEmojiView', () => {
    beforeAll(() => {
        URL.createObjectURL = jest.fn(() => 'blob:preview');
        URL.revokeObjectURL = jest.fn();
    });

    beforeEach(() => {
        jest.mocked(createCustomEmoji).mockReset();
        jest.mocked(getCustomEmojiByName).mockClear();
    });

    const render = () => {
        const onAdded = jest.fn();
        const onBack = jest.fn();
        renderWithContext(
            <AddCustomEmojiView
                onBack={onBack}
                onAdded={onAdded}
            />,
            stateWith() as never,
        );
        return {onAdded, onBack};
    };

    test('names the emoji after its image, checks the name with the server, saves it and hands it back', async () => {
        const created = emoji('e3', 'party_parrot', 'me');
        jest.mocked(createCustomEmoji).mockReturnValue({type: 'MOCK_CREATE_EMOJI', data: created} as never);
        const {onAdded} = render();

        await userEvent.upload(fileInput(), image());
        expect(nameInput()).toHaveValue('party_parrot');

        // The preview shows it at the sizes emoji appear at.
        expect(screen.getByRole('img', {name: 'Preview of :party_parrot: at 16, 22, 32 pixels'})).toBeInTheDocument();

        // The server says the name is free.
        expect(await screen.findByText(':party_parrot: is free.')).toBeInTheDocument();
        expect(getCustomEmojiByName).toHaveBeenCalledWith('party_parrot');

        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        expect(createCustomEmoji).toHaveBeenCalledWith({name: 'party_parrot', creator_id: 'me'}, expect.any(File));
        expect(onAdded).toHaveBeenCalledWith(created);
    });

    test('tells what is wrong with a name as it is typed', async () => {
        render();
        await userEvent.type(nameInput(), 'party parrot!');
        expect(nameInput()).toHaveAccessibleDescription('A name can’t have spaces “!”. Use lowercase letters, numbers, and - + _');
        expect(nameInput()).toHaveAttribute('aria-invalid', 'true');

        await userEvent.clear(nameInput());
        await userEvent.type(nameInput(), ':smile:');
        expect(nameInput()).toHaveAccessibleDescription(':smile: is a standard emoji. Choose another name.');

        await userEvent.clear(nameInput());
        await userEvent.type(nameInput(), 'blobcat');
        expect(nameInput()).toHaveAccessibleDescription('There’s already a custom emoji called :blobcat:.');

        await userEvent.clear(nameInput());
        await userEvent.type(nameInput(), 'a'.repeat(65));
        expect(nameInput()).toHaveAccessibleDescription('A name can have up to 64 characters.');
        expect(screen.getByText('65/64')).toBeInTheDocument();
    });

    test('a name the server already has is taken, even when the picker hasn\'t loaded that emoji', async () => {
        jest.mocked(getCustomEmojiByName).mockReturnValueOnce({type: 'MOCK_EMOJI_BY_NAME', data: emoji('e9', 'elsewhere', 'nina')} as never);
        render();
        await userEvent.type(nameInput(), 'elsewhere');
        await waitFor(() => expect(nameInput()).toHaveAccessibleDescription('There’s already a custom emoji called :elsewhere:.'));
    });

    test('refuses an image too large or of another type, inline', async () => {
        render();
        await userEvent.upload(fileInput(), image('big.gif', MAX_BYTES + 1));
        expect(screen.getByRole('alert')).toHaveTextContent('This image is larger than 512 KB. Choose a smaller one.');

        // The file chooser filters by type, but a drop or a paste doesn't.
        fireEvent.change(fileInput(), {target: {files: [image('vector.svg', 100, 'image/svg+xml')]}});
        expect(screen.getByRole('alert')).toHaveTextContent('Choose a PNG, GIF, JPEG or WebP image.');

        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        expect(createCustomEmoji).not.toHaveBeenCalled();
    });

    test('says what is missing when saving without an image or a name', async () => {
        render();
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        expect(screen.getByText('Choose an image.')).toBeInTheDocument();
        expect(nameInput()).toHaveAccessibleDescription('Give the emoji a name.');
        expect(createCustomEmoji).not.toHaveBeenCalled();
    });

    test('takes an image dropped on it or pasted', async () => {
        render();
        const zone = document.querySelector('.am-pv-drop') as HTMLElement;
        fireEvent.drop(zone, {dataTransfer: {files: [image('Dropped.png', 100, 'image/png')]}});
        expect(nameInput()).toHaveValue('dropped');
        expect(screen.getByText('Dropped.png')).toBeInTheDocument();

        fireEvent.paste(nameInput(), {clipboardData: {files: [image('pasted.gif')]}});
        expect(screen.getByText('pasted.gif')).toBeInTheDocument();

        // Pasting text in the name stays pasting text.
        fireEvent.paste(nameInput(), {clipboardData: {files: []}});
        expect(screen.getByText('pasted.gif')).toBeInTheDocument();
    });

    test('shows the server\'s error inline and stays open', async () => {
        jest.mocked(createCustomEmoji).mockReturnValue({type: 'MOCK_CREATE_EMOJI', error: {message: 'Unable to create emoji. Image exceeds maximum dimensions.'}} as never);
        const {onAdded} = render();
        await userEvent.upload(fileInput(), image());
        await screen.findByText(':party_parrot: is free.');
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        expect(await screen.findByText('Unable to create emoji. Image exceeds maximum dimensions.')).toBeInTheDocument();
        expect(onAdded).not.toHaveBeenCalled();
        expect(screen.getByRole('form', {name: 'Add a custom emoji'})).toBeInTheDocument();
    });

    test('Back and Cancel go back', async () => {
        const {onBack} = render();
        await userEvent.click(screen.getByRole('button', {name: 'Back'}));
        await userEvent.click(screen.getByRole('button', {name: 'Cancel'}));
        expect(onBack).toHaveBeenCalledTimes(2);
    });
});

describe('fusion/popovers/ManageCustomEmojiView', () => {
    beforeEach(() => {
        jest.mocked(deleteCustomEmoji).mockClear();
        jest.mocked(getCustomEmojis).mockClear();
        jest.mocked(searchCustomEmojis).mockClear();
    });

    const render = (state = stateWith(), props: Partial<React.ComponentProps<typeof ManageCustomEmojiView>> = {}) => renderWithContext(
        <ManageCustomEmojiView
            onBack={jest.fn()}
            {...props}
        />,
        state as never,
    );

    test('lists the server\'s custom emoji by name, with who added each and when', () => {
        render();
        const rows = screen.getAllByRole('listitem');
        expect(rows.map((r) => r.querySelector('.am-pv-row-name')?.textContent)).toEqual([':approved:', ':blobcat:']);
        expect(rows[1]).toHaveTextContent('nina · Oct 12, 2026');
        expect(getCustomEmojis).toHaveBeenCalledWith(0, 100, 'name', true);
    });

    test('searches what is loaded and asks the server for the rest', async () => {
        render();
        await userEvent.type(screen.getByRole('searchbox', {name: 'Search custom emoji'}), ':blob');
        expect(screen.getAllByRole('listitem')).toHaveLength(1);
        await waitFor(() => expect(searchCustomEmojis).toHaveBeenCalledWith('blob', {}, true));

        await userEvent.clear(screen.getByRole('searchbox', {name: 'Search custom emoji'}));
        await userEvent.type(screen.getByRole('searchbox', {name: 'Search custom emoji'}), 'zzz');

        // Once the server answered too.
        expect(await screen.findByText('No custom emoji matches “zzz”.')).toBeInTheDocument();
    });

    test('members delete only their own emoji', () => {
        render();
        expect(screen.getByRole('button', {name: 'Delete :approved:'})).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Delete :blobcat:'})).not.toBeInTheDocument();
    });

    test('delete_others_emojis without delete_emojis deletes nothing, as on the server', () => {
        render(stateWith({perms: ['delete_others_emojis']}));
        expect(screen.queryByRole('button', {name: /^Delete/})).not.toBeInTheDocument();
    });

    test('admins delete anyone\'s; guests no one\'s', () => {
        const {unmount} = render(stateWith({perms: ADMIN}));
        expect(screen.getByRole('button', {name: 'Delete :approved:'})).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Delete :blobcat:'})).toBeInTheDocument();
        unmount();

        render(stateWith({perms: []}));
        expect(screen.queryByRole('button', {name: /^Delete/})).not.toBeInTheDocument();
    });

    test('asks in the row before deleting: Keep keeps it, Delete deletes it', async () => {
        render(stateWith({perms: ADMIN}));
        await userEvent.click(screen.getByRole('button', {name: 'Delete :blobcat:'}));
        const confirm = screen.getByRole('group', {name: 'Delete :blobcat:? Messages and reactions using it will show its name instead.'});
        expect(screen.getByRole('button', {name: 'Keep'})).toHaveFocus();

        await userEvent.click(screen.getByRole('button', {name: 'Keep'}));
        expect(confirm).not.toBeInTheDocument();
        expect(deleteCustomEmoji).not.toHaveBeenCalled();

        await userEvent.click(screen.getByRole('button', {name: 'Delete :blobcat:'}));
        await userEvent.click(screen.getByRole('button', {name: 'Delete'}));
        expect(deleteCustomEmoji).toHaveBeenCalledWith('e1');
        expect(await screen.findByText(':blobcat: was deleted.')).toBeInTheDocument();
    });

    test('shows the server\'s refusal to delete inline', async () => {
        jest.mocked(deleteCustomEmoji).mockReturnValueOnce({type: 'MOCK_DELETE_EMOJI', error: {message: 'You do not have the appropriate permissions.'}} as never);
        render(stateWith({perms: ADMIN}));
        await userEvent.click(screen.getByRole('button', {name: 'Delete :blobcat:'}));
        await userEvent.click(screen.getByRole('button', {name: 'Delete'}));
        expect(await screen.findByRole('alert')).toHaveTextContent('You do not have the appropriate permissions.');
    });

    test('loads the next page when there may be more', async () => {
        const page = Array.from({length: 100}, (_, i) => emoji(`p${i}`, `e${String(i).padStart(3, '0')}`, 'nina'));
        jest.mocked(getCustomEmojis).mockReturnValueOnce({type: 'MOCK_GET_EMOJIS', data: page} as never);
        render();
        await act(async () => {});
        await userEvent.click(await screen.findByRole('button', {name: 'Show more'}));
        await waitFor(() => expect(getCustomEmojis).toHaveBeenCalledWith(1, 100, 'name', true));
    });

    test('draws long lists a chunk at a time', async () => {
        const many = Array.from({length: 150}, (_, i) => emoji(`m${i}`, `m${String(i).padStart(3, '0')}`, 'nina'));
        render(stateWith({emojis: many}));
        expect(screen.getAllByRole('listitem')).toHaveLength(60);
        await userEvent.click(screen.getByRole('button', {name: 'Show more'}));
        expect(screen.getAllByRole('listitem')).toHaveLength(120);
    });

    test('a row uses its emoji when the picker asks for it', async () => {
        const onPick = jest.fn();
        render(stateWith(), {onPick});
        await userEvent.click(screen.getByRole('button', {name: /^:blobcat:/}));
        expect(onPick).toHaveBeenCalledWith(expect.objectContaining({id: 'e1', name: 'blobcat'}));
    });

    test('Up and Down move between the rows', async () => {
        render(stateWith(), {onPick: jest.fn()});
        const rows = screen.getAllByRole('button', {name: /^:/});
        await userEvent.click(screen.getByRole('searchbox', {name: 'Search custom emoji'}));
        await userEvent.keyboard('{ArrowDown}');
        expect(rows[0]).toHaveFocus();
        await userEvent.keyboard('{ArrowDown}');
        expect(rows[1]).toHaveFocus();
        await userEvent.keyboard('{ArrowUp}');
        expect(rows[0]).toHaveFocus();
    });
});
