// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type React from 'react';
import {lazy} from 'react';

import type {ChannelCategory} from '@mattermost/types/channel_categories';

import {openModal} from 'actions/views/modals';

import {makeAsyncComponent} from 'components/async_load';

import {ModalIdentifiers} from 'utils/constants';

import type {ModalData} from 'types/actions';

// Dialogs of the classic web app that the Fusion UI opens as they are, until they get their own Fusion version.

// openDialog opens a classic dialog; the modal controller provides onHide/onExited, which some dialogs type as required.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const openDialog = (modalId: string, dialogType: React.ElementType<any>, dialogProps?: Record<string, unknown>) => openModal({modalId, dialogType, dialogProps} as ModalData<unknown>);

const BrowseChannels = makeAsyncComponent('FusionBrowseChannels', lazy(() => import('components/browse_channels')));
const DeleteCategoryModal = makeAsyncComponent('FusionDeleteCategoryModal', lazy(() => import('components/delete_category_modal')));
const EditCategoryModal = makeAsyncComponent('FusionEditCategoryModal', lazy(() => import('components/edit_category_modal')));
const InvitationModal = makeAsyncComponent('FusionInvitationModal', lazy(() => import('components/invitation_modal')));
const KeyboardShortcutsModal = makeAsyncComponent('FusionKeyboardShortcutsModal', lazy(() => import('components/keyboard_shortcuts/keyboard_shortcuts_modal/keyboard_shortcuts_modal')));
const LeaveTeamModal = makeAsyncComponent('FusionLeaveTeamModal', lazy(() => import('components/leave_team_modal')));
const MoreDirectChannels = makeAsyncComponent('FusionMoreDirectChannels', lazy(() => import('components/more_direct_channels')));
const NewChannelModal = makeAsyncComponent('FusionNewChannelModal', lazy(() => import('components/new_channel_modal/new_channel_modal')));
const TeamMembersModal = makeAsyncComponent('FusionTeamMembersModal', lazy(() => import('components/team_members_modal')));
const TeamSettingsModal = makeAsyncComponent('FusionTeamSettingsModal', lazy(() => import('components/team_settings_modal')));
const UserSettingsModal = makeAsyncComponent('FusionUserSettingsModal', lazy(() => import('components/user_settings/modal')));

export const openBrowseChannels = () => openDialog(ModalIdentifiers.MORE_CHANNELS, BrowseChannels);
export const openNewChannel = () => openDialog(ModalIdentifiers.NEW_CHANNEL_MODAL, NewChannelModal);
export const openCreateCategory = () => openDialog(ModalIdentifiers.EDIT_CATEGORY, EditCategoryModal, {});
export const openRenameCategory = (category: ChannelCategory) => openDialog(ModalIdentifiers.EDIT_CATEGORY, EditCategoryModal, {categoryId: category.id, initialCategoryName: category.display_name});
export const openDeleteCategory = (category: ChannelCategory) => openDialog(ModalIdentifiers.DELETE_CATEGORY, DeleteCategoryModal, {category});
export const openInvitePeople = () => openDialog(ModalIdentifiers.INVITATION, InvitationModal, {});
export const openTeamSettings = () => openDialog(ModalIdentifiers.TEAM_SETTINGS, TeamSettingsModal, {isOpen: true});
export const openTeamMembers = () => openDialog(ModalIdentifiers.TEAM_MEMBERS, TeamMembersModal, {});
export const openLeaveTeam = () => openDialog(ModalIdentifiers.LEAVE_TEAM, LeaveTeamModal);
export const openNewDirectMessage = () => openDialog(ModalIdentifiers.CREATE_DM_CHANNEL, MoreDirectChannels, {isExistingChannel: false});
export const openKeyboardShortcuts = () => openDialog(ModalIdentifiers.KEYBOARD_SHORTCUTS_MODAL, KeyboardShortcutsModal);
export const openClassicUserSettings = (activeTab?: string) => openDialog(ModalIdentifiers.USER_SETTINGS, UserSettingsModal, {isContentProductSettings: true, activeTab});
