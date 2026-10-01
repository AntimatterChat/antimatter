// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {ChannelType} from '@mattermost/types/channels';
import type {DeepPartial} from '@mattermost/types/utilities';

import SidebarChannel from 'components/sidebar/sidebar_channel/sidebar_channel';

import {renderWithContext, screen} from 'tests/react_testing_utils';

import type {GlobalState} from 'types/store';
import type {SidebarChannelFooterComponent} from 'types/store/plugins';

jest.mock('components/tours/onboarding_tour', () => ({
    ChannelsAndDirectMessagesTour: () => null,
}));

jest.mock('components/sidebar/sidebar_channel/sidebar_direct_channel', () => () => <div>{'Direct Channel'}</div>);
jest.mock('components/sidebar/sidebar_channel/sidebar_group_channel', () => () => <div>{'Group Channel'}</div>);

describe('components/sidebar/sidebar_channel', () => {
    const baseProps = {
        channel: {
            id: 'channel_id',
            display_name: 'channel_display_name',
            create_at: 0,
            update_at: 0,
            delete_at: 0,
            team_id: '',
            type: 'O' as ChannelType,
            name: '',
            header: '',
            purpose: '',
            last_post_at: 0,
            last_root_post_at: 0,
            creator_id: '',
            scheme_id: '',
            group_constrained: false,
        },
        channelId: 'channel_id',
        isDraggable: false,
        channelIndex: 0,
        currentTeamName: 'team_name',
        unreadMentions: 0,
        isUnread: false,
        setChannelRef: jest.fn(),
        isCategoryCollapsed: false,
        isCurrentChannel: false,
        isAutoSortedCategory: false,
        isCategoryDragged: false,
        isDropDisabled: false,
        draggingState: {},
        multiSelectedChannelIds: [],
        autoSortedCategoryIds: new Set<string>(),
        isChannelSelected: false,
    };

    test('should match snapshot', () => {
        const {container} = renderWithContext(
            <SidebarChannel {...baseProps}/>,
        );

        expect(container).toMatchSnapshot();
    });

    test('should match snapshot when collapsed', () => {
        const props = {
            ...baseProps,
            isCategoryCollapsed: true,
        };

        const {container} = renderWithContext(
            <SidebarChannel {...props}/>,
        );

        expect(container).toMatchSnapshot();
    });

    test('should match snapshot when unread', () => {
        const props = {
            ...baseProps,
            isUnread: true,
            unreadMentions: 1,
        };

        const {container} = renderWithContext(
            <SidebarChannel {...props}/>,
        );

        expect(container).toMatchSnapshot();
    });

    test('should match snapshot when active', () => {
        const props = {
            ...baseProps,
            isCurrentChannel: true,
        };

        const {container} = renderWithContext(
            <SidebarChannel {...props}/>,
        );

        expect(container).toMatchSnapshot();
    });

    test('should match snapshot when DM channel', () => {
        const props = {
            ...baseProps,
            channel: {
                ...baseProps.channel,
                type: 'D' as ChannelType,
            },
        };

        const {container} = renderWithContext(
            <SidebarChannel {...props}/>,
        );

        expect(container).toMatchSnapshot();
    });

    test('should match snapshot when GM channel', () => {
        const props = {
            ...baseProps,
            channel: {
                ...baseProps.channel,
                type: 'G' as ChannelType,
            },
        };

        const {container} = renderWithContext(
            <SidebarChannel {...props}/>,
        );

        expect(container).toMatchSnapshot();
    });

    test('should not be collapsed when there are unread messages', () => {
        const props = {
            ...baseProps,
            isCategoryCollapsed: true,
            isUnread: true,
        };

        renderWithContext(
            <SidebarChannel {...props}/>,
        );

        expect(screen.getByRole('listitem')).toHaveClass('expanded');
    });

    describe('plugin footer', () => {
        const Footer: SidebarChannelFooterComponent['component'] = ({channel}) => <span>{`footer of ${channel.id}`}</span>;
        const pluginState: DeepPartial<GlobalState> = {
            plugins: {
                components: {
                    SidebarChannelFooter: [{id: 'footer', pluginId: 'plugin', component: Footer}],
                },
            },
        };

        test('should render plugin components below the channel link', () => {
            renderWithContext(
                <SidebarChannel {...baseProps}/>,
                pluginState,
            );

            expect(screen.getByText('footer of channel_id')).toBeInTheDocument();
        });

        test('should not render plugin components when collapsed', () => {
            renderWithContext(
                <SidebarChannel
                    {...baseProps}
                    isCategoryCollapsed={true}
                />,
                pluginState,
            );

            expect(screen.queryByText('footer of channel_id')).not.toBeInTheDocument();
        });
    });

    test('should not be collapsed if channel is current channel', () => {
        const props = {
            ...baseProps,
            isCategoryCollapsed: true,
            isCurrentChannel: true,
        };

        renderWithContext(
            <SidebarChannel {...props}/>,
        );

        expect(screen.getByRole('listitem')).toHaveClass('expanded');
    });
});
