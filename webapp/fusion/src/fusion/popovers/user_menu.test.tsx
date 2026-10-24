// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {openDirectChannelToUserId} from 'actions/channel_actions';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';

import {UserMenu, useUserMenu} from './user_menu';

// Permissions, as "<team or channel>:<permission>" strings, that haveITeamPermission and haveIChannelPermission grant.
let mockPerms = new Set<string>();
let mockCanManage = false;

jest.mock('mattermost-redux/selectors/entities/roles', () => ({
    ...jest.requireActual('mattermost-redux/selectors/entities/roles'),
    haveITeamPermission: jest.fn((state: unknown, teamId: string, permission: string) => mockPerms.has(`${teamId}:${permission}`)),
    haveIChannelPermission: jest.fn((state: unknown, teamId: string, channelId: string, permission: string) => mockPerms.has(`${channelId}:${permission}`)),
}));
jest.mock('mattermost-redux/selectors/entities/channels', () => ({
    ...jest.requireActual('mattermost-redux/selectors/entities/channels'),
    canManageAnyChannelMembersInCurrentTeam: jest.fn(() => mockCanManage),
}));

const mockUpdateRoles = jest.fn(() => ({type: 'NOOP_ROLES'}));
jest.mock('mattermost-redux/actions/channels', () => ({
    ...jest.requireActual('mattermost-redux/actions/channels'),
    updateChannelMemberSchemeRoles: (...args: unknown[]) => mockUpdateRoles(...args),
}));

const mockBlocking = {enabled: true, level: '' as '' | 'ignore' | 'block', toggleIgnore: jest.fn(), toggleBlock: jest.fn(), block: jest.fn()};
jest.mock('fusion/privacy/message_requests', () => ({useBlockActions: () => mockBlocking}));

const mockDialogs = {timeOut: jest.fn(), removeFromTeam: jest.fn(), confirm: jest.fn()};
jest.mock('fusion/shell/dialogs_context', () => ({useDialogs: () => mockDialogs}));

const mockToast = jest.fn();
jest.mock('fusion/shell/toast_context', () => ({useToast: () => mockToast}));

jest.mock('fusion/attributes/edit_attributes_item', () => () => null);
jest.mock('components/add_user_to_channel_modal', () => () => null);
jest.mock('actions/channel_actions', () => ({openDirectChannelToUserId: jest.fn(() => ({type: 'NOOP_DM'}))}));
jest.mock('./user_popover', () => ({userId}: {userId: string}) => (
    <div
        role='dialog'
        aria-label={`Profile of ${userId}`}
    />
));

const user = (id: string, roles = 'system_user') => ({id, username: id, roles, delete_at: 0, first_name: '', last_name: '', nickname: ''});
const channel = (id: string, name: string, extra = {}) => ({id, team_id: 't1', type: 'O', name, display_name: name, delete_at: 0, ...extra});

function state({timedOut = false, allChannels = false, channelName = 'dev', channelType = 'O', otherInT2 = false, membersInChannel = {}} = {}) {
    return {
        entities: {
            general: {config: {}},
            users: {
                currentUserId: 'me',
                profiles: {me: user('me'), sofia: user('sofia'), boss: user('boss', 'system_admin system_user')},
                profilesInChannel: {c1: new Set(['me', 'sofia', 'boss'])},
            },
            teams: {
                currentTeamId: 't1',
                teams: {
                    t1: {id: 't1', name: 'lumen', display_name: 'Lumen', delete_at: 0, all_channels_membership: allChannels},
                    t2: {id: 't2', name: 'other', display_name: 'Other community', delete_at: 0},
                },
                myMembers: {t1: {team_id: 't1', user_id: 'me', roles: 'team_user'}, t2: {team_id: 't2', user_id: 'me', roles: 'team_user'}},
                membersInTeam: {
                    t1: {sofia: {team_id: 't1', user_id: 'sofia', delete_at: 0}, boss: {team_id: 't1', user_id: 'boss', delete_at: 0}},
                    t2: otherInT2 ? {sofia: {team_id: 't2', user_id: 'sofia', delete_at: 0}} : {},
                },
                timeouts: timedOut ? {t1: [{user_id: 'sofia', until: Date.now() + 60000}]} : {},
            },
            channels: {
                currentChannelId: 'c1',
                channels: {c1: channel('c1', channelName, {type: channelType})},
                channelsInTeam: {t1: new Set(['c1'])},
                myMembers: {c1: {channel_id: 'c1', user_id: 'me', roles: 'channel_user'}},
                membersInChannel: {c1: membersInChannel},
            },
            preferences: {myPreferences: {}},
        },
    };
}

const target = (userId: string) => ({userId, anchor: document.body, point: {x: 10, y: 10}});

const items = () => screen.getAllByRole('menuitem').map((el) => el.textContent);

beforeEach(() => {
    if (!document.getElementById('am-layer')) {
        const layer = document.createElement('div');
        layer.id = 'am-layer';
        document.body.appendChild(layer);
    }
});

describe('fusion/popovers/UserMenu', () => {
    beforeEach(() => {
        mockPerms = new Set();
        mockCanManage = false;
        mockBlocking.enabled = true;
        mockBlocking.level = '';
        mockUpdateRoles.mockClear();
    });

    const render = (userId: string, s = state(), onClose = jest.fn(), profile = true) => {
        renderWithContext(
            <UserMenu
                target={target(userId)}
                onProfile={profile ? jest.fn() : undefined}
                onClose={onClose}
            />,
            s as never,
        );
        return onClose;
    };

    test('offers a member nothing but the plain actions on someone else', () => {
        render('sofia');
        expect(screen.getByRole('menu', {name: 'sofia'})).toBeInTheDocument();
        expect(items()).toEqual(['Profile', 'Message', 'Mention', 'Copy username', 'Ignore', 'Block…']);
    });

    test('has no Message, Ignore, Block or moderation on oneself, even for a moderator', () => {
        mockPerms = new Set(['t1:remove_user_from_team', 't1:manage_team', 'c1:manage_public_channel_members']);
        render('me');
        expect(items()).toEqual(['Profile', 'Mention', 'Copy username']);
    });

    test('a system admin can be ignored but not blocked', () => {
        render('boss');
        expect(screen.getByRole('menuitem', {name: 'Ignore'})).toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: /Block/})).not.toBeInTheDocument();
    });

    test('hides ignoring and blocking when the admin turned them off', () => {
        mockBlocking.enabled = false;
        render('sofia');
        expect(screen.queryByRole('menuitem', {name: 'Ignore'})).not.toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: /Block/})).not.toBeInTheDocument();
    });

    test('toggles ignoring and blocking, closing the menu', async () => {
        const onClose = render('sofia');
        await userEvent.click(screen.getByRole('menuitem', {name: 'Ignore'}));
        expect(mockBlocking.toggleIgnore).toHaveBeenCalled();
        expect(onClose).toHaveBeenCalledTimes(1);
        await userEvent.click(screen.getByRole('menuitem', {name: 'Block…'}));
        expect(mockBlocking.toggleBlock).toHaveBeenCalled();
    });

    test('someone blocked can be unblocked, and ignoring is disabled', () => {
        mockBlocking.level = 'block';
        render('sofia');
        expect(screen.getByRole('menuitem', {name: 'Ignore'})).toBeDisabled();
        expect(screen.getByRole('menuitem', {name: 'Unblock'})).toBeInTheDocument();
    });

    test('someone ignored shows Stop ignoring', () => {
        mockBlocking.level = 'ignore';
        render('sofia');
        expect(screen.getByRole('menuitem', {name: 'Stop ignoring'})).toBeEnabled();
    });

    test('gives a moderator the mod view, timeouts and removals', async () => {
        mockPerms = new Set(['t1:remove_user_from_team', 'c1:manage_public_channel_members']);
        render('sofia');
        expect(items()).toEqual(expect.arrayContaining(['Mod view', 'Time out…', 'Remove from channel', 'Remove from community…']));

        await userEvent.click(screen.getByRole('menuitem', {name: 'Time out…'}));
        expect(mockDialogs.timeOut).toHaveBeenCalledWith('sofia');
        await userEvent.click(screen.getByRole('menuitem', {name: 'Remove from community…'}));
        expect(mockDialogs.removeFromTeam).toHaveBeenCalledWith('sofia');
    });

    test('someone allowed only to delete others\' posts gets the mod view, not removals', () => {
        mockPerms = new Set(['t1:delete_others_posts']);
        render('sofia');
        expect(screen.getByRole('menuitem', {name: 'Mod view'})).toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: 'Time out…'})).not.toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: 'Remove from community…'})).not.toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: 'Remove from channel'})).not.toBeInTheDocument();
    });

    test('offers to lift the timeout of someone timed out', () => {
        mockPerms = new Set(['t1:remove_user_from_team']);
        render('sofia', state({timedOut: true}));
        expect(screen.getByRole('menuitem', {name: 'Remove timeout'})).toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: 'Time out…'})).not.toBeInTheDocument();
    });

    test('a member who may manage the channel\'s members still doesn\'t remove others from it', () => {
        mockPerms = new Set(['c1:manage_public_channel_members']);
        render('sofia');
        expect(screen.queryByRole('menuitem', {name: 'Remove from channel'})).not.toBeInTheDocument();
    });

    test('the channel\'s admins remove others from a channel', () => {
        mockPerms = new Set(['c1:manage_public_channel_members', 'c1:manage_channel_roles']);
        render('sofia');
        expect(screen.getByRole('menuitem', {name: 'Remove from channel'})).toBeInTheDocument();
    });

    test('the community\'s admins remove others from a channel', () => {
        mockPerms = new Set(['c1:manage_public_channel_members', 't1:manage_team']);
        render('sofia');
        expect(screen.getByRole('menuitem', {name: 'Remove from channel'})).toBeInTheDocument();
    });

    test('nobody is removed from the default channel', () => {
        mockPerms = new Set(['c1:manage_public_channel_members', 'c1:manage_channel_roles']);
        render('sofia', state({channelName: 'general'}));
        expect(screen.queryByRole('menuitem', {name: 'Remove from channel'})).not.toBeInTheDocument();
    });

    test('nobody is removed from a channel of a community that has everyone in every channel', () => {
        mockPerms = new Set(['c1:manage_public_channel_members', 'c1:manage_channel_roles']);
        render('sofia', state({allChannels: true}));
        expect(screen.queryByRole('menuitem', {name: 'Remove from channel'})).not.toBeInTheDocument();
    });

    test('a plain member offers neither channel role action', () => {
        render('sofia');
        expect(screen.queryByRole('menuitem', {name: 'Make channel admin'})).not.toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: 'Make member'})).not.toBeInTheDocument();
    });

    test('someone allowed to manage channel roles may make a member a channel admin', async () => {
        mockPerms = new Set(['c1:manage_channel_roles']);
        render('sofia');
        expect(screen.queryByRole('menuitem', {name: 'Make member'})).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('menuitem', {name: 'Make channel admin'}));
        expect(mockUpdateRoles).toHaveBeenCalledWith('c1', 'sofia', true, true);
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('sofia is now a channel admin'));
    });

    test('and may make a channel admin a plain member again', async () => {
        mockPerms = new Set(['c1:manage_channel_roles']);
        render('sofia', state({membersInChannel: {sofia: {channel_id: 'c1', user_id: 'sofia', roles: 'channel_user', scheme_admin: true}}}));
        expect(screen.queryByRole('menuitem', {name: 'Make channel admin'})).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('menuitem', {name: 'Make member'}));
        expect(mockUpdateRoles).toHaveBeenCalledWith('c1', 'sofia', true, false);
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('sofia is now a member'));
    });

    test('without the permission, nobody offers either channel role action, even a channel admin', () => {
        render('sofia', state({membersInChannel: {sofia: {channel_id: 'c1', user_id: 'sofia', roles: 'channel_user', scheme_admin: true}}}));
        expect(screen.queryByRole('menuitem', {name: 'Make channel admin'})).not.toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: 'Make member'})).not.toBeInTheDocument();
    });

    test('a guest is never offered channel admin', () => {
        mockPerms = new Set(['c1:manage_channel_roles']);
        render('sofia', {
            ...state(),
            entities: {
                ...state().entities,
                users: {...state().entities.users, profiles: {...state().entities.users.profiles, sofia: user('sofia', 'system_guest')}},
            },
        });
        expect(screen.queryByRole('menuitem', {name: 'Make channel admin'})).not.toBeInTheDocument();
    });

    test('a direct message has no channel roles to change', () => {
        mockPerms = new Set(['c1:manage_channel_roles']);
        render('sofia', state({channelType: 'D'}));
        expect(screen.queryByRole('menuitem', {name: 'Make channel admin'})).not.toBeInTheDocument();
    });

    test('someone not a member of the channel is not offered a channel role either', () => {
        mockPerms = new Set(['c1:manage_channel_roles']);
        render('sofia', {
            ...state(),
            entities: {
                ...state().entities,
                users: {...state().entities.users, profilesInChannel: {c1: new Set(['me', 'boss'])}},
            },
        });
        expect(screen.queryByRole('menuitem', {name: 'Make channel admin'})).not.toBeInTheDocument();
    });

    test('offers to add someone to a channel only with the permission', () => {
        render('sofia');
        expect(screen.queryByRole('menuitem', {name: 'Add to a channel'})).not.toBeInTheDocument();
    });

    test('offers to add someone to a channel with the permission', () => {
        mockCanManage = true;
        render('sofia');
        expect(screen.getByRole('menuitem', {name: 'Add to a channel'})).toBeInTheDocument();
    });

    test('invites to the other communities one may add people to', () => {
        mockPerms = new Set(['t2:add_user_to_team']);
        render('sofia');
        expect(screen.getByRole('menuitem', {name: 'Invite to a community'})).toBeInTheDocument();
        expect(screen.getByRole('menu', {name: 'Communities'})).toHaveTextContent('Other community');
    });

    test('does not invite to a community they are already in, or without the permission', () => {
        mockPerms = new Set(['t2:add_user_to_team']);
        render('sofia', state({otherInT2: true}));
        expect(screen.queryByRole('menuitem', {name: 'Invite to a community'})).not.toBeInTheDocument();
    });

    test('does not invite without the permission', () => {
        render('sofia');
        expect(screen.queryByRole('menuitem', {name: 'Invite to a community'})).not.toBeInTheDocument();
    });

    test('tells why a message cannot be sent', async () => {
        jest.mocked(openDirectChannelToUserId).mockReturnValueOnce((() => ({error: {message: 'You can\'t message sofia.'}})) as never);
        render('sofia');
        await userEvent.click(screen.getByRole('menuitem', {name: 'Message'}));
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('You can\'t message sofia.'));
    });

    test('has no Profile item when there is no card to show', () => {
        render('sofia', state(), jest.fn(), false);
        expect(screen.queryByRole('menuitem', {name: 'Profile'})).not.toBeInTheDocument();
    });
});

describe('fusion/popovers/useUserMenu', () => {
    function People() {
        const [node, open, close] = useUserMenu();
        return (
            <>
                <button onContextMenu={(e) => open('sofia', e)}>{'Sofía avatar'}</button>
                <button onContextMenu={(e) => open('sofia', e)}>{'Sofía name'}</button>
                <button onClick={close}>{'close'}</button>
                {node}
            </>
        );
    }

    beforeEach(() => {
        mockPerms = new Set();
        mockBlocking.enabled = true;
        mockBlocking.level = '';
    });

    const render = () => renderWithContext(<People/>, state() as never);
    const rightClick = (name: string) => userEvent.pointer({keys: '[MouseRight]', target: screen.getByText(name)});

    test('a right click opens the menu', async () => {
        render();
        await rightClick('Sofía avatar');
        expect(screen.getByRole('menu', {name: 'sofia'})).toBeInTheDocument();
    });

    // The element that opened the menu is one of its toggles: pressing it again isn't a click elsewhere (which would
    // close the menu, and the context menu event reopen it).
    test('a second right click on the same element closes it', async () => {
        render();
        await rightClick('Sofía avatar');
        expect(screen.getByRole('menu', {name: 'sofia'})).toBeInTheDocument();
        await rightClick('Sofía avatar');
        expect(screen.queryByRole('menu', {name: 'sofia'})).not.toBeInTheDocument();
    });

    test('a right click on another element for the same person keeps a menu open there', async () => {
        render();
        await rightClick('Sofía avatar');
        await rightClick('Sofía name');
        expect(screen.getByRole('menu', {name: 'sofia'})).toBeInTheDocument();
    });

    test('Profile turns the menu into the profile card', async () => {
        render();
        await rightClick('Sofía avatar');
        await userEvent.click(screen.getByRole('menuitem', {name: 'Profile'}));
        expect(screen.queryByRole('menu', {name: 'sofia'})).not.toBeInTheDocument();
        expect(screen.getByRole('dialog', {name: 'Profile of sofia'})).toBeInTheDocument();

        // Right-clicking again opens the menu, not the card.
        await rightClick('Sofía name');
        expect(screen.getByRole('menu', {name: 'sofia'})).toBeInTheDocument();
        expect(screen.queryByRole('dialog', {name: 'Profile of sofia'})).not.toBeInTheDocument();
    });

    test('clicking elsewhere closes it', async () => {
        render();
        await rightClick('Sofía avatar');
        await userEvent.click(screen.getByText('close'));
        expect(screen.queryByRole('menu', {name: 'sofia'})).not.toBeInTheDocument();
    });
});
