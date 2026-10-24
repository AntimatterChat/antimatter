// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {defineMessage} from 'react-intl';
import type {MessageDescriptor} from 'react-intl';

import type {ChannelKindMode} from '@mattermost/types/channels';

// openTopicLabel is the header button's label, by mode (design §4.4): "its label follows the mode".
export function openTopicLabel(mode: ChannelKindMode): MessageDescriptor {
    switch (mode) {
    case 'qa':
        return defineMessage({id: 'fusion.forum.askQuestion', defaultMessage: 'Ask a question', description: 'Button that opens the New topic form in a Q&A forum.'});
    case 'ideas':
        return defineMessage({id: 'fusion.forum.suggestIdea', defaultMessage: 'Suggest an idea', description: 'Button that opens the New topic form in an Ideas forum.'});
    default:
        return defineMessage({id: 'fusion.forum.newTopic', defaultMessage: 'New topic', description: 'Button that opens the New topic form in a discussion or announcement forum.'});
    }
}

// The title field's placeholder, by mode.
export function topicTitlePlaceholder(mode: ChannelKindMode): MessageDescriptor {
    switch (mode) {
    case 'qa':
        return defineMessage({id: 'fusion.forum.questionPlaceholder', defaultMessage: 'What do you want to ask?', description: 'Placeholder of the title field of the New topic form in a Q&A forum.'});
    case 'ideas':
        return defineMessage({id: 'fusion.forum.ideaPlaceholder', defaultMessage: 'What\'s your idea?', description: 'Placeholder of the title field of the New topic form in an Ideas forum.'});
    default:
        return defineMessage({id: 'fusion.forum.titlePlaceholder', defaultMessage: 'Give your topic a title', description: 'Placeholder of the title field of the New topic form in a discussion or announcement forum.'});
    }
}
