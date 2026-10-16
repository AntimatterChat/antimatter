// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Channel} from '@mattermost/types/channels';

import {patchTeam} from 'mattermost-redux/actions/teams';

import {muteFor} from 'fusion/sidebar/timed_mute';
import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import ChannelMenu from './channel_menu';

// The default channel and all-channels communities in the channel menu: who may leave, archive, make default.

const mockNone: never[] = [];
const mockNoMappings = {};
let mockTeamPerms = new Set<string>();
let mockChannelPerms = new Set<string>();
jest.mock('mattermost-redux/selectors/entities/roles', () => ({
    ...jest.requireActual('mattermost-redux/selectors/entities/roles'),
    haveITeamPermission: jest.fn((state: unknown, teamId: string, permission: string) => mockTeamPerms.has(permission)),
    haveIChannelPermission: jest.fn((state: unknown, teamId: string, channelId: string, permission: string) => mockChannelPerms.has(permission)),
}));
jest.mock('mattermost-redux/selectors/entities/channel_categories', () => ({
    ...jest.requireActual('mattermost-redux/selectors/entities/channel_categories'),
    getCategoryInTeamWithChannel: () => undefined,
    getManagedCategoryMappings: () => mockNoMappings,
}));
jest.mock('selectors/views/channel_sidebar', () => ({getCategoriesForCurrentTeam: () => mockNone}));
jest.mock('fusion/sidebar/server_categories', () => ({
    SERVER_CATEGORY_PREFIX: 'managed_',
    canFileChannel: () => false,
    fileChannel: jest.fn(),
    fileChannelHere: jest.fn(),
    findServerCategory: () => undefined,
    getServerCategories: () => mockNone,
    getSidebarCategories: () => mockNone,
    isServerCategory: () => false,
}));
jest.mock('fusion/sidebar/timed_mute', () => ({
    MUTE_DURATIONS: [],
    getMuteEnd: () => 0,
    muteFor: jest.fn(() => ({type: 'MOCK_MUTE'})),
    unmute: jest.fn(() => ({type: 'MOCK_UNMUTE'})),
}));
jest.mock('mattermost-redux/actions/teams', () => ({
    ...jest.requireActual('mattermost-redux/actions/teams'),
    patchTeam: jest.fn(() => ({type: 'MOCK_PATCH_TEAM'})),
}));
jest.mock('utils/popouts/popout_windows', () => ({canPopout: () => false, getPopoutChannelTitle: jest.fn(), popoutChannel: jest.fn()}));
const mockToast = jest.fn();
jest.mock('fusion/shell/toast_context', () => ({useToast: () => mockToast}));
jest.mock('fusion/shell/dialogs_context', () => ({useDialogs: () => ({addPeople: jest.fn(), confirm: jest.fn()})}));

const ch = (id: string, name: string, extra: Partial<Channel> = {}) => TestHelper.getChannelMock({id, name, display_name: name, team_id: 't1', type: 'O', delete_at: 0, ...extra});
const town = ch('town', 'town-square');
const offTopic = ch('off', 'off-topic');
const dev = ch('dev', 'dev');
const secret = ch('secret', 'secret', {type: 'P'});
const all = [town, offTopic, dev, secret];

function state({allChannels = false, defaultChannelId = '', channels = all} = {}) {
    return {
        entities: {
            general: {config: {}},
            users: {currentUserId: 'me', profiles: {me: TestHelper.getUserMock({id: 'me', roles: 'system_user', delete_at: 0})}},
            teams: {
                currentTeamId: 't1',
                teams: {t1: TestHelper.getTeamMock({id: 't1', name: 'lumen', display_name: 'Lumen', all_channels_membership: allChannels, default_channel_id: defaultChannelId})},
                myMembers: {t1: {team_id: 't1', user_id: 'me', roles: 'team_user'}},
            },
            channels: {
                currentChannelId: '',
                channels: Object.fromEntries(channels.map((c) => [c.id, c])),
                channelsInTeam: {t1: new Set(channels.map((c) => c.id))},
                myMembers: Object.fromEntries(channels.map((c) => [c.id, TestHelper.getChannelMembershipMock({channel_id: c.id, user_id: 'me'})])),
                messageCounts: {},
            },
            preferences: {myPreferences: {}},
        },
    };
}

const MEMBER = ['delete_public_channel', 'delete_private_channel'];
const ADMIN = ['manage_team'];

describe('fusion/popovers/ChannelMenu: default channels and all-channels communities', () => {
    beforeEach(() => {
        mockTeamPerms = new Set();
        mockChannelPerms = new Set(MEMBER);
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const render = (channel: Channel, s = state()) => renderWithContext(
        <ChannelMenu
            channel={channel}
            point={{x: 1, y: 1}}
            onClose={jest.fn()}
        />,
        s as never,
    );
    const item = (name: string | RegExp) => screen.queryByRole('menuitem', {name});

    test('nobody leaves the default channel, and only admins archive it', () => {
        render(town);
        expect(item('Leave channel')).not.toBeInTheDocument();
        expect(item('Archive channel…')).not.toBeInTheDocument();
        expect(item(/Make it the default channel/)).not.toBeInTheDocument();
        expect(item('Convert to private channel…')).not.toBeInTheDocument();
    });

    test('a community admin may archive the default channel', () => {
        mockTeamPerms = new Set(ADMIN);
        render(town);
        expect(item('Archive channel…')).toBeInTheDocument();
        expect(item('Leave channel')).not.toBeInTheDocument();
    });

    test('Off-Topic can be left, but only admins archive it', () => {
        render(offTopic);
        expect(item('Leave channel')).toBeInTheDocument();
        expect(item('Archive channel…')).not.toBeInTheDocument();
    });

    test('an ordinary channel is left and archived by those allowed, and made default only by admins', () => {
        render(dev);
        expect(item('Leave channel')).toBeInTheDocument();
        expect(item('Archive channel…')).toBeInTheDocument();
        expect(item(/Make it the default channel/)).not.toBeInTheDocument();
    });

    test('without the permission, a member doesn\'t archive an ordinary channel', () => {
        mockChannelPerms = new Set();
        render(dev);
        expect(item('Archive channel…')).not.toBeInTheDocument();
    });

    test('an admin makes a channel the default one', async () => {
        mockTeamPerms = new Set(ADMIN);
        render(dev);
        await userEvent.click(screen.getByRole('menuitem', {name: /Make it the default channel/}));
        expect(patchTeam).toHaveBeenCalledWith({id: 't1', default_channel_id: 'dev'});
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('#dev is the default channel'));
    });

    test('once another channel is the default, Town Square can be left and that one can\'t', () => {
        render(town, state({defaultChannelId: 'dev'}));
        expect(item('Leave channel')).toBeInTheDocument();
    });

    test('the chosen default channel can\'t be left', () => {
        render(dev, state({defaultChannelId: 'dev'}));
        expect(item('Leave channel')).not.toBeInTheDocument();
        expect(item('Archive channel…')).not.toBeInTheDocument();
    });

    test('in an all-channels community, members mute public channels instead of leaving, and add nobody', async () => {
        render(dev, state({allChannels: true}));
        expect(item('Leave channel')).not.toBeInTheDocument();
        expect(item('Add members')).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('menuitem', {name: /Everyone in Lumen is in all its channels/}));
        expect(muteFor).toHaveBeenCalledWith('dev', 0);
    });

    test('in an all-channels community, invitation-only private channels are still left and joined by invitation', () => {
        render(secret, state({allChannels: true}));
        expect(item('Leave channel')).toBeInTheDocument();
        expect(item('Add members')).toBeInTheDocument();
        expect(item(/Everyone in Lumen is in all its channels/)).not.toBeInTheDocument();
    });

    test('elsewhere, public channels have Add members and Leave', () => {
        render(dev);
        expect(item('Add members')).toBeInTheDocument();
        expect(item(/Everyone in Lumen is in all its channels/)).not.toBeInTheDocument();
    });

    test('only community admins unarchive a channel', () => {
        const archived = ch('town', 'town-square', {delete_at: 3});
        render(archived, state({channels: [archived, dev]}));
        expect(item('Unarchive channel')).not.toBeInTheDocument();
    });

    test('a community admin unarchives an archived default channel', () => {
        mockTeamPerms = new Set(ADMIN);
        const archived = ch('town', 'town-square', {delete_at: 3});
        render(archived, state({channels: [archived, dev]}));
        expect(item('Unarchive channel')).toBeInTheDocument();
    });
});
