// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState, useRef, useCallback, useEffect} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import {Permissions} from 'mattermost-redux/constants';
import {isCustomGroupsEnabled} from 'mattermost-redux/selectors/entities/preferences';
import {haveISystemPermission, haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeam, getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';

import {isChannelAccessControlEnabled, isTeamMembershipAccessControlEnabled} from 'selectors/general';

import TeamSettings from 'components/team_settings';

import CommunityAttributesSettings from 'fusion/attributes/community_attributes_dialog';
import Icon from 'fusion/components/icon';
import {Dialog} from 'fusion/components/layer';
import AutolinksSettings from 'fusion/teams/autolinks_settings';
import ChannelsSettings from 'fusion/teams/channels_settings';
import OnboardingSettings from 'fusion/teams/onboarding_editor';
import RolesSettings from 'fusion/teams/roles_dialog';
import {TeamLayoutSettings} from 'fusion/teams/team_layout_dialog';
import {am} from 'fusion/utils/class_names';
import {focusElement} from 'utils/a11y_utils';

import type {GlobalState} from 'types/store';

import './team_settings_modal.scss';

const SHOW_PANEL_ERROR_STATE_TAB_SWITCH_TIMEOUT = 3000;

// The tabs the Fusion UI draws itself; the others are the classic web app's.
export type FusionTeamSettingsTab = 'channels' | 'roles' | 'onboarding' | 'layout' | 'attributes' | 'autolinks';
const FUSION_TABS = new Set<string>(['channels', 'roles', 'onboarding', 'layout', 'attributes', 'autolinks']);

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

    const teamId = useSelector(getCurrentTeamId);
    const team = useSelector(getCurrentTeam);
    const canInviteUsers = useSelector((state: GlobalState) =>
        haveITeamPermission(state, teamId, Permissions.INVITE_USER),
    );
    const abacEnabled = useSelector(isChannelAccessControlEnabled);
    const teamMembershipAbacEnabled = useSelector(isTeamMembershipAccessControlEnabled);
    const isSystemAdmin = useSelector((state: GlobalState) =>
        haveISystemPermission(state, {permission: Permissions.MANAGE_SYSTEM}),
    );
    const hasTeamAccessRulesPermission = useSelector((state: GlobalState) =>
        haveITeamPermission(state, teamId, Permissions.MANAGE_TEAM_ACCESS_RULES),
    );
    const canManageTeamAccessRules = isSystemAdmin || hasTeamAccessRulesPermission;
    const canManageTeam = useSelector((state: GlobalState) => haveITeamPermission(state, teamId, Permissions.MANAGE_TEAM));
    const canManageRoles = useSelector((state: GlobalState) => haveITeamPermission(state, teamId, Permissions.MANAGE_TEAM_ROLES));
    const customGroups = useSelector(isCustomGroupsEnabled);

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

            // Those who only manage the roles (not the community) get the Roles tab alone.
            display: canManageTeam || !canManageRoles,
        },
        {
            name: 'access',
            uiName: formatMessage({id: 'team_settings_modal.accessTab', defaultMessage: 'Access'}),
            icon: 'icon icon-account-multiple-outline',
            iconTitle: formatMessage({id: 'generic_icons.member', defaultMessage: 'Member Icon'}),
            display: canInviteUsers,
        },
        {
            name: 'team_membership',
            uiName: formatMessage({id: 'team_settings_modal.teamMembershipTab', defaultMessage: 'Team Membership'}),
            icon: 'icon icon-iframe-list-outline',
            iconTitle: formatMessage({id: 'generic_icons.team_membership', defaultMessage: 'Team Membership Icon'}),
            display: teamMembershipAbacEnabled && canManageTeamAccessRules,
        },
        {
            name: 'access_policies',
            uiName: formatMessage({id: 'team_settings_modal.accessPoliciesTab', defaultMessage: 'Channel Membership'}),
            icon: 'icon icon-message-check-outline',
            iconTitle: formatMessage({id: 'generic_icons.access_rules', defaultMessage: 'Membership Policy Icon'}),
            display: abacEnabled && canManageTeamAccessRules,
        },
        {
            name: 'channels',
            uiName: formatMessage({id: 'fusion.teamSettings.channels', defaultMessage: 'Channels'}),
            icon: 'icon icon-globe',
            iconTitle: formatMessage({id: 'fusion.teamSettings.channels', defaultMessage: 'Channels'}),
            display: canManageTeam,
        },
        {
            name: 'roles',
            uiName: formatMessage({id: 'fusion.roles.title', defaultMessage: 'Roles'}),
            icon: 'icon icon-shield-outline',
            iconTitle: formatMessage({id: 'fusion.roles.title', defaultMessage: 'Roles'}),
            display: canManageRoles && customGroups,
        },
        {
            name: 'onboarding',
            uiName: formatMessage({id: 'fusion.onboarding.editTitle', defaultMessage: 'Onboarding'}),
            icon: 'icon icon-message-text-outline',
            iconTitle: formatMessage({id: 'fusion.onboarding.editTitle', defaultMessage: 'Onboarding'}),
            display: canManageTeam && customGroups,
        },
        {
            name: 'attributes',
            uiName: formatMessage({id: 'fusion.communityAttributes.title', defaultMessage: 'Member attributes'}),
            icon: 'icon icon-tag-outline',
            iconTitle: formatMessage({id: 'fusion.communityAttributes.title', defaultMessage: 'Member attributes'}),
            display: canManageTeam && customGroups,
        },
        {
            name: 'layout',
            uiName: formatMessage({id: 'fusion.layout.title', defaultMessage: 'Community layout'}),
            icon: 'icon icon-view-grid-outline',
            iconTitle: formatMessage({id: 'fusion.layout.title', defaultMessage: 'Community layout'}),
            display: canManageTeam && customGroups,
        },
        {
            name: 'autolinks',
            uiName: formatMessage({id: 'fusion.teamSettings.autolinks', defaultMessage: 'Autolinks'}),
            icon: 'icon icon-link-variant',
            iconTitle: formatMessage({id: 'fusion.teamSettings.autolinks', defaultMessage: 'Autolinks'}),
            display: canManageTeam,
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
