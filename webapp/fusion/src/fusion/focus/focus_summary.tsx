// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {DateTime} from 'luxon';
import React, {useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {WebSocketEvents} from '@mattermost/client';
import type {WebSocketMessage} from '@mattermost/client';
import type {FocusSummary, FocusSummaryChannel, FocusSummaryThread} from '@mattermost/types/focus';

import {savePreferences} from 'mattermost-redux/actions/preferences';
import {getMissingProfilesByIds} from 'mattermost-redux/actions/users';
import {Client4} from 'mattermost-redux/client';
import {getChannel} from 'mattermost-redux/selectors/entities/channels';
import {get as getPreference} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentTeam, getTeams} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {selectPostById} from 'actions/views/rhs';

import webSocketClient from 'client/web_websocket_client';
import {Dialog} from 'fusion/components/layer';
import {useMyZone} from 'fusion/composer/schedule';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';
import {channelPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

import {FOCUS_PREF_CATEGORY, FOCUS_PREF_SEEN, setFocusSummary, useFocusSummaryWindow} from './focus';
import type {FocusWindow} from './focus';

// The other person of a direct message, from its name (their two ids).
function teammateId(channel: Pick<FocusSummaryChannel, 'name'>, me: string): string {
    const [a, b] = channel.name.split('__');
    return a === me ? b : a;
}

function useChannelName(item: FocusSummaryChannel): string {
    const me = useSelector(getCurrentUserId);
    const known = useSelector((state: GlobalState) => getChannel(state, item.channel_id));
    const teammate = useDisplayName(useUser(item.type === 'D' ? teammateId(item, me) : undefined));
    if (item.type === 'D') {
        return teammate;
    }
    return known?.display_name || item.display_name || item.name;
}

// useOpen opens a channel of the summary (in its community, or the current one for direct and group messages), and a
// thread in the right panel beside its channel.
function useOpen() {
    const dispatch = useDispatch();
    const current = useSelector(getCurrentTeam);
    const teams = useSelector(getTeams);
    return (channel: {team_id: string; name: string}, threadId?: string) => {
        const team = (channel.team_id && teams[channel.team_id]?.name) || current?.name || '';
        if (!team) {
            return;
        }
        getHistory().push(channelPath(team, channel));
        if (threadId) {
            dispatch(selectPostById(threadId));
        }
    };
}

function ChannelItem({item, onOpen}: {item: FocusSummaryChannel; onOpen: () => void}) {
    const {formatMessage} = useIntl();
    const name = useChannelName(item);
    const sender = useDisplayName(useUser(item.last_sender_id));
    const open = useOpen();
    const count = item.count >= 1000 ? '999+' : String(item.count);
    return (
        <li>
            <button
                type='button'
                className={am('focus-item')}
                onClick={() => {
                    open(item);
                    onOpen();
                }}
            >
                <span className={am('grow')}>
                    <b>{item.type === 'O' || item.type === 'P' ? `#${name}` : name}</b>
                    <span>
                        {formatMessage({id: 'fusion.focus.summaryCount', defaultMessage: '{count, plural, one {# new message} other {{shown} new messages}} · last from {name}', description: 'Summary after focus mode: how many messages arrived in a conversation, and who sent the last one; {shown} is the count, or 999+.'}, {count: item.count, shown: count, name: sender})}
                    </span>
                </span>
                {item.mention_count > 0 && (
                    <span
                        className={am('badge')}
                        aria-label={formatMessage({id: 'fusion.focus.summaryMentions', defaultMessage: '{count, plural, one {# mention} other {# mentions}}', description: 'Summary after focus mode: the mentions waiting in a conversation or thread.'}, {count: item.mention_count})}
                    >
                        {item.mention_count}
                    </span>
                )}
            </button>
        </li>
    );
}

function ThreadItem({item, onOpen}: {item: FocusSummaryThread; onOpen: () => void}) {
    const {formatMessage} = useIntl();
    const channel = useSelector((state: GlobalState) => getChannel(state, item.channel_id));
    const sender = useDisplayName(useUser(item.last_sender_id));
    const open = useOpen();
    if (!channel) {
        return null;
    }
    return (
        <li>
            <button
                type='button'
                className={am('focus-item')}
                onClick={() => {
                    open({team_id: channel.team_id || item.team_id, name: channel.name}, item.post_id);
                    onOpen();
                }}
            >
                <span className={am('grow')}>
                    <b>{formatMessage({id: 'fusion.focus.summaryThread', defaultMessage: 'Thread in {channel}', description: 'Summary after focus mode: a followed thread with new replies, in the channel named.'}, {channel: channel.type === 'O' || channel.type === 'P' ? `#${channel.display_name}` : channel.display_name})}</b>
                    <span>
                        {formatMessage({id: 'fusion.focus.summaryReplies', defaultMessage: '{count, plural, one {# new reply} other {{shown} new replies}} · last from {name}', description: 'Summary after focus mode: how many replies arrived in a thread, and who wrote the last one; {shown} is the count, or 999+.'}, {count: item.count, shown: item.count >= 1000 ? '999+' : String(item.count), name: sender})}
                    </span>
                </span>
                {item.mention_count > 0 && (
                    <span
                        className={am('badge')}
                        aria-label={formatMessage({id: 'fusion.focus.summaryMentions', defaultMessage: '{count, plural, one {# mention} other {# mentions}}', description: 'Summary after focus mode: the mentions waiting in a conversation or thread.'}, {count: item.mention_count})}
                    >
                        {item.mention_count}
                    </span>
                )}
            </button>
        </li>
    );
}

// useFocusRange is the focus as times, with the day when it spans more than one.
export function useFocusRange(span: FocusWindow): string {
    const intl = useIntl();
    const zone = useMyZone();
    const sameDay = DateTime.fromMillis(span.start_at, {zone}).hasSame(DateTime.fromMillis(span.end_at, {zone}), 'day');
    const options: Intl.DateTimeFormatOptions = sameDay ? {hour: 'numeric', minute: '2-digit', timeZone: zone} : {weekday: 'short', hour: 'numeric', minute: '2-digit', timeZone: zone};
    return `${intl.formatDate(span.start_at, options)}–${intl.formatDate(span.end_at, options)}`;
}

type Props = {
    span: FocusWindow;
    summary: FocusSummary;

    // markRead: whether to mark it all read (up to the end) before the badges come back.
    onDone: (markRead: boolean) => void;
};

// FocusSummaryDialog tells what you missed while you focused: direct and group messages, channels and followed threads
// with what came, the mentions waiting there; and either marks it all read, up to the focus's end, or brings the
// unread badges back. Closing it brings the badges back.
export function FocusSummaryDialog({span, summary, onDone}: Props) {
    const {formatMessage} = useIntl();
    const range = useFocusRange(span);
    const [busy, setBusy] = useState(false);
    const direct = summary.channels.filter((c) => c.type === 'D' || c.type === 'G');
    const channels = summary.channels.filter((c) => c.type !== 'D' && c.type !== 'G');
    const title = formatMessage({id: 'fusion.focus.summaryTitle', defaultMessage: 'While you were focusing ({range})', description: 'Title of the summary shown when focus mode ends; {range} is its start and end times, e.g. 14:00–15:30.'}, {range});
    const close = () => onDone(false);

    return (
        <Dialog
            label={title}
            onClose={close}
            className={am('focus-summary')}
        >
            <div className={am('pane')}>
                <h2>{title}</h2>
                <p className={am('lead')}>
                    {summary.mention_count > 0 ? formatMessage({id: 'fusion.focus.summaryLeadMentions', defaultMessage: '{count, plural, one {# mention waits} other {# mentions wait}} for you.', description: 'Summary after focus mode: the mentions waiting for you.'}, {count: summary.mention_count}) : formatMessage({id: 'fusion.focus.summaryLead', defaultMessage: 'Here is what came in.', description: 'Summary after focus mode, when nobody mentioned you: what follows is what arrived.'})}
                </p>
                <div className={am('focus-list')}>
                    {direct.length > 0 && (
                        <section>
                            <h3>{formatMessage({id: 'fusion.focus.summaryDirect', defaultMessage: 'Direct messages', description: 'Summary after focus mode: the heading of the direct and group messages that arrived.'})}</h3>
                            <ul>
                                {direct.map((item) => (
                                    <ChannelItem
                                        key={item.channel_id}
                                        item={item}
                                        onOpen={close}
                                    />
                                ))}
                            </ul>
                        </section>
                    )}
                    {channels.length > 0 && (
                        <section>
                            <h3>{formatMessage({id: 'fusion.focus.summaryChannels', defaultMessage: 'Channels', description: 'Summary after focus mode: the heading of the channels with new messages.'})}</h3>
                            <ul>
                                {channels.map((item) => (
                                    <ChannelItem
                                        key={item.channel_id}
                                        item={item}
                                        onOpen={close}
                                    />
                                ))}
                            </ul>
                        </section>
                    )}
                    {summary.threads.length > 0 && (
                        <section>
                            <h3>{formatMessage({id: 'fusion.focus.summaryThreads', defaultMessage: 'Threads', description: 'Summary after focus mode: the heading of the followed threads with new replies.'})}</h3>
                            <ul>
                                {summary.threads.map((item) => (
                                    <ThreadItem
                                        key={item.post_id}
                                        item={item}
                                        onOpen={close}
                                    />
                                ))}
                            </ul>
                        </section>
                    )}
                </div>
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={close}
                    >
                        {formatMessage({id: 'fusion.focus.showBadges', defaultMessage: 'Show unread badges', description: 'Summary after focus mode: closes it, and the unread badges come back for what you missed.'})}
                    </button>
                    <button
                        type='button'
                        className={am('btn', 'primary')}
                        disabled={busy}
                        onClick={() => {
                            setBusy(true);
                            onDone(true);
                        }}
                    >
                        {formatMessage({id: 'fusion.focus.markAllRead', defaultMessage: 'Mark all as read', description: 'Summary after focus mode: marks everything it lists as read, up to the end of the focus.'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}

// FocusSummaryHost shows, once, what you missed when a focus of yours ends: told by the server (focus_ended, on any of
// your devices), or found on loading or reconnecting when it ended while this window was away. Seen on one device, it
// isn't shown again on another (a preference keeps the last one seen). While it waits, your unread badges stay hidden.
export default function FocusSummaryHost() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const userId = useSelector(getCurrentUserId);
    const seen = useSelector((state: GlobalState) => getPreference(state, FOCUS_PREF_CATEGORY, FOCUS_PREF_SEEN, ''));
    const seenRef = useRef(seen);
    seenRef.current = seen;
    const span = useFocusSummaryWindow();
    const [summary, setSummary] = useState<FocusSummary | null>(null);

    useEffect(() => {
        let gone = false;
        const check = async () => {
            try {
                const state = await Client4.getFocus();
                if (!gone && !state.active && state.ended_at > 0 && String(state.start_at) !== seenRef.current) {
                    setFocusSummary({start_at: state.start_at, end_at: state.ended_at});
                }
            } catch {
                // Nothing to show.
            }
        };
        const onMessage = (message: WebSocketMessage) => {
            if (message.event === WebSocketEvents.FocusEnded) {
                setFocusSummary({start_at: message.data.start_at, end_at: message.data.end_at});
            }
        };
        check();
        webSocketClient.addMessageListener(onMessage);
        webSocketClient.addReconnectListener(check);
        return () => {
            gone = true;
            webSocketClient.removeMessageListener(onMessage);
            webSocketClient.removeReconnectListener(check);
        };
    }, []);

    const markSeen = (start: number) => {
        if (userId) {
            dispatch(savePreferences(userId, [{user_id: userId, category: FOCUS_PREF_CATEGORY, name: FOCUS_PREF_SEEN, value: String(start)}]));
        }
        setSummary(null);
        setFocusSummary(null);
    };

    // The summary of the focus that ended; nothing missed, nothing to show.
    const start = span?.start_at;
    const end = span?.end_at;
    useEffect(() => {
        if (!start || !end) {
            return undefined;
        }

        // Seen on another device meanwhile: not again here.
        if (String(start) === seenRef.current) {
            setFocusSummary(null);
            return undefined;
        }
        let gone = false;
        Client4.getFocusSummary(start, end).then((found) => {
            if (gone) {
                return;
            }
            if (!found.channels.length && !found.threads.length) {
                toast(formatMessage({id: 'fusion.focus.endedNothing', defaultMessage: 'Focus ended: nothing new came in', description: 'Toast when focus mode ends and nothing arrived meanwhile.'}));
                markSeen(start);
                return;
            }
            dispatch(getMissingProfilesByIds([...found.channels.map((c) => c.last_sender_id), ...found.threads.map((t) => t.last_sender_id)].filter(Boolean)));
            setSummary(found);
        }).catch(() => {
            if (!gone) {
                setFocusSummary(null);
            }
        });
        return () => {
            gone = true;
        };

        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [start, end]);

    // Seen elsewhere while shown here: it goes here too.
    useEffect(() => {
        if (start && seen === String(start)) {
            setSummary(null);
            setFocusSummary(null);
        }
    }, [seen, start]);

    if (!span || !summary) {
        return null;
    }
    return (
        <FocusSummaryDialog
            span={span}
            summary={summary}
            onDone={async (markRead) => {
                if (markRead) {
                    try {
                        await Client4.markFocusSummaryRead(span.start_at, span.end_at);
                    } catch {
                        toast(formatMessage({id: 'fusion.focus.markFailed', defaultMessage: 'What you missed couldn\'t be marked as read', description: 'Toast when marking everything from the summary after focus mode as read failed.'}));
                    }
                }
                markSeen(span.start_at);
            }}
        />
    );
}
