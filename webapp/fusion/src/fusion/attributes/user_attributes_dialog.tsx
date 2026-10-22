// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {UserAttributeValue, UserPropertyField} from '@mattermost/types/properties_user';

import {UserTypes} from 'mattermost-redux/action_types';
import {getCustomProfileAttributeValues} from 'mattermost-redux/actions/users';
import {Client4} from 'mattermost-redux/client';
import {Permissions} from 'mattermost-redux/constants';
import {getCustomProfileAttributes} from 'mattermost-redux/selectors/entities/general';
import {haveISystemPermission, haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';
import {isPropertyFieldSyncedForAuthService} from 'mattermost-redux/utils/property_utils';

import {Dialog} from 'fusion/components/layer';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {Section} from 'fusion/modals/settings_controls';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {AttributeInput, AttributeValue, attributeLabel, isEmptyValue} from './attribute_inputs';
import type {AttributeField} from './attribute_inputs';
import {saveMemberAttributeValues, useCommunityAttributes, useMemberAttributeValues} from './community_attributes';

// The server's user attributes a person's dialog shows: not those only an integration reads.
export function shownServerAttributes(fields: UserPropertyField[]): UserPropertyField[] {
    return fields.filter((field) => field.attrs?.access_mode !== 'source_only');
}

// useCanEditServerAttributes tells whether the current user sets anyone's server attributes: those who administer
// users.
export function useCanEditServerAttributes(): boolean {
    return useSelector((state: GlobalState) => haveISystemPermission(state, {permission: Permissions.EDIT_OTHER_USERS}) || haveISystemPermission(state, {permission: Permissions.MANAGE_SYSTEM}));
}

// serverAttributeLock tells why an admin can't set a server attribute here, or '' when they can: those a login
// provider or an integration sets, and those whose kind the dialog doesn't edit.
function serverAttributeLock(field: UserPropertyField, authService: string, formatMessage: ReturnType<typeof useIntl>['formatMessage']): string {
    const attrs = field.attrs || {};
    if (isPropertyFieldSyncedForAuthService(field, authService)) {
        return formatMessage({id: 'fusion.attributes.lockedByProvider', defaultMessage: 'Set by their login provider.'});
    }
    if (attrs.owners?.length || attrs.protected) {
        return formatMessage({id: 'fusion.attributes.lockedByIntegration', defaultMessage: 'Managed by an integration.'});
    }
    if (attrs.options_omitted || !['text', 'select', 'multiselect', 'date', 'user', 'multiuser'].includes(field.type)) {
        return formatMessage({id: 'fusion.attributes.lockedConsole', defaultMessage: 'Set it in the System Console.'});
    }
    return '';
}

type Props = {
    userId: string;
    teamId: string;
    toast?: (text: string) => void;
    onHide?: () => void;
};

// UserAttributesDialog sets someone's attributes: the server's, for those who administer users, and the
// community's, for its admins (and for the person themselves, those the admins don't manage).
export default function UserAttributesDialog({userId, teamId, toast, onHide}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const user = useUser(userId);
    const name = useDisplayName(user);
    const me = useSelector(getCurrentUserId);
    const team = useSelector((state: GlobalState) => getTeam(state, teamId));
    const serverAdmin = useCanEditServerAttributes();
    const communityAdmin = useSelector((state: GlobalState) => haveITeamPermission(state, teamId, Permissions.MANAGE_TEAM));
    const serverFields = shownServerAttributes(useSelector(getCustomProfileAttributes));
    const communityFields = useCommunityAttributes(teamId) || [];
    const communityValues = useMemberAttributeValues(teamId, userId) || {};
    const serverValues = user?.custom_profile_attributes || {};
    const [draft, setDraft] = useState<Record<string, UserAttributeValue>>({});
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');

    useEffect(() => {
        if (serverAdmin && serverFields.length) {
            dispatch(getCustomProfileAttributeValues(userId));
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [userId, serverAdmin, serverFields.length]);

    const close = () => onHide?.();
    const changed = (stored: Record<string, UserAttributeValue>) => Object.fromEntries(Object.entries(draft).filter(([id]) => id in draft && JSON.stringify(draft[id]) !== JSON.stringify(stored[id] ?? (Array.isArray(draft[id]) ? [] : ''))));
    const serverIds = new Set(serverFields.map((f) => f.id));
    const communityIds = new Set(communityFields.map((f) => f.id));

    const save = async () => {
        if (busy) {
            return;
        }
        const serverChanges = Object.fromEntries(Object.entries(changed(serverValues)).filter(([id]) => serverIds.has(id)));
        const communityChanges = Object.fromEntries(Object.entries(changed(communityValues)).filter(([id]) => communityIds.has(id)));
        setBusy(true);
        setError('');
        try {
            if (Object.keys(serverChanges).length) {
                const saved = await Client4.updateUserCustomProfileAttributesValues(userId, serverChanges as Record<string, string | string[]>);
                dispatch({type: UserTypes.RECEIVED_CPA_VALUES, data: {userID: userId, customAttributeValues: saved}});
            }
            if (Object.keys(communityChanges).length) {
                await saveMemberAttributeValues(teamId, userId, communityChanges);
            }
        } catch (err) {
            setBusy(false);
            setError((err as {message?: string}).message || formatMessage({id: 'fusion.attributes.saveFailed', defaultMessage: 'The attributes couldn\'t be saved.'}));
            return;
        }
        setBusy(false);
        toast?.(formatMessage({id: 'fusion.attributes.saved', defaultMessage: 'Saved {name}\'s attributes'}, {name}));
        close();
    };

    const row = (field: AttributeField, stored: Record<string, UserAttributeValue>, lock: string) => {
        const value = draft[field.id] ?? stored[field.id];
        const visibility = field.attrs?.visibility;
        let note = lock;
        if (!note && visibility === 'admin') {
            note = formatMessage({id: 'fusion.attributes.adminOnlyNote', defaultMessage: 'Only admins and {name} see it.'}, {name});
        } else if (!note && visibility === 'hidden') {
            note = formatMessage({id: 'fusion.attributes.hiddenNote', defaultMessage: 'Not shown on profiles.'});
        }
        return (
            <div
                key={field.id}
                className={am('field', 'attr-field')}
            >
                <span>{attributeLabel(field)}</span>
                {lock ? (
                    <span className={am('attr-readonly')}>
                        {isEmptyValue(value) ? formatMessage({id: 'fusion.attributes.none', defaultMessage: '—'}) : (
                            <AttributeValue
                                field={field}
                                value={value}
                            />
                        )}
                    </span>
                ) : (
                    <AttributeInput
                        field={field}
                        value={value}
                        teamId={teamId}
                        onChange={(next) => setDraft((d) => ({...d, [field.id]: next}))}
                    />
                )}
                {note && <small>{note}</small>}
            </div>
        );
    };

    const showServer = serverAdmin && serverFields.length > 0;
    const communityLock = (field: AttributeField) => {
        if (communityAdmin || (userId === me && field.attrs?.managed !== 'admin')) {
            return '';
        }
        return formatMessage({id: 'fusion.attributes.lockedCommunity', defaultMessage: 'Set by the community\'s admins.'});
    };
    const title = formatMessage({id: 'fusion.attributes.dialogTitle', defaultMessage: '{name}\'s attributes'}, {name});

    return (
        <Dialog
            label={title}
            onClose={close}
            onSubmit={save}
        >
            <div className={am('pane', 'attr-pane')}>
                <h2>{title}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.attributes.dialogLead', defaultMessage: 'Details about them that admins keep up to date, such as where they work or for whom. They show on their profile.'})}</p>
                {showServer && (
                    <>
                        <Section title={formatMessage({id: 'fusion.attributes.serverSection', defaultMessage: 'Everywhere on the server'})}/>
                        {serverFields.map((field) => row(field, serverValues, serverAttributeLock(field, user?.auth_service || '', formatMessage)))}
                    </>
                )}
                {communityFields.length > 0 && (
                    <>
                        <Section title={formatMessage({id: 'fusion.attributes.communitySection', defaultMessage: 'In {community}'}, {community: team?.display_name || ''})}/>
                        {communityFields.map((field) => row(field, communityValues, communityLock(field)))}
                    </>
                )}
                {error && (
                    <p
                        className={am('field-error')}
                        role='alert'
                    >
                        {error}
                    </p>
                )}
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={close}
                    >
                        {formatMessage({id: 'fusion.attributes.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button
                        type='submit'
                        className={am('btn', 'primary')}
                        disabled={busy || !Object.keys(draft).length}
                    >
                        {formatMessage({id: 'fusion.attributes.save', defaultMessage: 'Save'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
