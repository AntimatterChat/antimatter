// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useSelector} from 'react-redux';

import {getTeamsUnreadStatuses} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';

import {getUnreadDirectChannels} from 'fusion/selectors';

// useUnreadMentions counts the mentions waiting for you: in the current team's channels and in direct messages.
export function useUnreadMentions(): number {
    const teamId = useSelector(getCurrentTeamId);
    const [, mentionsInTeam] = useSelector(getTeamsUnreadStatuses);
    const unreadDMs = useSelector(getUnreadDirectChannels);
    return (mentionsInTeam.get(teamId) || 0) + unreadDMs.reduce((n, dm) => n + dm.mentions, 0);
}
