// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {DeepPartial} from '@mattermost/types/utilities';

import {EmojiTypes} from 'mattermost-redux/action_types';
import {createCustomEmoji} from 'mattermost-redux/actions/emojis';

import {LAYER_ID} from 'fusion/components/layer';
import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';
import type {EmojiPickerTabProps, EmojiPickerTabRegistration} from 'types/store/plugins';

import EmojiPicker from './emoji_picker';
import type {PickerKit} from './picker_views';

jest.mock('mattermost-redux/actions/emojis', () => ({
    ...jest.requireActual('mattermost-redux/actions/emojis'),
    createCustomEmoji: jest.fn(),
    getCustomEmojiByName: jest.fn(() => ({type: 'MOCK_EMOJI_BY_NAME', error: {status_code: 404}})),
    getCustomEmojis: jest.fn(() => ({type: 'MOCK_GET_EMOJIS', data: []})),
}));

function makeTab(partial: Partial<EmojiPickerTabRegistration> = {}): EmojiPickerTabRegistration {
    return {
        id: 'gifs',
        pluginId: 'gifs-plugin',
        label: 'GIFs',
        order: 0,
        component: ({channelId, rootId, isFusion, onSelectDone, insertText}: EmojiPickerTabProps) => (
            <button
                onClick={() => {
                    insertText?.('gif');
                    onSelectDone();
                }}
            >
                {`gifs of ${channelId}/${rootId}${isFusion ? ' in Fusion' : ''}`}
            </button>
        ),
        shouldRender: () => true,
        replacesGifPicker: true,
        ...partial,
    };
}

function stateWith(tabs: EmojiPickerTabRegistration[]): DeepPartial<GlobalState> {
    return {
        plugins: {
            components: {
                EmojiPickerTab: tabs,
            },
        },
    };
}

const tabs = [makeTab(), makeTab({id: 'stickers', label: 'Stickers', order: 1, replacesGifPicker: false})];

describe('fusion/popovers/EmojiPicker', () => {
    beforeAll(() => {
        // The emoji grids draw once they scroll into view.
        window.IntersectionObserver = class {
            observe() {}
            disconnect() {}
        } as unknown as typeof IntersectionObserver;
    });

    beforeEach(() => {
        const layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.appendChild(layer);
    });

    afterEach(() => {
        document.getElementById(LAYER_ID)?.remove();
    });

    test('has no tabs for reactions', () => {
        renderWithContext(
            <EmojiPicker
                anchor={null}
                onPick={jest.fn()}
                onClose={jest.fn()}
            />,
            stateWith(tabs),
        );

        expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
        expect(screen.getByRole('textbox', {name: 'Search emoji'})).toBeInTheDocument();
    });

    test('has no tabs in the composer without plugin tabs', () => {
        renderWithContext(
            <EmojiPicker
                anchor={null}
                onPick={jest.fn()}
                onClose={jest.fn()}
                compose={{channelId: 'channel', insertText: jest.fn()}}
            />,
            stateWith([]),
        );

        expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
    });

    test('shows the plugin tabs before Emoji in the composer and opens on the last tab used', async () => {
        const onClose = jest.fn();
        const insertText = jest.fn();
        const picker = (
            <EmojiPicker
                anchor={null}
                onPick={jest.fn()}
                onClose={onClose}
                keepOpen={true}
                compose={{channelId: 'channel', rootId: 'root', insertText}}
            />
        );
        const {unmount} = renderWithContext(picker, stateWith(tabs));

        expect(screen.getAllByRole('tab').map((t) => t.textContent)).toEqual(['GIFs', 'Stickers', 'Emoji']);
        expect(screen.getByRole('tab', {name: 'Emoji'})).toHaveAttribute('aria-selected', 'true');

        await userEvent.click(screen.getByRole('tab', {name: 'GIFs'}));
        expect(screen.getByRole('tab', {name: 'GIFs'})).toHaveAttribute('aria-selected', 'true');
        expect(screen.queryByRole('textbox', {name: 'Search emoji'})).not.toBeInTheDocument();

        await userEvent.click(screen.getByRole('button', {name: 'gifs of channel/root in Fusion'}));
        expect(insertText).toHaveBeenCalledWith('gif');
        expect(onClose).toHaveBeenCalled();

        unmount();
        renderWithContext(picker, stateWith(tabs));
        expect(screen.getByRole('tab', {name: 'GIFs'})).toHaveAttribute('aria-selected', 'true');
    });

    test('puts the plugin tabs of order 10 or more after Emoji', () => {
        renderWithContext(
            <EmojiPicker
                anchor={null}
                onPick={jest.fn()}
                onClose={jest.fn()}
                compose={{channelId: 'channel', insertText: jest.fn()}}
            />,
            stateWith([makeTab({id: 'memes', label: 'Memes', order: 20}), ...tabs]),
        );

        expect(screen.getAllByRole('tab').map((t) => t.textContent)).toEqual(['GIFs', 'Stickers', 'Emoji', 'Memes']);
    });

    test('falls back to the Emoji tab when the last tab is gone', () => {
        renderWithContext(
            <EmojiPicker
                anchor={null}
                onPick={jest.fn()}
                onClose={jest.fn()}
                compose={{channelId: 'channel', insertText: jest.fn()}}
            />,
            stateWith([tabs[1]]),
        );

        expect(screen.getAllByRole('tab').map((t) => t.textContent)).toEqual(['Stickers', 'Emoji']);
        expect(screen.getByRole('tab', {name: 'Emoji'})).toHaveAttribute('aria-selected', 'true');
        expect(screen.getByRole('textbox', {name: 'Search emoji'})).toBeInTheDocument();
    });

    // Custom emoji: "+" adds one and "Custom emoji" manages them, in views of the picker, for those allowed to.
    const MEMBER = ['create_emojis', 'delete_emojis'];
    const ADMIN = ['create_emojis', 'delete_emojis', 'delete_others_emojis'];
    const emojiState = ({perms = MEMBER, enabled = true, systemRole = 'system_user'} = {}) => ({
        entities: {
            general: {config: {EnableCustomEmoji: enabled ? 'true' : 'false'}},
            users: {currentUserId: 'me', profiles: {me: TestHelper.getUserMock({id: 'me', roles: systemRole})}},
            roles: {roles: {[systemRole]: {id: 'r1', name: systemRole, permissions: perms}}},
            teams: {currentTeamId: 'team1', teams: {team1: TestHelper.getTeamMock({id: 'team1', name: 'lab'})}, myMembers: {}},
        },
    }) as DeepPartial<GlobalState>;
    const renderPicker = (state: DeepPartial<GlobalState>, props: Partial<React.ComponentProps<typeof EmojiPicker>> = {}) => {
        const onClose = jest.fn();
        const onPick = jest.fn();
        renderWithContext(
            <EmojiPicker
                anchor={null}
                onPick={onPick}
                onClose={onClose}
                {...props}
            />,
            state,
        );
        return {onClose, onPick};
    };

    test('members have "+" and "Custom emoji" next to the search', () => {
        renderPicker(emojiState());
        expect(screen.getByRole('button', {name: 'Add custom emoji'})).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Custom emoji'})).toBeInTheDocument();
    });

    test('guests have neither, nor anyone when custom emoji are off', () => {
        const {unmount} = renderWithContext(
            <EmojiPicker
                anchor={null}
                onPick={jest.fn()}
                onClose={jest.fn()}
            />,
            emojiState({perms: [], systemRole: 'system_guest'}),
        );
        expect(screen.queryByRole('button', {name: 'Add custom emoji'})).not.toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Custom emoji'})).not.toBeInTheDocument();
        unmount();

        renderPicker(emojiState({perms: ADMIN, enabled: false, systemRole: 'system_admin'}));
        expect(screen.queryByRole('button', {name: 'Add custom emoji'})).not.toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Custom emoji'})).not.toBeInTheDocument();
    });

    test('someone who may delete emoji but not add any has only "Custom emoji"', () => {
        renderPicker(emojiState({perms: ['delete_emojis']}));
        expect(screen.queryByRole('button', {name: 'Add custom emoji'})).not.toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Custom emoji'})).toBeInTheDocument();
    });

    test('"+" shows the add view in the picker, which stays open; Back returns to the grid', async () => {
        const {onClose} = renderPicker(emojiState());
        await userEvent.click(screen.getByRole('button', {name: 'Add custom emoji'}));

        const picker = screen.getByRole('dialog', {name: 'Emoji'});
        expect(picker).toContainElement(screen.getByRole('form', {name: 'Add a custom emoji'}));
        expect(screen.queryByRole('textbox', {name: 'Search emoji'})).not.toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Back'})).toHaveFocus();

        await userEvent.click(screen.getByRole('button', {name: 'Back'}));
        expect(screen.getByRole('textbox', {name: 'Search emoji'})).toBeInTheDocument();
        await waitFor(() => expect(screen.getByRole('button', {name: 'Add custom emoji'})).toHaveFocus());
        expect(onClose).not.toHaveBeenCalled();
    });

    test('Escape goes back from a view to the grid, and closes the picker from the grid', async () => {
        const {onClose} = renderPicker(emojiState());
        await userEvent.click(screen.getByRole('button', {name: 'Custom emoji'}));
        expect(screen.getByRole('region', {name: 'Custom emoji'})).toBeInTheDocument();

        await userEvent.keyboard('{Escape}');
        expect(screen.queryByRole('region', {name: 'Custom emoji'})).not.toBeInTheDocument();
        expect(screen.getByRole('textbox', {name: 'Search emoji'})).toBeInTheDocument();
        expect(onClose).not.toHaveBeenCalled();

        await userEvent.keyboard('{Escape}');
        expect(onClose).toHaveBeenCalled();
    });

    test('a saved emoji comes back first on the grid, focused and ready to use', async () => {
        URL.createObjectURL = jest.fn(() => 'blob:preview');
        URL.revokeObjectURL = jest.fn();
        const created = {id: 'e3', name: 'party_parrot', creator_id: 'me', create_at: 1, update_at: 1, delete_at: 0, category: 'custom'};
        jest.mocked(createCustomEmoji).mockReturnValue({type: EmojiTypes.RECEIVED_CUSTOM_EMOJI, data: created} as never);
        const {onClose, onPick} = renderPicker(emojiState(), {keepOpen: true, compose: {channelId: 'channel', insertText: jest.fn()}});

        await userEvent.click(screen.getByRole('button', {name: 'Add custom emoji'}));
        await userEvent.upload(document.querySelector('input[type="file"]') as HTMLInputElement, new File(['x'], 'Party Parrot.gif', {type: 'image/gif'}));
        await screen.findByText(':party_parrot: is free.');
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));

        expect(await screen.findByRole('heading', {name: 'Just added'})).toBeInTheDocument();
        const fresh = screen.getByRole('button', {name: 'party_parrot'});
        expect(fresh).toHaveFocus();
        expect(screen.getByText(':party_parrot:', {selector: 'code'})).toBeInTheDocument();

        await userEvent.keyboard('{Enter}');
        expect(onPick).toHaveBeenCalledWith('party_parrot');
        expect(onClose).not.toHaveBeenCalled();
    });

    test('hands the picker kit to the plugins\' tabs, whose views go back on Escape too', async () => {
        // A plugin tab with an add view, made of the kit as a plugin would.
        const component = ({pickerKit}: EmojiPickerTabProps) => {
            const kit = pickerKit as PickerKit;
            const [view, open, back] = kit.usePickerView<'add'>();
            if (view === 'add') {
                return (
                    <kit.PickerViewFrame
                        title='Add a sticker'
                        onBack={back}
                    >
                        {'sticker form'}
                    </kit.PickerViewFrame>
                );
            }
            return (
                <kit.PickerViewButtons
                    onAdd={() => open('add')}
                    addLabel='Add a sticker'
                    manageLabel='Manage stickers'
                />
            );
        };
        const onClose = jest.fn();
        renderWithContext(
            <EmojiPicker
                anchor={null}
                onPick={jest.fn()}
                onClose={onClose}
                compose={{channelId: 'channel', insertText: jest.fn()}}
            />,
            stateWith([makeTab({id: 'stickers', label: 'Stickers', component})]),
        );
        await userEvent.click(screen.getByRole('tab', {name: 'Stickers'}));

        // Only the buttons a tab passes a handler for.
        expect(screen.queryByRole('button', {name: 'Manage stickers'})).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('button', {name: 'Add a sticker'}));
        expect(screen.getByRole('region', {name: 'Add a sticker'})).toHaveTextContent('sticker form');

        await userEvent.keyboard('{Escape}');
        expect(screen.queryByRole('region', {name: 'Add a sticker'})).not.toBeInTheDocument();
        expect(onClose).not.toHaveBeenCalled();
    });

    // The search ranks recently used emoji first: it must get their ids, not their names (an unknown id crashed the
    // whole app on the first letter typed).
    test('searches emoji, recently used ones first', async () => {
        // The grids draw at once here.
        const lazy = window.IntersectionObserver;
        window.IntersectionObserver = class {
            constructor(private callback: IntersectionObserverCallback) {}
            observe() {
                this.callback([{isIntersecting: true} as IntersectionObserverEntry], this as unknown as IntersectionObserver);
            }
            disconnect() {}
        } as unknown as typeof IntersectionObserver;
        const onPick = jest.fn();
        renderWithContext(
            <EmojiPicker
                anchor={null}
                onPick={onPick}
                onClose={jest.fn()}
            />,
            {} as DeepPartial<GlobalState>,
        );

        await userEvent.type(screen.getByRole('textbox', {name: 'Search emoji'}), 'ta');

        expect(screen.getByRole('heading', {name: 'Search results'})).toBeInTheDocument();
        const results = screen.getAllByRole('button', {name: /ta/});

        // :tada: is one of the usual reactions shown as recently used until anything is.
        expect(results[0]).toHaveAccessibleName('tada');
        await userEvent.click(results[0]);
        expect(onPick).toHaveBeenCalledWith('tada');

        window.IntersectionObserver = lazy;
    });
});
