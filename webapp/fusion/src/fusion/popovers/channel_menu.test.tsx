// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import fs from 'fs';
import path from 'path';

import React from 'react';
import {createIntl} from 'react-intl';
import type {IntlShape} from 'react-intl';

import type {ChannelCategory} from '@mattermost/types/channel_categories';
import type {Channel} from '@mattermost/types/channels';
import type {TeamKind} from '@mattermost/types/teams';

import {addChannelToCategory} from 'mattermost-redux/actions/channel_categories';
import {favoriteChannel} from 'mattermost-redux/actions/channels';
import {patchTeam} from 'mattermost-redux/actions/teams';

import {muteFor} from 'fusion/sidebar/timed_mute';
import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import ChannelMenu from './channel_menu';

// The tests mock every i18n/*.json import: read the real French catalog.
const frMessages: Record<string, string> = JSON.parse(fs.readFileSync(path.join(__dirname, '..', '..', 'i18n', 'fr.json'), 'utf8'));

// The tests' useIntl is English (tests/react-intl_mock.ts): speak French for these.
const intlModule: {useIntl: () => IntlShape} = jest.requireMock('react-intl');
const speakFrench = () => jest.spyOn(intlModule, 'useIntl').mockReturnValue(createIntl({locale: 'fr', messages: frMessages, defaultLocale: 'en', timeZone: 'Etc/UTC', textComponent: 'span'}));

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
let mockCategories: ChannelCategory[] = [];
jest.mock('selectors/views/channel_sidebar', () => ({getCategoriesForCurrentTeam: () => mockCategories}));
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
jest.mock('mattermost-redux/actions/channel_categories', () => ({
    ...jest.requireActual('mattermost-redux/actions/channel_categories'),
    addChannelToCategory: jest.fn(() => ({type: 'MOCK_ADD_TO_CATEGORY'})),
}));
jest.mock('mattermost-redux/actions/channels', () => ({
    ...jest.requireActual('mattermost-redux/actions/channels'),
    favoriteChannel: jest.fn(() => ({type: 'MOCK_FAVORITE'})),
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
const town = ch('town', 'general');
const offTopic = ch('off', 'off-topic');
const dev = ch('dev', 'dev');
const secret = ch('secret', 'secret', {type: 'P'});
const all = [town, offTopic, dev, secret];

function state({allChannels = false, defaultChannelId = '', channels = all, kind = 'community' as TeamKind} = {}) {
    return {
        entities: {
            general: {config: {}},
            users: {currentUserId: 'me', profiles: {me: TestHelper.getUserMock({id: 'me', roles: 'system_user', delete_at: 0})}},
            teams: {
                currentTeamId: 't1',
                teams: {t1: TestHelper.getTeamMock({id: 't1', name: 'lumen', display_name: 'Lumen', all_channels_membership: allChannels, default_channel_id: defaultChannelId, kind})},
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

    // Antimatter: in a workplace community, the channel's admins (whoever created it) archive it; in a community, not
    // without the permission.
    test('in a workplace community, the channel\'s admins archive it', () => {
        mockChannelPerms = new Set(['manage_channel_roles']);
        render(dev, state({kind: 'workplace'}));
        expect(item('Archive channel…')).toBeInTheDocument();
    });

    test('in a community, the channel\'s admins don\'t archive it without the permission', () => {
        mockChannelPerms = new Set(['manage_channel_roles']);
        render(dev);
        expect(item('Archive channel…')).not.toBeInTheDocument();
    });

    test('an archived channel offers nothing to change, but takes members', () => {
        mockTeamPerms = new Set(ADMIN);
        mockChannelPerms = new Set(['manage_channel_roles']);
        const {unmount} = render(dev);
        expect(item(/Permissions…/)).toBeInTheDocument();
        unmount();

        render(ch('old', 'old', {delete_at: 5}));
        expect(item(/Permissions…/)).not.toBeInTheDocument();
        expect(item('Channel settings')).not.toBeInTheDocument();

        // It still takes members, to read it.
        expect(item('Add members')).toBeInTheDocument();
    });

    test('an admin makes a channel the default one', async () => {
        mockTeamPerms = new Set(ADMIN);
        render(dev);
        await userEvent.click(screen.getByRole('menuitem', {name: /Make it the default channel/}));
        expect(patchTeam).toHaveBeenCalledWith({id: 't1', default_channel_id: 'dev'});
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('#dev is the default channel'));
    });

    test('once another channel is the default, General can be left and that one can\'t', () => {
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
        const archived = ch('town', 'general', {delete_at: 3});
        render(archived, state({channels: [archived, dev]}));
        expect(item('Unarchive channel')).not.toBeInTheDocument();
    });

    test('a community admin unarchives an archived default channel', () => {
        mockTeamPerms = new Set(ADMIN);
        const archived = ch('town', 'general', {delete_at: 3});
        render(archived, state({channels: [archived, dev]}));
        expect(item('Unarchive channel')).toBeInTheDocument();
    });

    // Antimatter's English interface spells it the British way.
    test('Favourite adds the channel to Favourites, spelt the British way', async () => {
        render(dev);
        expect(item('Favorite')).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('menuitem', {name: 'Favourite'}));
        expect(favoriteChannel).toHaveBeenCalledWith('dev');
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('Added to Favourites'));
    });
});

describe('fusion/popovers/ChannelMenu: the categories to move to, in the user\'s language', () => {
    const fr = frMessages;
    const category = (id: string, type: ChannelCategory['type'], displayName: string) => ({id, team_id: 't1', user_id: 'me', type, display_name: displayName, channel_ids: [], sorting: 'default', muted: false, collapsed: false}) as unknown as ChannelCategory;

    beforeEach(() => {
        mockTeamPerms = new Set();
        mockChannelPerms = new Set(MEMBER);
        mockCategories = [
            category('favorites_me_t1', 'favorites', 'Favourites'),
            category('channels_me_t1', 'channels', 'Channels'),
            category('custom1', 'custom', 'Work'),
            category('direct_messages_me_t1', 'direct_messages', 'Direct Messages'),
        ];
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });
    let spy: jest.SpyInstance;
    beforeEach(() => {
        spy = speakFrench();
    });
    afterEach(() => {
        spy.mockRestore();
        mockCategories = [];
    });

    test('in French, the default categories have their French names and a custom one keeps its own', async () => {
        renderWithContext(
            <ChannelMenu
                channel={dev}
                point={{x: 1, y: 1}}
                onClose={jest.fn()}
            />,
            state() as never,
            {locale: 'fr', intlMessages: fr},
        );

        await userEvent.click(screen.getByRole('menuitem', {name: fr['fusion.channelMenu.move']}));
        expect(screen.getByRole('menuitemradio', {name: fr['sidebar_left.sidebar_channel_menu.favorites']})).toBeInTheDocument();
        expect(screen.getByRole('menuitemradio', {name: fr['sidebar_left.sidebar_channel_menu.channels']})).toBeInTheDocument();
        expect(screen.getByRole('menuitemradio', {name: 'Work'})).toBeInTheDocument();
        expect(screen.queryByRole('menuitemradio', {name: 'Favourites'})).not.toBeInTheDocument();
        expect(fr['sidebar_left.sidebar_channel_menu.favorites']).toBe('Favoris');

        await userEvent.click(screen.getByRole('menuitemradio', {name: 'Favoris'}));
        expect(addChannelToCategory).toHaveBeenCalledWith('favorites_me_t1', 'dev');
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('Déplacé vers Favoris'));
    });
});
