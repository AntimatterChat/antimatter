// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import type {Group} from '@mattermost/types/groups';

import {getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';

import {Dialog} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';

import TeamEditor from './team_editor';

// TeamEditDialog edits a team from its card, with the editor of the community settings' Teams tab: its name, @mention,
// badge, lead, channel, what it does and its members.
export default function TeamEditDialog({team, onClose}: {team: Group; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const currentTeamId = useSelector(getCurrentTeamId);
    return (
        <Dialog
            label={formatMessage({id: 'fusion.teams.editTitle', defaultMessage: 'Edit {team}'}, {team: team.display_name})}
            onClose={onClose}
        >
            <div className={am('pane')}>
                <h2>{team.display_name}</h2>
                <TeamEditor
                    team={team}
                    communityId={team.team_id || currentTeamId}
                    onDone={onClose}
                    onCancel={onClose}
                />
            </div>
        </Dialog>
    );
}
