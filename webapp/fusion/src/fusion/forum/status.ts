// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {defineMessage} from 'react-intl';
import type {MessageDescriptor} from 'react-intl';

import type {ForumTopicStatus} from '@mattermost/types/posts';

// An Ideas topic's status (design §3.3, §4.4): missing on the server means open. In the order the Ideas sort and the
// status menu show them (the owner's decision, DECISIONS.md "Ideas status sort").
export const FORUM_TOPIC_STATUSES: ForumTopicStatus[] = ['in_progress', 'planned', 'open', 'done', 'declined'];

// The order the status filter chips and the "Set status" menu show instead (the design's own, open first).
export const FORUM_TOPIC_STATUS_MENU_ORDER: ForumTopicStatus[] = ['open', 'planned', 'in_progress', 'done', 'declined'];

export function topicStatusLabel(status: ForumTopicStatus): MessageDescriptor {
    switch (status) {
    case 'planned':
        return defineMessage({id: 'fusion.topicStatus.planned', defaultMessage: 'Planned', description: 'An Ideas topic\'s status: picked, not started yet.'});
    case 'in_progress':
        return defineMessage({id: 'fusion.topicStatus.inProgress', defaultMessage: 'In progress', description: 'An Ideas topic\'s status: being worked on.'});
    case 'done':
        return defineMessage({id: 'fusion.topicStatus.done', defaultMessage: 'Done', description: 'An Ideas topic\'s status: shipped.'});
    case 'declined':
        return defineMessage({id: 'fusion.topicStatus.declined', defaultMessage: 'Declined', description: 'An Ideas topic\'s status: not going ahead.'});
    default:
        return defineMessage({id: 'fusion.topicStatus.open', defaultMessage: 'Open', description: 'An Ideas topic\'s status: not yet looked at (the default, missing on the server).'});
    }
}
