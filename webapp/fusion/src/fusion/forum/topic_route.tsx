// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect} from 'react';
import {useDispatch, useSelector} from 'react-redux';
import {useHistory, useParams} from 'react-router-dom';

import {getCurrentChannel} from 'mattermost-redux/selectors/entities/channels';

import {onChannelByIdentifierEnter} from 'components/channel_layout/channel_identifier_router/actions';
import type {Match} from 'components/channel_layout/channel_identifier_router/channel_identifier_router';
import LoadingScreen from 'components/loading_screen';

import TopicPage from './topic_page';

type Params = {
    team: string;
    identifier: string;
    rootId: string;
};

// ForumTopicRoute opens /<team>/channels/<channel>/topics/<root id> (design §4.4): the channel is resolved the same
// way the plain channel route is (ChannelIdentifierRouter's own action, by name or id), then the topic page shows
// instead of the topics view.
export default function ForumTopicRoute() {
    const dispatch = useDispatch();
    const history = useHistory();
    const {team, identifier, rootId} = useParams<Params>();
    const channel = useSelector(getCurrentChannel);

    useEffect(() => {
        const match: Match = {params: {team, identifier, path: 'channels'}, isExact: true, path: '', url: ''};
        dispatch(onChannelByIdentifierEnter({match, history}));

        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [team, identifier]);

    if (!channel || (channel.name !== identifier && channel.id !== identifier)) {
        return <LoadingScreen/>;
    }

    return (
        <TopicPage
            channel={channel}
            rootId={rootId}
        />
    );
}
