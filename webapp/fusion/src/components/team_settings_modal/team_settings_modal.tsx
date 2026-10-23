// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState, useRef, useCallback, useEffect} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector} from 'react-redux';

import {getCurrentTeam, getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';

import TeamSettings from 'components/team_settings';

import CommunityAttributesSettings from 'fusion/attributes/community_attributes_dialog';
import Icon from 'fusion/components/icon';
import {Dialog} from 'fusion/components/layer';
import AISettings from 'fusion/teams/ai_settings';
import AutolinksSettings from 'fusion/teams/autolinks_settings';
import {getCommunitySettingsAccess} from 'fusion/teams/community_settings_access';
import GeneralSettings from 'fusion/teams/general_settings';
import MembersSettings from 'fusion/teams/members_settings';
import OnboardingRulesSettings from 'fusion/teams/onboarding_rules_settings';
import RolesSettings from 'fusion/teams/roles_dialog';
import TeamsSettings from 'fusion/teams/teams_settings';
import {am} from 'fusion/utils/class_names';
import {openInvitePeople} from 'fusion/utils/modals';
import {focusElement} from 'utils/a11y_utils';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

import './team_settings_modal.scss';

const SHOW_PANEL_ERROR_STATE_TAB_SWITCH_TIMEOUT = 3000;

// The tabs the Fusion UI draws itself; the others are the classic web app's.
export type FusionTeamSettingsTab = 'info' | 'roles' | 'members' | 'attributes' | 'teams' | 'onboarding' | 'ai' | 'autolinks';
const FUSION_TABS = new Set<string>(['info', 'roles', 'members', 'attributes', 'teams', 'onboarding', 'ai', 'autolinks']);

// The tabs that became parts of others: the channels and the layout are in General, the rules with the onboarding,
// the model connections with the agents.
const MERGED_TABS: Record<string, string> = {channels: 'info', layout: 'info', rules: 'onboarding', ai_connections: 'ai', agents: 'ai'};
const tabOf = (tab?: string) => (tab && MERGED_TABS[tab]) || tab || 'info';

type Props = {
    isOpen: boolean;
    onExited: () => void;
    focusOriginElement?: string;

    // The tab to open on, when its opener may see it (Antimatter: the community menu's entries).
    initialTab?: string;
};

const TeamSettingsModal = ({isOpen, onExited, focusOriginElement, initialTab}: Props) => {
    const [activeTab, setActiveTab] = useState(tabOf(initialTab));
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
        setActiveTab(tabOf(initialTab));
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

    // The tabs: General, Roles, Members (then who may join: its invitations, member attributes and membership rules),
    // Teams, Onboarding and rules, AI, Autolinks; then the integrations' pages.
    const tabs = [
        {
            name: 'info',
            uiName: formatMessage({id: 'fusion.teamSettings.general', defaultMessage: 'General'}),
            display: access.manage,
        },
        {
            name: 'roles',
            uiName: formatMessage({id: 'fusion.roles.title', defaultMessage: 'Roles'}),
            display: access.roles,
        },
        {

            // Antimatter: the members (the menu's former Manage members), for everyone who opens the settings.
            name: 'members',
            uiName: formatMessage({id: 'fusion.teamSettings.members', defaultMessage: 'Members'}),
            display: true,
        },
        {
            name: 'access',
            uiName: formatMessage({id: 'team_settings_modal.accessTab', defaultMessage: 'Access'}),

            // The community's invitations: changing them takes managing it (the server checks manage_team).
            display: access.access,
        },
        {
            name: 'attributes',
            uiName: formatMessage({id: 'fusion.communityAttributes.title', defaultMessage: 'Member attributes'}),
            display: access.manage && access.teams,
        },
        {
            name: 'team_membership',
            uiName: formatMessage({id: 'fusion.teamSettings.membershipRules', defaultMessage: 'Membership rules'}),
            display: access.teamMembership,
        },
        {
            name: 'access_policies',
            uiName: formatMessage({id: 'fusion.teamSettings.channelMembership', defaultMessage: 'Channel membership'}),
            display: access.accessPolicies,
        },
        {

            // And its teams (the menu's former Teams…), when the server has them.
            name: 'teams',
            uiName: formatMessage({id: 'fusion.teamSettings.teams', defaultMessage: 'Teams'}),
            display: access.teams,
        },
        {
            name: 'onboarding',
            uiName: formatMessage({id: 'fusion.teamSettings.onboardingRules', defaultMessage: 'Onboarding and rules'}),
            display: access.manage,
        },
        {

            // Antimatter: the community's own model connections (its admins, and the roles granted it) and agents.
            name: 'ai',
            uiName: formatMessage({id: 'fusion.teamSettings.ai', defaultMessage: 'AI'}),
            display: access.aiConnections || access.manage,
        },
        {
            name: 'autolinks',
            uiName: formatMessage({id: 'fusion.teamSettings.autolinks', defaultMessage: 'Autolinks'}),
            display: access.manage,
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

    const modalTitle = formatMessage({id: 'fusion.teamSettings.title', defaultMessage: 'Community settings'});
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
                {shownTab === 'info' && <GeneralSettings setUnsaved={setAreThereUnsavedChanges}/>}
                {shownTab === 'roles' && <RolesSettings onClose={handleHide}/>}
                {shownTab === 'members' && <MembersSettings onInvite={() => leaveFor(() => dispatch(openInvitePeople()))}/>}
                {shownTab === 'attributes' && (
                    <CommunityAttributesSettings
                        teamId={teamId}
                        onClose={handleHide}
                    />
                )}
                {shownTab === 'teams' && <TeamsSettings setUnsaved={setAreThereUnsavedChanges}/>}
                {shownTab === 'onboarding' && (
                    <OnboardingRulesSettings
                        setUnsaved={setAreThereUnsavedChanges}
                        onboarding={access.teams}
                    />
                )}
                {shownTab === 'ai' && team && (
                    <AISettings
                        team={team}
                        connections={access.aiConnections}
                        agents={access.manage}
                    />
                )}
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
                    aria-label={formatMessage({id: 'fusion.teamSettings.closeLabel', defaultMessage: 'Close the community settings'})}
                    onClick={handleHide}
                >
                    <Icon name='x'/>
                </button>
            </div>
        </Dialog>
    );
};

export default TeamSettingsModal;
