// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Channel} from '@mattermost/types/channels';
import type {TeamKind} from '@mattermost/types/teams';

import {leaveChannel} from 'actions/views/channel';

import DeleteChannelModal from 'components/delete_channel_modal';
import LeaveChannelModal from 'components/leave_channel_modal';
import UnarchiveChannelModal from 'components/unarchive_channel_modal';

import {LAYER_ID} from 'fusion/components/layer';
import {fakeMatchMedia} from 'fusion/hooks/media_test_utils';
import {muteFor} from 'fusion/sidebar/timed_mute';
import {openDialog} from 'fusion/utils/modals';
import {act, fireEvent, renderWithContext, screen, userEvent, within} from 'tests/react_testing_utils';
import {ModalIdentifiers} from 'utils/constants';
import {TestHelper} from 'utils/test_helper';

import ChannelNameMenu from './channel_name_menu';

// The menu under a conversation's name in the chat header: the actions per kind of channel and role, each once, in
// their groups; leaving and archiving asking first; the keyboard; and nobody seeing what their role doesn't allow.

let mockTeamPerms = new Set<string>();
let mockChannelPerms = new Set<string>();
jest.mock('mattermost-redux/selectors/entities/roles', () => ({
    ...jest.requireActual('mattermost-redux/selectors/entities/roles'),
    haveITeamPermission: jest.fn((state: unknown, teamId: string, permission: string) => mockTeamPerms.has(permission)),
    haveIChannelPermission: jest.fn((state: unknown, teamId: string, channelId: string, permission: string) => mockChannelPerms.has(permission)),
}));
jest.mock('mattermost-redux/actions/channels', () => ({
    ...jest.requireActual('mattermost-redux/actions/channels'),
    favoriteChannel: jest.fn(() => ({type: 'MOCK_FAVORITE'})),
    getChannelStats: jest.fn(() => ({type: 'MOCK_STATS'})),
}));
jest.mock('actions/views/channel', () => ({
    ...jest.requireActual('actions/views/channel'),
    leaveChannel: jest.fn(() => ({type: 'MOCK_LEAVE'})),
    leaveDirectChannel: jest.fn(() => ({type: 'MOCK_LEAVE_DIRECT'})),
}));
jest.mock('fusion/sidebar/timed_mute', () => ({
    MUTE_DURATIONS: [[36e5, {id: 'fusion.mute.1h', defaultMessage: 'For 1 hour'}], [0, {id: 'fusion.mute.forever', defaultMessage: 'Until I unmute it'}]],
    getMuteEnd: () => 0,
    muteFor: jest.fn(() => ({type: 'MOCK_MUTE'})),
    unmute: jest.fn(() => ({type: 'MOCK_UNMUTE'})),
}));
jest.mock('fusion/utils/modals', () => ({
    ...jest.requireActual('fusion/utils/modals'),
    openDialog: jest.fn(() => ({type: 'MOCK_OPEN_DIALOG'})),
    openAddToGroup: jest.fn(() => ({type: 'MOCK_ADD_TO_GROUP'})),
}));
jest.mock('utils/popouts/popout_windows', () => ({canPopout: () => false, getPopoutChannelTitle: jest.fn(), popoutChannel: jest.fn()}));
const mockToast = jest.fn();
jest.mock('fusion/shell/toast_context', () => ({useToast: () => mockToast}));
const mockDialogs = {addPeople: jest.fn(), confirm: jest.fn(), channelPermissions: jest.fn(), exportChannel: jest.fn()};
jest.mock('fusion/shell/dialogs_context', () => ({useDialogs: () => mockDialogs}));

const ch = (id: string, name: string, extra: Partial<Channel> = {}) => TestHelper.getChannelMock({id, name, display_name: name, team_id: 't1', type: 'O', delete_at: 0, creator_id: 'me', purpose: '', header: '', ...extra});
const general = ch('town', 'general');
const dev = ch('dev', 'dev', {purpose: 'Where the code is talked about'});
const secret = ch('secret', 'secret', {type: 'P'});
const old = ch('old', 'old', {delete_at: 5});
const dm = ch('dm', 'me__bob', {type: 'D', team_id: '', display_name: 'bob', header: 'Pairing on Fridays'});
const gm = ch('gm', 'gm-abc', {type: 'G', team_id: '', display_name: 'bob, carol, me'});
const policy = ch('pol', 'policy', {type: 'P', policy_enforced: true});
const all = [general, dev, secret, old, dm, gm, policy];

const appAction = jest.fn();
function state({kind = 'community' as TeamKind, allChannels = false, apps = false} = {}) {
    return {
        entities: {
            general: {config: {}},
            users: {
                currentUserId: 'me',
                profiles: {
                    me: TestHelper.getUserMock({id: 'me', username: 'me', roles: 'system_user', delete_at: 0}),
                    bob: TestHelper.getUserMock({id: 'bob', username: 'bob', roles: 'system_user', delete_at: 0}),
                },
            },
            teams: {
                currentTeamId: 't1',
                teams: {t1: TestHelper.getTeamMock({id: 't1', name: 'lumen', display_name: 'Lumen', kind, all_channels_membership: allChannels})},
                myMembers: {t1: {team_id: 't1', user_id: 'me', roles: 'team_user'}},
            },
            channels: {
                currentChannelId: '',
                channels: Object.fromEntries(all.map((c) => [c.id, c])),
                channelsInTeam: {t1: new Set(all.filter((c) => c.team_id).map((c) => c.id))},
                myMembers: Object.fromEntries(all.map((c) => [c.id, TestHelper.getChannelMembershipMock({channel_id: c.id, user_id: 'me'})])),
                messageCounts: {},
                stats: {dev: {channel_id: 'dev', member_count: 12, guest_count: 0, pinnedpost_count: 0, files_count: 0}},
            },
            preferences: {myPreferences: {}},
        },
        plugins: {
            components: {
                ChannelHeader: apps ? [
                    {id: 'jira', pluginId: 'com.example.jira', text: 'Create an issue', action: appAction, shouldRender: () => true},
                    {id: 'hidden', pluginId: 'com.example.other', text: 'Not here', action: jest.fn(), shouldRender: () => false},
                ] : [],
            },
        },
    };
}

// The roles, by the permissions the menu asks about. A system admin has every permission, as a community admin has
// here.
const MEMBERS = ['manage_public_channel_members', 'manage_private_channel_members'];
const CHANNEL_ADMIN = [...MEMBERS, 'manage_channel_roles', 'manage_public_channel_properties', 'manage_private_channel_properties', 'convert_public_channel_to_private'];
const ALL_CHANNEL = [...CHANNEL_ADMIN, 'delete_public_channel', 'delete_private_channel'];
const ROLES: Record<string, {team: string[]; channel: string[]}> = {
    guest: {team: [], channel: []},
    member: {team: [], channel: MEMBERS},
    channelAdmin: {team: [], channel: CHANNEL_ADMIN},
    communityAdmin: {team: ['manage_team', 'remove_user_from_team', 'manage_team_roles'], channel: ALL_CHANNEL},
    systemAdmin: {team: ['manage_team', 'remove_user_from_team', 'manage_team_roles', 'add_user_to_team'], channel: ALL_CHANNEL},
};
const actAs = (role: keyof typeof ROLES) => {
    mockTeamPerms = new Set(ROLES[role].team);
    mockChannelPerms = new Set(ROLES[role].channel);
};

const onClose = jest.fn();
let anchor: HTMLButtonElement;
const render = (channel: Channel, s = state()) => renderWithContext(
    <ChannelNameMenu
        channel={channel}
        title={channel.display_name}
        anchor={anchor}
        onClose={onClose}
    />,
    s as never,
);
const menu = () => screen.getByRole('menu');
const ids = () => Array.from(menu().querySelectorAll('[data-action]')).map((el) => el.getAttribute('data-action'));
const item = (name: string | RegExp) => screen.queryByRole('menuitem', {name});

beforeEach(() => {
    jest.clearAllMocks();
    actAs('member');
    document.body.innerHTML = '';
    const layer = document.createElement('div');
    layer.id = LAYER_ID;
    document.body.appendChild(layer);
    anchor = document.createElement('button');
    anchor.textContent = 'channel name';
    document.body.appendChild(anchor);
});

describe('fusion/channel/ChannelNameMenu: the actions per kind of channel and role', () => {
    const QUICK = ['mute', 'notifications', 'favourite', 'copy-link'];
    const cases: Array<[string, Channel, keyof typeof ROLES, string[]]> = [
        ['public', dev, 'guest', [...QUICK, 'settings', 'export', 'leave']],
        ['public', dev, 'member', [...QUICK, 'add-people', 'settings', 'export', 'leave']],
        ['public', dev, 'channelAdmin', [...QUICK, 'add-people', 'settings', 'permissions', 'export', 'leave']],
        ['public', dev, 'communityAdmin', [...QUICK, 'add-people', 'settings', 'permissions', 'make-default', 'export', 'leave', 'archive']],
        ['public', dev, 'systemAdmin', [...QUICK, 'add-people', 'settings', 'permissions', 'make-default', 'export', 'leave', 'archive']],
        ['private', secret, 'guest', [...QUICK, 'settings', 'export', 'leave']],
        ['private', secret, 'member', [...QUICK, 'add-people', 'settings', 'export', 'leave']],
        ['private', secret, 'channelAdmin', [...QUICK, 'add-people', 'settings', 'permissions', 'export', 'leave']],
        ['private', secret, 'communityAdmin', [...QUICK, 'add-people', 'settings', 'permissions', 'export', 'leave', 'archive']],
        ['direct', dm, 'guest', [...QUICK, 'edit-header', 'export', 'close']],
        ['direct', dm, 'member', [...QUICK, 'edit-header', 'export', 'close']],
        ['group', gm, 'guest', [...QUICK, 'add-people', 'edit-header', 'rename-group', 'convert', 'export', 'close']],
        ['group', gm, 'member', [...QUICK, 'add-people', 'edit-header', 'rename-group', 'convert', 'export', 'close']],
        ['archived', old, 'guest', [...QUICK, 'export', 'close']],
        ['archived', old, 'member', [...QUICK, 'add-people', 'export', 'close']],
        ['archived', old, 'communityAdmin', [...QUICK, 'add-people', 'export', 'unarchive', 'close']],
        ['default', general, 'member', [...QUICK, 'add-people', 'settings', 'export']],
        ['default', general, 'communityAdmin', [...QUICK, 'add-people', 'settings', 'permissions', 'export', 'archive']],
    ];

    test.each(cases)('a %s channel, for a %s, offers its actions in their groups, in order', (kind, channel, role, expected) => {
        actAs(role);
        render(channel);
        expect(ids()).toEqual(expected);
    });

    test.each(cases)('a %s channel, for a %s, offers each action once', (kind, channel, role) => {
        actAs(role);
        render(channel);
        fireEvent.click(menu().querySelector('[data-action="mute"]')!);
        const shown = ids();
        expect(new Set(shown).size).toBe(shown.length);
        expect(shown).toContain('mute-0');
    });

    test('the header says what the channel is: its name, kind and purpose, who made it and how many are in it', () => {
        render(dev);
        const head = menu().querySelector('.am-cm-head') as HTMLElement;
        expect(within(head).getByText('dev')).toBeInTheDocument();
        expect(within(head).getByText('Where the code is talked about')).toBeInTheDocument();
        expect(within(head).getByText(/12 members/)).toBeInTheDocument();
        expect(menu()).toHaveAttribute('aria-describedby', head.id);
    });

    test('without a purpose, the header shows the topic', () => {
        render(dm);
        expect(within(menu().querySelector('.am-cm-head') as HTMLElement).getByText('Pairing on Fridays')).toBeInTheDocument();
    });

    test('an archived channel says so in the header', () => {
        render(old);
        expect(within(menu().querySelector('.am-cm-head') as HTMLElement).getByText('Archived')).toBeInTheDocument();
    });

    // What the settings dialog changes isn't repeated: its header, public or private.
    test('a channel\'s header and privacy are in its settings, not in the menu', () => {
        actAs('communityAdmin');
        render(dev);
        expect(item('Edit header')).not.toBeInTheDocument();
        expect(item('Convert to private channel…')).not.toBeInTheDocument();
        expect(item('Channel settings')).toBeInTheDocument();
    });

    test('someone who may make a channel private without its settings\' Info tab still finds it in the menu', () => {
        mockChannelPerms = new Set([...MEMBERS, 'convert_public_channel_to_private']);
        render(dev);
        expect(item('Convert to private channel…')).toBeInTheDocument();
    });

    // In a community that has everyone in all its channels, members mute channels instead of leaving them: the mute
    // item says so, and there's no second one.
    test('in an all-channels community, Mute says why there is no Leave, and adding people is gone', () => {
        render(dev, state({allChannels: true}));
        expect(item('Leave channel…')).not.toBeInTheDocument();
        expect(item(/Add members/)).not.toBeInTheDocument();
        expect(screen.getAllByRole('menuitem', {name: /Mute channel/})).toHaveLength(1);
        expect(item(/Everyone in Lumen is in all its channels/)).toBeInTheDocument();
    });

    test('the apps\' actions are one group, with their apps\' icons, and only those that want to show', async () => {
        render(dev, state({apps: true}));
        const apps = screen.getByRole('group', {name: 'Apps'});
        const action = within(apps).getByRole('menuitem', {name: 'Create an issue'});
        expect(action.querySelector('.am-mi-app')).toHaveAttribute('data-plugin-id', 'com.example.jira');
        expect(item('Not here')).not.toBeInTheDocument();
        expect(ids().indexOf('app:jira')).toBeGreaterThan(ids().indexOf('export'));
        expect(ids().indexOf('app:jira')).toBeLessThan(ids().indexOf('leave'));
        await userEvent.click(action);
        expect(appAction).toHaveBeenCalledWith('dev');
        expect(onClose).toHaveBeenCalled();
    });

    test('Favourite is a checkbox, spelt the British way', async () => {
        render(dev);
        const favourite = screen.getByRole('menuitemcheckbox', {name: 'Favourite'});
        expect(favourite).toHaveAttribute('aria-checked', 'false');
        await userEvent.click(favourite);
        expect(mockToast).toHaveBeenCalledWith('Added to Favourites');
    });

    test('Mute shows its durations under it, then mutes for the one chosen', async () => {
        render(dev);
        const mute = screen.getByRole('menuitem', {name: 'Mute channel'});
        expect(mute).toHaveAttribute('aria-expanded', 'false');
        await userEvent.click(mute);
        expect(mute).toHaveAttribute('aria-expanded', 'true');
        await userEvent.click(within(screen.getByRole('group', {name: 'Mute for'})).getByRole('menuitem', {name: 'For 1 hour'}));
        expect(muteFor).toHaveBeenCalledWith('dev', 36e5);
        expect(onClose).toHaveBeenCalled();
    });

    test('on a phone the menu is a sheet from the bottom of the screen', () => {
        const original = window.matchMedia;
        fakeMatchMedia((query) => query.includes('max-width: 640px'));
        render(dev);
        expect(menu()).toHaveClass('am-sheet');
        window.matchMedia = original;
    });
});

describe('fusion/channel/ChannelNameMenu: leaving and archiving ask first', () => {
    test('leaving a public channel asks, and leaves once confirmed', async () => {
        render(dev);
        await userEvent.click(screen.getByRole('menuitem', {name: 'Leave channel…'}));
        expect(leaveChannel).not.toHaveBeenCalled();
        expect(mockDialogs.confirm).toHaveBeenCalledWith(expect.objectContaining({title: 'Leave #dev?', confirmLabel: 'Leave channel', danger: true}));
        await act(async () => {
            await mockDialogs.confirm.mock.calls[0][0].onConfirm();
        });
        expect(leaveChannel).toHaveBeenCalledWith('dev');
        expect(mockToast).toHaveBeenCalledWith('You left #dev');
    });

    test('leaving a private channel says one must be added again', async () => {
        render(secret);
        await userEvent.click(screen.getByRole('menuitem', {name: 'Leave channel…'}));
        expect(mockDialogs.confirm).toHaveBeenCalledWith(expect.objectContaining({message: expect.stringContaining('added again')}));
        expect(leaveChannel).not.toHaveBeenCalled();
    });

    test('leaving a channel with a membership policy opens its own dialog, which offers muting', async () => {
        render(policy);
        await userEvent.click(screen.getByRole('menuitem', {name: 'Leave channel…'}));
        expect(openDialog).toHaveBeenCalledWith(ModalIdentifiers.LEAVE_PRIVATE_CHANNEL_MODAL, LeaveChannelModal, {channel: policy});
        expect(leaveChannel).not.toHaveBeenCalled();
    });

    test('archiving opens the archive dialog, destructive and last', async () => {
        actAs('communityAdmin');
        render(dev);
        const archive = screen.getByRole('menuitem', {name: 'Archive channel…'});
        expect(archive).toHaveClass('am-danger');
        expect(ids()[ids().length - 1]).toBe('archive');
        await userEvent.click(archive);
        expect(openDialog).toHaveBeenCalledWith(ModalIdentifiers.DELETE_CHANNEL, DeleteChannelModal, {channel: dev});
    });

    test('unarchiving opens its confirmation', async () => {
        actAs('communityAdmin');
        render(old);
        await userEvent.click(screen.getByRole('menuitem', {name: 'Unarchive channel'}));
        expect(openDialog).toHaveBeenCalledWith(ModalIdentifiers.UNARCHIVE_CHANNEL, UnarchiveChannelModal, {channel: old});
    });
});

describe('fusion/channel/ChannelNameMenu: the keyboard', () => {
    test('the first item takes the focus; the arrow keys, Home and End move between the items, round at the ends', async () => {
        render(dev);
        const items = Array.from(menu().querySelectorAll<HTMLElement>('[role^="menuitem"]'));
        expect(items.length).toBeGreaterThan(3);
        expect(items[0]).toHaveFocus();
        await userEvent.keyboard('{ArrowDown}');
        expect(items[1]).toHaveFocus();
        await userEvent.keyboard('{End}');
        expect(items[items.length - 1]).toHaveFocus();
        await userEvent.keyboard('{ArrowDown}');
        expect(items[0]).toHaveFocus();
        await userEvent.keyboard('{ArrowUp}');
        expect(items[items.length - 1]).toHaveFocus();
        await userEvent.keyboard('{Home}');
        expect(items[0]).toHaveFocus();
    });

    test('Enter opens Mute\'s durations, and the arrows go into them', async () => {
        render(dev);
        expect(screen.getByRole('menuitem', {name: 'Mute channel'})).toHaveFocus();
        await userEvent.keyboard('{Enter}');
        await userEvent.keyboard('{ArrowDown}');
        expect(screen.getByRole('menuitem', {name: 'For 1 hour'})).toHaveFocus();
    });

    test('Escape closes it', async () => {
        render(dev);
        await userEvent.keyboard('{Escape}');
        expect(onClose).toHaveBeenCalled();
    });
});

describe('fusion/channel/ChannelNameMenu: roles that mustn\'t see an admin\'s or a moderator\'s items', () => {
    test('a member sees neither permissions, nor making it the default, nor archiving', () => {
        actAs('member');
        render(dev);
        expect(ids()).not.toContain('permissions');
        expect(ids()).not.toContain('make-default');
        expect(ids()).not.toContain('archive');
    });

    test('a guest doesn\'t add people', () => {
        actAs('guest');
        render(dev);
        expect(ids()).not.toContain('add-people');
    });

    test('a member doesn\'t unarchive', () => {
        actAs('member');
        render(old);
        expect(ids()).not.toContain('unarchive');
    });

    test('in a community, a channel\'s admins don\'t archive it; in a workplace community, they do', () => {
        actAs('channelAdmin');
        const {unmount} = render(dev);
        expect(ids()).not.toContain('archive');
        unmount();
        render(dev, state({kind: 'workplace'}));
        expect(ids()).toContain('archive');
    });

    test('a channel admin doesn\'t archive the default channel, a community admin does', () => {
        mockChannelPerms = new Set(ALL_CHANNEL);
        const {unmount} = render(general);
        expect(ids()).not.toContain('archive');
        unmount();
        actAs('communityAdmin');
        render(general);
        expect(ids()).toContain('archive');
    });

    test('exporting can be kept to admins', () => {
        actAs('member');
        const s = state();
        s.entities.general.config = {RestrictChannelExportToAdmins: 'true'} as never;
        render(dev, s);
        expect(ids()).not.toContain('export');
    });
});
