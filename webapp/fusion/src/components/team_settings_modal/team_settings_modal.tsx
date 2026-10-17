// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState, useRef, useCallback, useEffect} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector} from 'react-redux';

import {getCurrentTeam, getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';

import TeamSettings from 'components/team_settings';

import ModelConnections from 'fusion/ai/model_connections';
import CommunityAttributesSettings from 'fusion/attributes/community_attributes_dialog';
import Icon from 'fusion/components/icon';
import {Dialog} from 'fusion/components/layer';
import AutolinksSettings from 'fusion/teams/autolinks_settings';
import ChannelsSettings from 'fusion/teams/channels_settings';
import {getCommunitySettingsAccess} from 'fusion/teams/community_settings_access';
import MembersSettings from 'fusion/teams/members_settings';
import OnboardingSettings from 'fusion/teams/onboarding_editor';
import RolesSettings from 'fusion/teams/roles_dialog';
import {TeamLayoutSettings} from 'fusion/teams/team_layout_dialog';
import TeamsSettings from 'fusion/teams/teams_settings';
import {am} from 'fusion/utils/class_names';
import {openInvitePeople, openUserGroups} from 'fusion/utils/modals';
import {focusElement} from 'utils/a11y_utils';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

import './team_settings_modal.scss';

const SHOW_PANEL_ERROR_STATE_TAB_SWITCH_TIMEOUT = 3000;

// The tabs the Fusion UI draws itself; the others are the classic web app's.
export type FusionTeamSettingsTab = 'members' | 'teams' | 'channels' | 'roles' | 'onboarding' | 'layout' | 'attributes' | 'autolinks' | 'ai_connections';
const FUSION_TABS = new Set<string>(['members', 'teams', 'channels', 'roles', 'onboarding', 'layout', 'attributes', 'autolinks', 'ai_connections']);

type Props = {
    isOpen: boolean;
    onExited: () => void;
    focusOriginElement?: string;

    // The tab to open on, when its opener may see it (Antimatter: the community menu's entries).
    initialTab?: string;
};

const TeamSettingsModal = ({isOpen, onExited, focusOriginElement, initialTab}: Props) => {
    const [activeTab, setActiveTab] = useState(initialTab || 'info');
    const [show, setShow] = useState(isOpen);
    const [areThereUnsavedChanges, setAreThereUnsavedChanges] = useState(false);
    const [showTabSwitchError, setShowTabSwitchError] = useState(false);
    const [hasBeenWarned, setHasBeenWarned] = useState(false);
    const modalBodyRef = useRef<HTMLDivElement>(null);
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();

    const teamId = useSelector(getCurrentTeamId);
    const team = useSelector(getCurrentTeam);

    // Antimatter: what one may manage, page by page, as the community menu decides who sees the settings
    // (fusion/teams/community_settings_access.ts).
    const access = useSelector((state: GlobalState) => getCommunitySettingsAccess(state, teamId), shallowEqual);

    useEffect(() => {
        setShow(isOpen);
    }, [isOpen]);

    const updateTab = useCallback((tab: string) => {
        if (areThereUnsavedChanges) {
            setShowTabSwitchError(true);
            setTimeout(() => {
                setShowTabSwitchError(false);
            }, SHOW_PANEL_ERROR_STATE_TAB_SWITCH_TIMEOUT);
            return;
        }
        setActiveTab(tab);

        if (modalBodyRef.current) {
            modalBodyRef.current.scrollTop = 0;
        }
    }, [areThereUnsavedChanges]);

    const handleHide = useCallback(() => {
        // Prevent modal closing if there are unsaved changes (warn once, then allow)
        if (areThereUnsavedChanges && !hasBeenWarned) {
            setHasBeenWarned(true);
            setShowTabSwitchError(true);
            setTimeout(() => {
                setShowTabSwitchError(false);
            }, SHOW_PANEL_ERROR_STATE_TAB_SWITCH_TIMEOUT);
        } else {
            handleHideConfirm();
        }
    }, [areThereUnsavedChanges, hasBeenWarned]);

    const handleHideConfirm = useCallback(() => {
        setShow(false);
    }, []);

    // Leaves the settings for a dialog or page of their own (inviting people, every team, the integrations): not with
    // unsaved changes, which warn as switching tabs does.
    const leaveFor = (go: () => void) => {
        if (areThereUnsavedChanges) {
            setShowTabSwitchError(true);
            setTimeout(() => {
                setShowTabSwitchError(false);
            }, SHOW_PANEL_ERROR_STATE_TAB_SWITCH_TIMEOUT);
            return;
        }
        setShow(false);
        go();
    };

    const handleExited = useCallback(() => {
        // Reset all state
        setActiveTab(initialTab || 'info');
        setAreThereUnsavedChanges(false);
        setShowTabSwitchError(false);
        setHasBeenWarned(false);

        // Restore focus
        if (focusOriginElement) {
            focusElement(focusOriginElement, true);
        }

        // Notify parent
        onExited();
    }, [onExited, focusOriginElement, initialTab]);

    const tabs = [
        {
            name: 'info',
            uiName: formatMessage({id: 'fusion.teamSettings.overview', defaultMessage: 'Overview'}),
            icon: 'icon icon-information-outline',
            iconTitle: formatMessage({id: 'generic_icons.info', defaultMessage: 'Info Icon'}),
            display: access.manage,
        },
        {
            name: 'access',
            uiName: formatMessage({id: 'team_settings_modal.accessTab', defaultMessage: 'Access'}),
            icon: 'icon icon-account-multiple-outline',
            iconTitle: formatMessage({id: 'generic_icons.member', defaultMessage: 'Member Icon'}),

            // The community's invitations: changing them takes managing it (the server checks manage_team).
            display: access.access,
        },
        {
            name: 'team_membership',
            uiName: formatMessage({id: 'team_settings_modal.teamMembershipTab', defaultMessage: 'Team Membership'}),
            icon: 'icon icon-iframe-list-outline',
            iconTitle: formatMessage({id: 'generic_icons.team_membership', defaultMessage: 'Team Membership Icon'}),
            display: access.teamMembership,
        },
        {
            name: 'access_policies',
            uiName: formatMessage({id: 'team_settings_modal.accessPoliciesTab', defaultMessage: 'Channel Membership'}),
            icon: 'icon icon-message-check-outline',
            iconTitle: formatMessage({id: 'generic_icons.access_rules', defaultMessage: 'Membership Policy Icon'}),
            display: access.accessPolicies,
        },
        {
            // Antimatter: the members (the menu's former Manage members), for everyone who opens the settings.
            name: 'members',
            uiName: formatMessage({id: 'fusion.teamSettings.members', defaultMessage: 'Members'}),
            icon: 'icon icon-account-multiple-outline',
            iconTitle: formatMessage({id: 'fusion.teamSettings.members', defaultMessage: 'Members'}),
            display: true,
        },
        {
            // And its teams (the menu's former Teams…), when the server has them.
            name: 'teams',
            uiName: formatMessage({id: 'fusion.teamSettings.teams', defaultMessage: 'Teams'}),
            icon: 'icon icon-account-group-outline',
            iconTitle: formatMessage({id: 'fusion.teamSettings.teams', defaultMessage: 'Teams'}),
            display: access.teams,
        },
        {
            name: 'channels',
            uiName: formatMessage({id: 'fusion.teamSettings.channels', defaultMessage: 'Channels'}),
            icon: 'icon icon-globe',
            iconTitle: formatMessage({id: 'fusion.teamSettings.channels', defaultMessage: 'Channels'}),
            display: access.manage,
        },
        {
            name: 'roles',
            uiName: formatMessage({id: 'fusion.roles.title', defaultMessage: 'Roles'}),
            icon: 'icon icon-shield-outline',
            iconTitle: formatMessage({id: 'fusion.roles.title', defaultMessage: 'Roles'}),
            display: access.roles,
        },
        {
            name: 'onboarding',
            uiName: formatMessage({id: 'fusion.onboarding.editTitle', defaultMessage: 'Onboarding'}),
            icon: 'icon icon-message-text-outline',
            iconTitle: formatMessage({id: 'fusion.onboarding.editTitle', defaultMessage: 'Onboarding'}),
            display: access.manage && access.teams,
        },
        {
            name: 'attributes',
            uiName: formatMessage({id: 'fusion.communityAttributes.title', defaultMessage: 'Member attributes'}),
            icon: 'icon icon-tag-outline',
            iconTitle: formatMessage({id: 'fusion.communityAttributes.title', defaultMessage: 'Member attributes'}),
            display: access.manage && access.teams,
        },
        {
            name: 'layout',
            uiName: formatMessage({id: 'fusion.layout.title', defaultMessage: 'Community layout'}),
            icon: 'icon icon-view-grid-outline',
            iconTitle: formatMessage({id: 'fusion.layout.title', defaultMessage: 'Community layout'}),
            display: access.manage && access.teams,
        },
        {
            name: 'autolinks',
            uiName: formatMessage({id: 'fusion.teamSettings.autolinks', defaultMessage: 'Autolinks'}),
            icon: 'icon icon-link-variant',
            iconTitle: formatMessage({id: 'fusion.teamSettings.autolinks', defaultMessage: 'Autolinks'}),
            display: access.manage,
        },
        {
            name: 'ai_connections',
            uiName: formatMessage({id: 'fusion.teamSettings.aiConnections', defaultMessage: 'Model connections'}),
            icon: 'icon icon-creation-outline',
            iconTitle: formatMessage({id: 'fusion.teamSettings.aiConnections', defaultMessage: 'Model connections'}),

            // Antimatter: the community's own model connections, for its admins (and the roles granted it).
            display: access.aiConnections,
        },
    ];

    // Closing the Fusion dialog has no fade-out to wait for.
    useEffect(() => {
        if (!show) {
            handleExited();
        }
    }, [show]); // eslint-disable-line react-hooks/exhaustive-deps

    if (!show) {
        return null;
    }

    const modalTitle = formatMessage({id: 'team_settings_modal.title', defaultMessage: 'Team Settings'});
    const visibleTabs = tabs.filter((tab) => tab.display !== false);

    // A tab one may not see (asked for by an opener, or no longer allowed) falls back to the first one.
    const shownTab = visibleTabs.some((tab) => tab.name === activeTab) ? activeTab : visibleTabs[0]?.name;

    // The Fusion UI shows the team settings in its own dialog, as the channel settings: the tabs on the left under the
    // team's name, the tab's name as the heading of the pane.
    return (
        <Dialog
            label={modalTitle}
            small={false}
            className={am('team-settings')}
            onClose={handleHide}
        >
            <nav
                role='tablist'
                aria-orientation='vertical'
            >
                <h5 title={team?.display_name}>{team?.display_name}</h5>
                {visibleTabs.map((tab) => (
                    <button
                        key={tab.name}
                        type='button'
                        role='tab'
                        className={am({on: tab.name === shownTab})}
                        aria-selected={tab.name === shownTab}
                        onClick={() => updateTab(tab.name)}
                    >
                        {tab.uiName}
                    </button>
                ))}

                {/* Antimatter: the community's integrations have pages of their own (the menu's former link in the
                    user menu): this leaves the settings for them. */}
                {access.integrations && team && (
                    <>
                        <hr/>
                        <button
                            type='button'
                            className={am('ts-link')}
                            onClick={() => leaveFor(() => getHistory().push(`/${team.name}/integrations`))}
                        >
                            {formatMessage({id: 'fusion.teamSettings.integrations', defaultMessage: 'Integrations'})}
                            <Icon
                                name='popout'
                                size='xs'
                            />
                        </button>
                    </>
                )}
            </nav>
            <div
                ref={modalBodyRef}
                className={am('pane')}
                role='tabpanel'
            >
                <h2>{visibleTabs.find((tab) => tab.name === shownTab)?.uiName}</h2>

                {/* The Fusion tabs have their own Save and Cancel (or Close); the classic ones, the bar that saves. */}
                {showTabSwitchError && FUSION_TABS.has(shownTab || '') && (
                    <p
                        className={am('field-error')}
                        role='alert'
                    >
                        {formatMessage({id: 'saveChangesPanel.message', defaultMessage: 'You have unsaved changes'})}
                    </p>
                )}
                {shownTab === 'members' && <MembersSettings onInvite={() => leaveFor(() => dispatch(openInvitePeople()))}/>}
                {shownTab === 'teams' && <TeamsSettings onOpenAll={() => leaveFor(() => dispatch(openUserGroups(teamId)))}/>}
                {shownTab === 'channels' && <ChannelsSettings/>}
                {shownTab === 'roles' && <RolesSettings onClose={handleHide}/>}
                {shownTab === 'onboarding' && <OnboardingSettings setUnsaved={setAreThereUnsavedChanges}/>}
                {shownTab === 'attributes' && (
                    <CommunityAttributesSettings
                        teamId={teamId}
                        onClose={handleHide}
                    />
                )}
                {shownTab === 'layout' && <TeamLayoutSettings onClose={handleHide}/>}
                {shownTab === 'autolinks' && <AutolinksSettings setUnsaved={setAreThereUnsavedChanges}/>}
                {shownTab === 'ai_connections' && teamId && (
                    <ModelConnections
                        scope={teamId}
                        listScope={teamId}
                    />
                )}
                {shownTab && !FUSION_TABS.has(shownTab) && (

                    // The tabs' classic styles are scoped to the classic dialog's class.
                    <div className='TeamSettingsModal'>
                        <TeamSettings
                            activeTab={shownTab}
                            areThereUnsavedChanges={areThereUnsavedChanges}
                            setAreThereUnsavedChanges={setAreThereUnsavedChanges}
                            showTabSwitchError={showTabSwitchError}
                            setShowTabSwitchError={setShowTabSwitchError}
                        />
                    </div>
                )}
                <button
                    type='button'
                    className={am('icon-btn', 'close')}
                    aria-label={formatMessage({id: 'fusion.teamSettings.close', defaultMessage: 'Close team settings'})}
                    onClick={handleHide}
                >
                    <Icon name='x'/>
                </button>
            </div>
        </Dialog>
    );
};

export default TeamSettingsModal;
