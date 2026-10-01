// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {forwardRef, useEffect, useImperativeHandle, useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';
import type {Group} from '@mattermost/types/groups';
import type {UserProfile} from '@mattermost/types/users';

import Permissions from 'mattermost-redux/constants/permissions';
import {getDefaultAgent} from 'mattermost-redux/selectors/entities/agents';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getAssociatedGroupsForReference} from 'mattermost-redux/selectors/entities/groups';
import {makeGetProfilesForThread} from 'mattermost-redux/selectors/entities/posts';
import {haveIChannelPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';
import type {ActionResult} from 'mattermost-redux/types/actions';

import {autocompleteChannels} from 'actions/channel_actions';
import {autocompleteUsersInChannel} from 'actions/views/channel';
import {searchAssociatedGroupsForReference} from 'actions/views/group';

import RenderEmoji from 'components/emoji/render_emoji';
import AtMentionProvider from 'components/suggestion/at_mention_provider';
import ChannelMentionProvider from 'components/suggestion/channel_mention_provider';
import CommandProvider from 'components/suggestion/command_provider/command_provider';
import EmoticonProvider from 'components/suggestion/emoticon_provider';
import type Provider from 'components/suggestion/provider';
import {flattenItems, flattenTerms} from 'components/suggestion/suggestion_results';

import Avatar from 'fusion/components/avatar';
import ChannelIcon from 'fusion/sidebar/channel_icon';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

export type AutocompleteHandle = {

    // handleKeyDown lets the list take arrows, Enter, Tab and Escape while it's open; true when it did.
    handleKeyDown: (e: React.KeyboardEvent) => boolean;
};

type Props = {
    channelId: string;
    rootId: string;
    value: string;
    textareaRef: React.RefObject<HTMLTextAreaElement | null>;
    onChange: (value: string) => void;
};

type Kind = 'mention' | 'channel' | 'emoji' | 'command';
type Row = {term: string; item: unknown};

const SPECIAL_MENTIONS = ['here', 'channel', 'all'];
const SPECIAL_DESCRIPTIONS: Record<string, {id: string; defaultMessage: string}> = {
    here: {id: 'fusion.ac.here', defaultMessage: 'Notifies everyone online in this channel'},
    channel: {id: 'fusion.ac.channel', defaultMessage: 'Notifies everyone in this channel'},
    all: {id: 'fusion.ac.all', defaultMessage: 'Notifies everyone in this channel'},
};

function RowContent({kind, row}: {kind: Kind; row: Row}) {
    const {formatMessage} = useIntl();
    const item = row.item as Record<string, unknown>;
    if (kind === 'mention' && item && typeof item.username === 'string' && item.id) {
        const user = item as unknown as UserProfile;
        if (SPECIAL_MENTIONS.includes(user.username)) {
            return (
                <>
                    <span className={am('cmd-ic')}>{'@'}</span>
                    <span className={am('main')}>{`@${user.username}`}</span>
                    <span className={am('desc')}>{formatMessage(SPECIAL_DESCRIPTIONS[user.username])}</span>
                </>
            );
        }
        return (
            <>
                <Avatar
                    userId={user.id}
                    size='sm'
                    status={true}
                />
                <span className={am('main')}>{`@${user.username}`}</span>
                <span className={am('desc')}>
                    {[user.first_name, user.last_name].filter(Boolean).join(' ') || user.nickname}
                    {user.is_bot ? ' · ' + formatMessage({id: 'fusion.ac.bot', defaultMessage: 'bot'}) : ''}
                </span>
            </>
        );
    }

    // A user group (@group).
    if (kind === 'mention' && item && typeof item.name === 'string' && typeof item.display_name === 'string') {
        return (
            <>
                <span className={am('cmd-ic')}>{'@'}</span>
                <span className={am('main')}>{`@${item.name}`}</span>
                <span className={am('desc')}>{item.display_name}</span>
            </>
        );
    }
    if (kind === 'channel' && item && typeof item.channel === 'object') {
        const channel = item.channel as Channel;
        return (
            <>
                <span className={am('cmd-ic')}><ChannelIcon channel={channel}/></span>
                <span className={am('main')}>{`~${channel.display_name || channel.name}`}</span>
                <span className={am('desc')}>{channel.purpose || channel.header || ''}</span>
            </>
        );
    }
    if (kind === 'emoji' && item && typeof item.name === 'string') {
        return (
            <>
                <span className={am('glyph')}>
                    <RenderEmoji
                        emojiName={item.name}
                        size={18}
                    />
                </span>
                <span className={am('main')}>{`:${item.name}:`}</span>
            </>
        );
    }
    if (kind === 'command' && item && (typeof item.Suggestion === 'string' || typeof item.Complete === 'string')) {
        return (
            <>
                <span className={am('cmd-ic')}>{'/'}</span>
                <span className={am('main')}>{String(item.Suggestion || item.Complete)}</span>
                <span className={am('hint')}>{String(item.Hint || '')}</span>
                <span className={am('desc')}>
                    {typeof item.IconData === 'string' && (/^(data:|https?:)/).test(item.IconData) && (
                        <img
                            className={am('plug-ic')}
                            src={item.IconData}
                            alt=''
                        />
                    )}
                    {String(item.Description || '')}
                </span>
            </>
        );
    }
    return <span className={am('main')}>{row.term}</span>;
}

// Autocomplete suggests people (@), channels (~), commands (/) and emoji (:) as you type, with the classic web app's
// suggestion providers, drawn as the mockup's list above the composer.
const Autocomplete = forwardRef<AutocompleteHandle, Props>(({channelId, rootId, value, textareaRef, onChange}, ref) => {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const getProfilesForThread = useMemo(() => makeGetProfilesForThread(), []);
    const currentUserId = useSelector(getCurrentUserId);
    const teamId = useSelector(getCurrentTeamId);
    const useChannelMentions = useSelector((state: GlobalState) => haveIChannelPermission(state, teamId, channelId, Permissions.USE_CHANNEL_MENTIONS));
    const autocompleteGroups = useSelector((state: GlobalState) => (haveIChannelPermission(state, teamId, channelId, Permissions.USE_GROUP_MENTIONS) ? getAssociatedGroupsForReference(state, teamId, channelId) : null));
    const priorityProfiles = useSelector((state: GlobalState) => getProfilesForThread(state, rootId));
    const delayChannelAutocomplete = useSelector((state: GlobalState) => getConfig(state).DelayChannelAutocomplete === 'true');
    const defaultAgent = useSelector(getDefaultAgent);
    const [results, setResults] = useState<{kind: Kind; matchedPretext: string; rows: Row[]} | null>(null);
    const [selected, setSelected] = useState(0);
    const dismissedFor = useRef<string | null>(null);

    const providers = useMemo<Array<[Kind, Provider]>>(() => [
        ['command', new CommandProvider({teamId, channelId, rootId})],
        ['mention', new AtMentionProvider({
            currentUserId,
            channelId,
            autocompleteUsersInChannel: (prefix: string) => dispatch(autocompleteUsersInChannel(prefix, channelId)),
            useChannelMentions,
            autocompleteGroups,
            searchAssociatedGroupsForReference: (prefix: string) => dispatch(searchAssociatedGroupsForReference(prefix, teamId, channelId)) as Promise<ActionResult<Group[]>>,
            priorityProfiles,
            defaultAgent,
        })],
        ['channel', new ChannelMentionProvider((term, success, error) => dispatch(autocompleteChannels(term, success, error)), delayChannelAutocomplete)],
        ['emoji', new EmoticonProvider()],

        // eslint-disable-next-line react-hooks/exhaustive-deps
    ], [teamId, channelId, rootId, currentUserId, useChannelMentions, delayChannelAutocomplete]);

    // Ask the providers about the text before the caret whenever it changes.
    useEffect(() => {
        const el = textareaRef.current;
        const pretext = value.slice(0, el ? el.selectionEnd : value.length);
        if (dismissedFor.current === pretext) {
            return;
        }
        dismissedFor.current = null;
        let handled = false;
        for (const [kind, provider] of providers) {
            handled = provider.handlePretextChanged(pretext, (res) => {
                const terms = flattenTerms(res);
                const items = flattenItems(res);
                const rows = terms.map((term, i) => ({term, item: items[i]})).filter((r) => r.item && !(typeof r.item === 'object' && r.item !== null && 'loading' in (r.item as object)));
                setResults(rows.length ? {kind, matchedPretext: res.matchedPretext, rows: rows.slice(0, 12)} : null);
                setSelected(0);
            });
            if (handled) {
                break;
            }
        }
        if (!handled) {
            setResults(null);
        }
    }, [value, providers, textareaRef]);

    const choose = (row: Row) => {
        const el = textareaRef.current;
        if (!el || !results) {
            return;
        }
        const caret = el.selectionEnd;
        const pretext = value.slice(0, caret);
        const {matchedPretext} = results;
        const prefix = pretext.toLowerCase().endsWith(matchedPretext.toLowerCase()) ? pretext.slice(0, pretext.length - matchedPretext.length) : pretext;
        const term = row.term.replace(/(EXECUTE_CURRENT_COMMAND_ITEM_ID|OPEN_COMMAND_IN_MODAL_ITEM_ID)$/, '');
        const next = prefix + term + ' ' + value.slice(caret);
        onChange(next);
        setResults(null);
        requestAnimationFrame(() => {
            el.focus();
            el.setSelectionRange(prefix.length + term.length + 1, prefix.length + term.length + 1);
        });
    };

    useImperativeHandle(ref, () => ({
        handleKeyDown: (e: React.KeyboardEvent) => {
            if (!results) {
                return false;
            }
            if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
                e.preventDefault();
                const n = results.rows.length;
                setSelected((s) => (s + (e.key === 'ArrowDown' ? 1 : n - 1)) % n);
                return true;
            }
            if ((e.key === 'Enter' && !e.shiftKey) || e.key === 'Tab') {
                e.preventDefault();
                choose(results.rows[selected]);
                return true;
            }
            if (e.key === 'Escape') {
                e.preventDefault();
                const el = textareaRef.current;
                dismissedFor.current = value.slice(0, el ? el.selectionEnd : value.length);
                setResults(null);
                return true;
            }
            return false;
        },
    }));

    if (!results) {
        return null;
    }
    const title = {
        mention: formatMessage({id: 'fusion.ac.people', defaultMessage: 'People'}),
        channel: formatMessage({id: 'fusion.ac.channels', defaultMessage: 'Channels'}),
        emoji: formatMessage({id: 'fusion.ac.emoji', defaultMessage: 'Emoji'}),
        command: formatMessage({id: 'fusion.ac.commands', defaultMessage: 'Commands'}),
    }[results.kind];
    return (
        <div
            className={am('ac-pop')}
            role='listbox'
            aria-label={title}
        >
            <h5>{title}</h5>
            {results.rows.map((row, i) => (
                <button
                    key={row.term + i}
                    type='button'
                    className={am('ac-item', {sel: i === selected})}
                    role='option'
                    aria-selected={i === selected}
                    onMouseDown={(e) => {
                        e.preventDefault();
                        choose(row);
                    }}
                    onMouseEnter={() => setSelected(i)}
                >
                    <RowContent
                        kind={results.kind}
                        row={row}
                    />
                </button>
            ))}
        </div>
    );
});
Autocomplete.displayName = 'Autocomplete';

export default Autocomplete;
