// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {removeUserFromTeam} from 'mattermost-redux/actions/teams';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import {Dialog} from 'fusion/components/layer';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';

// ConfirmRemove asks before removing someone from the team: the mockup's moderation dialog, without the reason
// (Mattermost doesn't keep one).
export default function RemoveMemberDialog({userId, onClose}: {userId: string; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const user = useUser(userId);
    const name = useDisplayName(user);
    const team = useSelector(getCurrentTeam);
    const [error, setError] = useState('');

    const remove = async () => {
        if (!team) {
            return;
        }
        const result = await dispatch(removeUserFromTeam(team.id, userId));
        if (result && 'error' in result && result.error) {
            setError((result.error as {message?: string}).message || formatMessage({id: 'fusion.userMenu.removeFailed', defaultMessage: 'Could not remove {name}.'}, {name}));
            return;
        }
        toast(formatMessage({id: 'fusion.toast.removedFromTeam', defaultMessage: '{name} was removed from {team}'}, {name, team: team.display_name}));
        onClose();
    };

    return (
        <Dialog
            label={formatMessage({id: 'fusion.userMenu.removeTitle', defaultMessage: 'Remove {name} from {team}?'}, {name, team: team?.display_name})}
            onClose={onClose}
            onSubmit={remove}
        >
            <div className={am('pane')}>
                <h2>{formatMessage({id: 'fusion.userMenu.removeTitle', defaultMessage: 'Remove {name} from {team}?'}, {name, team: team?.display_name})}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.userMenu.removeLead', defaultMessage: '{name} leaves every channel of the community, but can rejoin with a new invite.'}, {name})}</p>
                {error && (
                    <p
                        className={am('lead')}
                        role='alert'
                        style={{color: 'var(--am-danger)'}}
                    >
                        {error}
                    </p>
                )}
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.userMenu.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button className={am('btn', 'danger')}>
                        {formatMessage({id: 'fusion.userMenu.remove', defaultMessage: 'Remove'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
