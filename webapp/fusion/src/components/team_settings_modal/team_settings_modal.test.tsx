// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {patchTeam} from 'mattermost-redux/actions/teams';
import {Client4} from 'mattermost-redux/client';
import {Permissions} from 'mattermost-redux/constants';

import TeamSettingsModal from 'components/team_settings_modal/team_settings_modal';

import {openInvitePeople} from 'fusion/utils/modals';
import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {getHistory} from 'utils/browser_history';

// Mock Redux actions
jest.mock('mattermost-redux/actions/teams', () => ({
    patchTeam: jest.fn(() => async () => ({data: {}, error: null})),
    getTeam: jest.fn(() => async () => ({data: {}, error: null})),
    removeTeamIcon: jest.fn(() => async () => ({data: {}, error: null})),
    setTeamIcon: jest.fn(() => async () => ({data: {}, error: null})),
}));

// General loads the community's public channels.
jest.mock('mattermost-redux/actions/channels', () => ({
    ...jest.requireActual('mattermost-redux/actions/channels'),
    getChannels: jest.fn(() => ({type: 'MOCK_GET_CHANNELS'})),
}));

// The Members page shows the classic member list, which loads people from the server.
jest.mock('components/member_list_team', () => ({teamId}: {teamId: string}) => jest.requireActual('react').createElement('div', null, `Member list of ${teamId}`));
jest.mock('fusion/utils/modals', () => ({
    ...jest.requireActual('fusion/utils/modals'),
    openInvitePeople: jest.fn(() => ({type: 'MOCK_INVITE'})),
}));

describe('components/team_settings_modal', () => {
    const baseProps = {
        isOpen: true,
        onExited: jest.fn(),
    };

    beforeEach(() => {
        jest.clearAllMocks();

        // The Roles tab sets up the community's default role.
        jest.spyOn(Client4, 'ensureTeamDefaultRole').mockResolvedValue({id: 'default-role'} as never);

        // The Fusion dialog is drawn in the Fusion layer.
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const baseState = {
        entities: {
            teams: {
                currentTeamId: 'team-id',
                teams: {
                    'team-id': {
                        id: 'team-id',
                        display_name: 'Team Name',
                        description: 'Team Description',
                        name: 'team-name',
                    },
                },
                myMembers: {
                    'team-id': {
                        team_id: 'team-id',
                        user_id: 'user-id',
                        roles: 'team_user',
                    },
                },
            },
            roles: {
                roles: {
                    team_user: {
                        permissions: [Permissions.INVITE_USER, Permissions.MANAGE_TEAM],
                    },
                },
            },
            users: {
                currentUserId: 'user-id',
                profiles: {
                    'user-id': {
                        id: 'user-id',
                        roles: 'team_user',
                    },
                },
            },
        },
    };

    test('should hide the modal when the close button is clicked', async () => {
        renderWithContext(
            <TeamSettingsModal
                {...baseProps}
            />,
            baseState,
        );
        const modal = screen.getByRole('dialog', {name: 'Community settings'});
        expect(modal).toBeInTheDocument();
        const closeButton = screen.getByLabelText('Close the community settings');
        await userEvent.click(closeButton);

        await waitFor(() => {
            expect(baseProps.onExited).toHaveBeenCalled();
        });
    });

    test('should display access tab when can invite users', async () => {
        renderWithContext(
            <TeamSettingsModal
                {...baseProps}
            />,
            baseState,
        );
        const infoButton = screen.getByRole('tab', {name: 'General'});
        expect(infoButton).toBeDefined();
        const accessButton = screen.getByRole('tab', {name: 'Access'});
        expect(accessButton).toBeDefined();
    });

    test('should not display access tab when can not invite users', async () => {
        const stateWithoutPermission = {
            ...baseState,
            entities: {
                ...baseState.entities,
                roles: {
                    roles: {
                        team_user: {
                            permissions: [Permissions.MANAGE_TEAM],
                        },
                    },
                },
            },
        };

        renderWithContext(
            <TeamSettingsModal
                {...baseProps}
            />,
            stateWithoutPermission,
        );
        expect(screen.getAllByRole('tab').map((tab) => tab.textContent)).toEqual(['General', 'Members', 'Onboarding and rules', 'AI', 'Autolinks']);
    });

    test('should warn on first close attempt with unsaved changes and stay open', async () => {
        renderWithContext(
            <TeamSettingsModal
                {...baseProps}
            />,
            baseState,
        );

        const modal = screen.getByRole('dialog', {name: 'Community settings'});
        expect(modal).toBeInTheDocument();

        // Create unsaved changes by modifying team name
        const nameInput = screen.getByDisplayValue('Team Name');
        await userEvent.clear(nameInput);
        await userEvent.type(nameInput, 'Modified Team Name');

        // Attempt to close modal with unsaved changes
        const closeButton = screen.getByLabelText('Close the community settings');
        await userEvent.click(closeButton);

        // Verify modal remains open
        expect(modal).toBeInTheDocument();
        expect(modal).toBeVisible();

        // Verify warning message is displayed
        expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();

        // Verify close was prevented
        expect(baseProps.onExited).not.toHaveBeenCalled();
    });

    test('should allow close on second attempt with unsaved changes (warn-once behavior)', async () => {
        renderWithContext(
            <TeamSettingsModal
                {...baseProps}
            />,
            baseState,
        );

        // Create unsaved changes
        const nameInput = screen.getByDisplayValue('Team Name');
        await userEvent.clear(nameInput);
        await userEvent.type(nameInput, 'Modified Team Name');

        const closeButton = screen.getByLabelText('Close the community settings');

        // First close attempt triggers warning
        await userEvent.click(closeButton);

        // Verify warning is displayed
        expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();

        // Second close attempt closes modal
        await userEvent.click(closeButton);

        // Verify modal closes successfully
        await waitFor(() => {
            expect(baseProps.onExited).toHaveBeenCalled();
        });
    });

    test('should close modal normally when no unsaved changes', async () => {
        renderWithContext(
            <TeamSettingsModal
                {...baseProps}
            />,
            baseState,
        );

        const modal = screen.getByRole('dialog', {name: 'Community settings'});
        expect(modal).toBeInTheDocument();

        // Close modal with no unsaved changes
        const closeButton = screen.getByLabelText('Close the community settings');
        await userEvent.click(closeButton);

        // Verify modal closes normally
        await waitFor(() => {
            expect(baseProps.onExited).toHaveBeenCalled();
        });
    });

    test('should reset warning state when changes are saved', async () => {
        renderWithContext(
            <TeamSettingsModal
                {...baseProps}
            />,
            baseState,
        );

        // Create unsaved changes
        const nameInput = screen.getByDisplayValue('Team Name');
        await userEvent.clear(nameInput);
        await userEvent.type(nameInput, 'Modified Team Name');

        // Save changes immediately (without triggering warning first)
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        await waitFor(() => expect(patchTeam).toHaveBeenCalledWith({id: 'team-id', display_name: 'Modified Team Name', description: 'Team Description'}));

        // After saving, close modal - should work immediately (single click)
        const closeButton = screen.getByLabelText('Close the community settings');
        await userEvent.click(closeButton);

        // Verify modal closes successfully without warning
        await waitFor(() => {
            expect(baseProps.onExited).toHaveBeenCalled();
        });
    });

    // Antimatter: General holds what members do with the community's channels and categories: its kind, the
    // channels everyone is in, where people land, the join and leave messages, and its layout.
    test('General has the community, its channels and its layout', () => {
        renderWithContext(<TeamSettingsModal {...baseProps}/>, withPermissions([Permissions.MANAGE_TEAM]));
        expect(screen.getByRole('heading', {name: 'General'})).toBeInTheDocument();
        expect(screen.getByRole('textbox', {name: 'Name'})).toHaveValue('Team Name');
        expect(screen.getByRole('group', {name: 'Kind of community'})).toBeInTheDocument();
        expect(screen.getByRole('switch', {name: 'Members are in every channel'})).toBeInTheDocument();
        expect(screen.getByRole('combobox', {name: 'Default channel'})).toBeInTheDocument();
        expect(screen.getByRole('combobox', {name: 'Join and leave messages'})).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Export as a template'})).toBeInTheDocument();
        expect(screen.queryByRole('tab', {name: 'Channels'})).not.toBeInTheDocument();
        expect(screen.queryByRole('tab', {name: 'Community layout'})).not.toBeInTheDocument();
    });

    test('the former tabs open where they went', () => {
        for (const [tab, now] of [['channels', 'General'], ['layout', 'General'], ['rules', 'Onboarding and rules'], ['agents', 'AI']]) {
            const {unmount} = renderWithContext(
                <TeamSettingsModal
                    {...baseProps}
                    initialTab={tab}
                />,
                withPermissions([Permissions.MANAGE_TEAM]),
            );
            expect(screen.getByRole('tab', {name: now})).toHaveAttribute('aria-selected', 'true');
            unmount();
        }
    });

    test('shows the Autolinks tab to those who manage the team', async () => {
        const state = {
            ...baseState,
            entities: {...baseState.entities, roles: {roles: {team_user: {permissions: [Permissions.INVITE_USER, Permissions.MANAGE_TEAM]}}}},
        };
        renderWithContext(<TeamSettingsModal {...baseProps}/>, state);
        await userEvent.click(screen.getByRole('tab', {name: 'Autolinks'}));
        expect(screen.getByRole('button', {name: 'Add an autolink'})).toBeInTheDocument();
    });

    test('hides the Autolinks tab from the others', () => {
        renderWithContext(<TeamSettingsModal {...baseProps}/>, {...baseState, entities: {...baseState.entities, roles: {roles: {team_user: {permissions: [Permissions.MANAGE_TEAM_ROLES]}}}}});
        expect(screen.queryByRole('tab', {name: 'Autolinks'})).not.toBeInTheDocument();
    });

    // Antimatter: the community's rules, which its members agree to before they post, are for its admins to write.
    test('shows the Rules tab to those who manage the community', async () => {
        const rules = jest.spyOn(Client4, 'getCommunityRules').mockResolvedValue({team_id: 'team-id', enabled: true, text: 'Be kind.', version: 1, updated_by: 'admin', update_at: 1, accepted_version: 0, must_accept: false});
        const stats = jest.spyOn(Client4, 'getCommunityRulesStats').mockResolvedValue({team_id: 'team-id', version: 1, accepted_count: 3, member_count: 5});
        const state = {
            ...baseState,
            entities: {...baseState.entities, roles: {roles: {team_user: {permissions: [Permissions.MANAGE_TEAM]}}}},
        };
        renderWithContext(<TeamSettingsModal {...baseProps}/>, state);
        await userEvent.click(screen.getByRole('tab', {name: 'Onboarding and rules'}));
        expect(await screen.findByRole('switch', {name: 'Members must agree to the rules'})).toHaveAttribute('aria-checked', 'true');
        expect(await screen.findByText('3 of 5 members agreed to the current rules.')).toBeInTheDocument();
        rules.mockRestore();
        stats.mockRestore();
    });

    test('hides the Rules tab from the others', () => {
        // Members who don't manage the community (the base state's user does).
        renderWithContext(<TeamSettingsModal {...baseProps}/>, withPermissions([Permissions.INVITE_USER]));
        expect(screen.queryByRole('tab', {name: 'Onboarding and rules'})).not.toBeInTheDocument();
    });

    // Antimatter: the community's roles, onboarding, attributes and layout are tabs of its settings, which the
    // community menu opens on the right tab.
    const withPermissions = (permissions: string[]) => ({
        ...baseState,
        entities: {
            ...baseState.entities,
            general: {config: {EnableCustomGroups: 'true'}},
            roles: {roles: {team_user: {permissions}}},
        },
    });

    test('shows the roles, onboarding, attributes and AI tabs to the community\'s admins', () => {
        renderWithContext(<TeamSettingsModal {...baseProps}/>, withPermissions([Permissions.MANAGE_TEAM, Permissions.MANAGE_TEAM_ROLES]));
        for (const name of ['General', 'Roles', 'Onboarding and rules', 'Member attributes', 'AI', 'Autolinks']) {
            expect(screen.getByRole('tab', {name})).toBeInTheDocument();
        }
    });

    test('opens on the tab asked for', () => {
        renderWithContext(
            <TeamSettingsModal
                {...baseProps}
                initialTab='onboarding'
            />,
            withPermissions([Permissions.MANAGE_TEAM, Permissions.MANAGE_TEAM_ROLES]),
        );
        expect(screen.getByRole('tab', {name: 'Onboarding and rules'})).toHaveAttribute('aria-selected', 'true');
        expect(screen.getByRole('heading', {name: 'Onboarding and rules'})).toBeInTheDocument();

        // The rules first, then the onboarding: one panel, each with its own Save.
        const sections = screen.getAllByRole('heading', {level: 3}).map((h) => h.textContent);
        expect(sections).toEqual(['Rules', 'Onboarding']);
        expect(screen.getAllByRole('button', {name: 'Save'}).length).toBeGreaterThanOrEqual(1);
    });

    test('those who only manage the roles get the members, the teams and the roles', () => {
        renderWithContext(
            <TeamSettingsModal
                {...baseProps}
                initialTab='roles'
            />,
            withPermissions([Permissions.MANAGE_TEAM_ROLES, Permissions.INVITE_USER]),
        );

        // Inviting without managing the community isn't changing its invitations (Access).
        expect(screen.getAllByRole('tab').map((tab) => tab.textContent)).toEqual(['Roles', 'Members', 'Teams']);
        expect(screen.getByRole('tab', {name: 'Roles'})).toHaveAttribute('aria-selected', 'true');
    });

    test('a tab one may not see opens the first one instead', () => {
        renderWithContext(
            <TeamSettingsModal
                {...baseProps}
                initialTab='teams'
            />,
            baseState,
        );
        expect(screen.getByRole('tab', {name: 'General'})).toHaveAttribute('aria-selected', 'true');
    });

    test('Close in the Roles tab closes the settings', async () => {
        renderWithContext(
            <TeamSettingsModal
                {...baseProps}
                initialTab='roles'
            />,
            withPermissions([Permissions.MANAGE_TEAM, Permissions.MANAGE_TEAM_ROLES]),
        );
        await userEvent.click(screen.getByRole('button', {name: 'Close'}));
        await waitFor(() => {
            expect(baseProps.onExited).toHaveBeenCalled();
        });
    });

    // Antimatter: a community's own model connections, for those who may manage them (its admins, and the roles
    // granted it).
    // AI: the model connections and the agents in one tab.
    describe('AI tab', () => {
        let list: jest.SpyInstance;
        let agents: jest.SpyInstance;
        beforeEach(() => {
            list = jest.spyOn(Client4, 'getAIConnections').mockResolvedValue([]);
            agents = jest.spyOn(Client4, 'getAIAgents').mockResolvedValue([] as never);
        });
        afterEach(() => {
            list.mockRestore();
            agents.mockRestore();
        });

        test('the community\'s admins without the permission get its agents only', async () => {
            renderWithContext(
                <TeamSettingsModal
                    {...baseProps}
                    initialTab='ai'
                />,
                withPermissions([Permissions.MANAGE_TEAM, Permissions.MANAGE_TEAM_ROLES]),
            );
            expect(screen.getByRole('tab', {name: 'AI'})).toHaveAttribute('aria-selected', 'true');
            expect(screen.getByRole('heading', {name: 'Agents'})).toBeInTheDocument();
            expect(screen.queryByRole('heading', {name: 'Model connections'})).not.toBeInTheDocument();
            await waitFor(() => expect(agents).toHaveBeenCalled());
            expect(list).not.toHaveBeenCalled();
        });

        test('the former Model connections tab opens the AI tab', () => {
            renderWithContext(
                <TeamSettingsModal
                    {...baseProps}
                    initialTab='ai_connections'
                />,
                baseState,
            );
            expect(screen.getByRole('tab', {name: 'AI'})).toHaveAttribute('aria-selected', 'true');
            expect(list).not.toHaveBeenCalled();
        });

        test('lists the community\'s own connections, then its agents, for those who may manage them', async () => {
            renderWithContext(
                <TeamSettingsModal
                    {...baseProps}
                    initialTab='ai_connections'
                />,
                withPermissions([Permissions.MANAGE_TEAM, Permissions.MANAGE_TEAM_AI_CONNECTIONS]),
            );
            expect(screen.getByRole('tab', {name: 'AI'})).toHaveAttribute('aria-selected', 'true');
            expect(screen.getAllByRole('heading', {level: 3}).map((h) => h.textContent)).toEqual(['Model connections', 'Agents']);
            expect(await screen.findByText('No model connections yet.')).toBeInTheDocument();
            expect(list).toHaveBeenCalledWith('team-id');
        });

        test('those who only manage the model connections get that tab alone', async () => {
            renderWithContext(
                <TeamSettingsModal
                    {...baseProps}
                    initialTab='ai_connections'
                />,
                withPermissions([Permissions.MANAGE_TEAM_AI_CONNECTIONS]),
            );
            expect(screen.getAllByRole('tab').map((tab) => tab.textContent)).toEqual(['Members', 'Teams', 'AI']);
            expect(agents).not.toHaveBeenCalled();
            expect(await screen.findByRole('button', {name: 'Add a connection'})).toBeInTheDocument();
        });
    });

    // Antimatter: the community menu's Manage members, Teams… and the user menu's Integrations are in the settings.
    describe('members, teams and integrations', () => {
        const admin = [Permissions.MANAGE_TEAM, Permissions.MANAGE_TEAM_ROLES, Permissions.INVITE_USER, Permissions.ADD_USER_TO_TEAM];

        test('the admins\' tabs, in order', () => {
            renderWithContext(<TeamSettingsModal {...baseProps}/>, withPermissions(admin));
            expect(screen.getAllByRole('tab').map((tab) => tab.textContent)).toEqual([
                'General', 'Roles', 'Members', 'Access', 'Member attributes', 'Teams', 'Onboarding and rules', 'AI', 'Autolinks',
            ]);
        });

        test('the Members page lists the community\'s members, and invites more for those who may', async () => {
            renderWithContext(
                <TeamSettingsModal
                    {...baseProps}
                    initialTab='members'
                />,
                withPermissions(admin),
            );
            expect(screen.getByRole('heading', {name: 'Members'})).toBeInTheDocument();
            expect(screen.getByText('Member list of team-id')).toBeInTheDocument();

            await userEvent.click(screen.getByRole('button', {name: 'Invite people'}));
            expect(openInvitePeople).toHaveBeenCalled();
            await waitFor(() => expect(baseProps.onExited).toHaveBeenCalled());
        });

        test('no Invite people on the Members page for those who can\'t add people', () => {
            renderWithContext(
                <TeamSettingsModal
                    {...baseProps}
                    initialTab='members'
                />,
                withPermissions([Permissions.MANAGE_TEAM_ROLES]),
            );
            expect(screen.getByText('Member list of team-id')).toBeInTheDocument();
            expect(screen.queryByRole('button', {name: 'Invite people'})).not.toBeInTheDocument();
        });

        test('the Teams page lists the community\'s teams, and creates them in place', async () => {
            const state = withPermissions(admin) as typeof baseState & {entities: Record<string, unknown>};
            const group = (id: string, name: string, teamId: string) => ({id, name: id, display_name: name, tag: name.slice(0, 3).toUpperCase(), source: 'custom', is_team: true, team_id: teamId, delete_at: 0, member_ids: ['user-id']});
            (state.entities as Record<string, unknown>).groups = {groups: {g1: group('g1', 'Design', 'team-id'), g2: group('g2', 'Ops', 'other-team')}};
            renderWithContext(
                <TeamSettingsModal
                    {...baseProps}
                    initialTab='teams'
                />,
                state,
            );
            expect(screen.getByRole('heading', {name: 'Teams'})).toBeInTheDocument();
            expect(screen.getByRole('button', {name: /Design/})).toBeInTheDocument();
            expect(screen.queryByRole('button', {name: /Ops/})).not.toBeInTheDocument();
            expect(screen.getByRole('switch', {name: 'Everyone shows a team badge'})).toBeInTheDocument();

            // No separate dialog: the editor opens under the list.
            await userEvent.click(screen.getByRole('button', {name: 'Create team'}));
            expect(screen.getByRole('textbox', {name: /Mention/})).toBeInTheDocument();
            expect(baseProps.onExited).not.toHaveBeenCalled();
        });

        test('no Teams page when the server has no teams', () => {
            renderWithContext(<TeamSettingsModal {...baseProps}/>, baseState);
            expect(screen.queryByRole('tab', {name: 'Teams'})).not.toBeInTheDocument();
            expect(screen.getByRole('tab', {name: 'Members'})).toBeInTheDocument();
        });

        test('Integrations leads to the community\'s integrations pages, for those who manage some', async () => {
            const push = jest.spyOn(getHistory(), 'push').mockImplementation(() => undefined);
            try {
                renderWithContext(<TeamSettingsModal {...baseProps}/>, withPermissions([Permissions.MANAGE_OWN_INCOMING_WEBHOOKS]));

                // Its own pages, not a tab: those who only manage integrations get the members and teams as tabs.
                expect(screen.queryByRole('tab', {name: 'Integrations'})).not.toBeInTheDocument();
                expect(screen.getAllByRole('tab').map((tab) => tab.textContent)).toEqual(['Members', 'Teams']);
                await userEvent.click(screen.getByRole('button', {name: 'Integrations'}));
                expect(push).toHaveBeenCalledWith('/team-name/integrations');
                await waitFor(() => expect(baseProps.onExited).toHaveBeenCalled());
            } finally {
                push.mockRestore();
            }
        });

        test('no Integrations for the community\'s admins without integration rights', () => {
            renderWithContext(<TeamSettingsModal {...baseProps}/>, withPermissions(admin));
            expect(screen.queryByRole('button', {name: 'Integrations'})).not.toBeInTheDocument();
        });
    });
});
