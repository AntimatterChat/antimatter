// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {addUsersToGroup, removeUsersFromGroup} from 'mattermost-redux/actions/groups';
import {getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {Dialog} from 'fusion/components/layer';
import {Toggle} from 'fusion/modals/settings_controls';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {getTeamRoles} from './roles';
import {loadTeams} from './teams';

// SelfRolesDialog lets people take or leave the team's roles open to all, as Discord's self-assigned roles.
export default function SelfRolesDialog({onClose}: {onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const me = useSelector(getCurrentUserId);
    const roles = useSelector((state: GlobalState) => getTeamRoles(state, getCurrentTeamId(state))).filter((r) => r.self_assignable);

    const set = async (roleId: string, on: boolean) => {
        const result = await dispatch(on ? addUsersToGroup(roleId, [me]) : removeUsersFromGroup(roleId, [me]));
        if ('error' in result && result.error) {
            toast((result.error as {message?: string}).message || formatMessage({id: 'fusion.selfRoles.failed', defaultMessage: 'The role couldn\'t be changed.'}));
            return;
        }
        dispatch(loadTeams());
    };

    return (
        <Dialog
            label={formatMessage({id: 'fusion.selfRoles.title', defaultMessage: 'Your roles'})}
            onClose={onClose}
        >
            <div className={am('pane')}>
                <h2>{formatMessage({id: 'fusion.selfRoles.title', defaultMessage: 'Your roles'})}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.selfRoles.lead', defaultMessage: 'Roles anyone in the community can take: they colour your name, and may let you do more.'})}</p>
                {roles.map((r) => (
                    <Toggle
                        key={r.id}
                        title={r.display_name}
                        desc={`@${r.name}`}
                        on={(r.member_ids || []).includes(me)}
                        onChange={(on) => set(r.id, on)}
                    />
                ))}
                {!roles.length && <p className={am('lead')}>{formatMessage({id: 'fusion.selfRoles.none', defaultMessage: 'The community has no roles open to all.'})}</p>}
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn', 'primary')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.selfRoles.done', defaultMessage: 'Done'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
