// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import type {IntlShape} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {fetchRemoteClusterInfo} from 'mattermost-redux/actions/shared_channels';
import {getDirectTeammate} from 'mattermost-redux/selectors/entities/channels';
import {getRemoteDisplayName} from 'mattermost-redux/selectors/entities/shared_channels';
import {getCurrentRelativeTeamUrl} from 'mattermost-redux/selectors/entities/teams';
import {displayLastActiveLabel, getLastActivityForUserId} from 'mattermost-redux/selectors/entities/users';
import {getUserCurrentTimezone} from 'mattermost-redux/utils/timezone_utils';

import {makeGetCustomStatus} from 'selectors/views/custom_status';

import Markdown from 'components/markdown';

import {Popover} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';
import Constants from 'utils/constants';
import {handleFormattedTextClick} from 'utils/utils';

import type {GlobalState} from 'types/store';

const MARKDOWN_OPTIONS = {singleline: true, mentionHighlight: false, atMentions: true};

// DirectTopic is the topic line of a direct message: the other person's local time and custom status (or the
// conversation's header), or their server when they're on another one.
// lastOnlineAgo says how long ago a moment was, in the largest whole unit: 6 minutes ago, 3 hours ago, 2 days ago.
export function lastOnlineAgo(intl: IntlShape, at: number, now = Date.now()): string {
    const minutes = Math.max(1, Math.round((now - at) / 60000));
    if (minutes < 60) {
        return intl.formatRelativeTime(-minutes, 'minute', {numeric: 'auto'});
    }
    const hours = Math.round(minutes / 60);
    if (hours < 24) {
        return intl.formatRelativeTime(-hours, 'hour', {numeric: 'auto'});
    }
    return intl.formatRelativeTime(-Math.round(hours / 24), 'day', {numeric: 'auto'});
}

function DirectTopic({channel}: {channel: Channel}) {
    const intl = useIntl();
    const {formatMessage, formatTime} = intl;
    const dispatch = useDispatch();
    const teammate = useSelector((state: GlobalState) => getDirectTeammate(state, channel.id));
    const getCustomStatus = useMemo(() => makeGetCustomStatus(), []);
    const custom = useSelector((state: GlobalState) => (teammate ? getCustomStatus(state, teammate.id) : undefined));
    const remote = useSelector((state: GlobalState) => (teammate?.remote_id ? getRemoteDisplayName(state, teammate.remote_id) : null));

    // When they were last online, while they're away or offline, as the classic header says (the server and they can
    // turn it off).
    const lastOnline = useSelector((state: GlobalState) => (teammate && displayLastActiveLabel(state, teammate.id) ? getLastActivityForUserId(state, teammate.id) : 0));

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
        const last = lastOnline ? formatMessage({id: 'fusion.header.lastOnline', defaultMessage: 'Last online {when}'}, {when: lastOnlineAgo(intl, lastOnline)}) : '';
        text = [last, time, custom?.text || channel.header].filter(Boolean).join(' · ');
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

    if (channel.type === Constants.DM_CHANNEL) {
        return <DirectTopic channel={channel}/>;
    }
    if (channel.type === Constants.GM_CHANNEL && !channel.header) {
        return <span className={am('topic')}>{formatMessage({id: 'fusion.header.group', defaultMessage: 'Group message'})}</span>;
    }
    return <ChannelTopic channel={channel}/>;
}

// ChannelTopic is a channel's header (or purpose) on one line; clicking it shows the whole header and the purpose,
// as the classic header's popover does. Links in it open as links.
function ChannelTopic({channel}: {channel: Channel}) {
    const {formatMessage} = useIntl();
    const teamUrl = useSelector(getCurrentRelativeTeamUrl);
    const ref = useRef<HTMLSpanElement>(null);
    const [open, setOpen] = useState(false);
    const topic = channel.header || channel.purpose;
    if (!topic) {
        return null;
    }
    return (
        <>
            <span
                ref={ref}
                className={am('topic', 'md', 'topic-btn')}
                role='button'
                tabIndex={0}
                title={formatMessage({id: 'fusion.header.showTopic', defaultMessage: 'Show the whole header'})}
                aria-expanded={open}
                onClick={(e) => {
                    if ((e.target as HTMLElement).closest('a')) {
                        handleFormattedTextClick(e, teamUrl);
                        return;
                    }
                    setOpen(!open);
                }}
                onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        setOpen(!open);
                    }
                }}
            >
                <Markdown
                    message={topic}
                    options={MARKDOWN_OPTIONS}
                />
            </span>
            {open && (
                <Popover
                    anchor={ref.current}
                    placement='below'
                    className='topic-pop'
                    label={formatMessage({id: 'fusion.header.topicLabel', defaultMessage: 'About {name}'}, {name: channel.display_name})}
                    onClose={() => setOpen(false)}
                >
                    {channel.header && (
                        <section onClick={(e) => handleFormattedTextClick(e, teamUrl)}>
                            <h5>{formatMessage({id: 'fusion.header.header', defaultMessage: 'Header'})}</h5>
                            <Markdown message={channel.header}/>
                        </section>
                    )}
                    {channel.purpose && (
                        <section>
                            <h5>{formatMessage({id: 'fusion.header.purpose', defaultMessage: 'Purpose'})}</h5>
                            <p>{channel.purpose}</p>
                        </section>
                    )}
                </Popover>
            )}
        </>
    );
}
