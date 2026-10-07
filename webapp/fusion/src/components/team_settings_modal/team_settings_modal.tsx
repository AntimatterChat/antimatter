// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState, useRef, useCallback, useEffect} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import {Permissions} from 'mattermost-redux/constants';
import {haveISystemPermission, haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeam, getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';

import {isChannelAccessControlEnabled, isTeamMembershipAccessControlEnabled} from 'selectors/general';

import TeamSettings from 'components/team_settings';

import Icon from 'fusion/components/icon';
import {Dialog} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';
import {focusElement} from 'utils/a11y_utils';

import type {GlobalState} from 'types/store';

import './team_settings_modal.scss';

const SHOW_PANEL_ERROR_STATE_TAB_SWITCH_TIMEOUT = 3000;

type Props = {
    isOpen: boolean;
    onExited: () => void;
    focusOriginElement?: string;
};

const TeamSettingsModal = ({isOpen, onExited, focusOriginElement}: Props) => {
    const [activeTab, setActiveTab] = useState('info');
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
        setActiveTab('info');
        setAreThereUnsavedChanges(false);
        setShowTabSwitchError(false);
        setHasBeenWarned(false);

        // Restore focus
        if (focusOriginElement) {
            focusElement(focusOriginElement, true);
        }

        // Notify parent
        onExited();
    }, [onExited, focusOriginElement]);

    const tabs = [
        {
            name: 'info',
            uiName: formatMessage({id: 'fusion.teamSettings.overview', defaultMessage: 'Overview'}),
            icon: 'icon icon-information-outline',
            iconTitle: formatMessage({id: 'generic_icons.info', defaultMessage: 'Info Icon'}),
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
                        className={am({on: tab.name === activeTab})}
                        aria-selected={tab.name === activeTab}
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
                <h2>{visibleTabs.find((tab) => tab.name === activeTab)?.uiName}</h2>

                {/* The tabs' classic styles are scoped to the classic dialog's class. */}
                <div className='TeamSettingsModal'>
                    <TeamSettings
                        activeTab={activeTab}
                        areThereUnsavedChanges={areThereUnsavedChanges}
                        setAreThereUnsavedChanges={setAreThereUnsavedChanges}
                        showTabSwitchError={showTabSwitchError}
                        setShowTabSwitchError={setShowTabSwitchError}
                    />
                </div>
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
