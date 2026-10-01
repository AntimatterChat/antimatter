// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {fetchRemoteClusterInfo} from 'mattermost-redux/actions/shared_channels';
import {getDirectTeammate} from 'mattermost-redux/selectors/entities/channels';
import {getRemoteDisplayName} from 'mattermost-redux/selectors/entities/shared_channels';
import {getCurrentRelativeTeamUrl} from 'mattermost-redux/selectors/entities/teams';
import {getUserCurrentTimezone} from 'mattermost-redux/utils/timezone_utils';

import {makeGetCustomStatus} from 'selectors/views/custom_status';

import Markdown from 'components/markdown';

import {am} from 'fusion/utils/class_names';
import Constants from 'utils/constants';
import {handleFormattedTextClick} from 'utils/utils';

import type {GlobalState} from 'types/store';

const MARKDOWN_OPTIONS = {singleline: true, mentionHighlight: false, atMentions: true};

// A one-line plain version of a channel header, for its tooltip.
function plain(text: string) {
    return text.replace(/\[([^\]]*)\]\([^)]*\)/g, '$1').replace(/[*_~`>#]/g, '').replace(/\s+/g, ' ').trim();
}

// DirectTopic is the topic line of a direct message: the other person's local time and custom status (or the
// conversation's header), or their server when they're on another one.
function DirectTopic({channel}: {channel: Channel}) {
    const {formatMessage, formatTime} = useIntl();
    const dispatch = useDispatch();
    const teammate = useSelector((state: GlobalState) => getDirectTeammate(state, channel.id));
    const getCustomStatus = useMemo(() => makeGetCustomStatus(), []);
    const custom = useSelector((state: GlobalState) => (teammate ? getCustomStatus(state, teammate.id) : undefined));
    const remote = useSelector((state: GlobalState) => (teammate?.remote_id ? getRemoteDisplayName(state, teammate.remote_id) : null));

    useEffect(() => {
        if (teammate?.remote_id) {
            dispatch(fetchRemoteClusterInfo(teammate.remote_id));
        }
    }, [teammate?.remote_id, dispatch]);

    if (!teammate) {
        return null;
    }
    let text;
    if (teammate.remote_id) {
        text = remote ? formatMessage({id: 'fusion.header.federatedFrom', defaultMessage: 'Federated · {host}'}, {host: remote}) : formatMessage({id: 'fusion.header.federated', defaultMessage: 'Federated'});
    } else {
        const zone = teammate.is_bot ? '' : getUserCurrentTimezone(teammate.timezone);
        let time = '';
        try {
            time = zone ? formatMessage({id: 'fusion.header.localTime', defaultMessage: '{time} local time'}, {time: formatTime(Date.now(), {timeZone: zone, hour: '2-digit', minute: '2-digit'})}) : '';
        } catch {
            time = '';
        }
        text = [time, custom?.text || channel.header].filter(Boolean).join(' · ');
    }
    if (!text) {
        return null;
    }
    return (
        <span
            className={am('topic')}
            title={text}
        >
            {text}
        </span>
    );
}

// HeaderTopic is the line beside a conversation's name: a channel's header (or purpose) as one line of markdown,
// "Group message" for a group, or a direct message's topic line.
export default function HeaderTopic({channel}: {channel: Channel}) {
    const {formatMessage} = useIntl();
    const teamUrl = useSelector(getCurrentRelativeTeamUrl);

    if (channel.type === Constants.DM_CHANNEL) {
        return <DirectTopic channel={channel}/>;
    }
    if (channel.type === Constants.GM_CHANNEL && !channel.header) {
        return <span className={am('topic')}>{formatMessage({id: 'fusion.header.group', defaultMessage: 'Group message'})}</span>;
    }
    const topic = channel.header || channel.purpose;
    if (!topic) {
        return null;
    }
    return (
        <span
            className={am('topic', 'md')}
            title={plain(topic)}
            onClick={(e) => handleFormattedTextClick(e, teamUrl)}
        >
            <Markdown
                message={topic}
                options={MARKDOWN_OPTIONS}
            />
        </span>
    );
}
