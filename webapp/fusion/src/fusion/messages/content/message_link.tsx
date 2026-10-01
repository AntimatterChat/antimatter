// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {createContext, useContext, useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Post, PostPreviewMetadata} from '@mattermost/types/posts';

import {getPost as fetchPost} from 'mattermost-redux/actions/posts';
import {getDirectChannel} from 'mattermost-redux/selectors/entities/channels';
import {getPost} from 'mattermost-redux/selectors/entities/posts';
import {getTeammateNameDisplaySetting} from 'mattermost-redux/selectors/entities/preferences';
import {getUser} from 'mattermost-redux/selectors/entities/users';
import {displayUsername} from 'mattermost-redux/utils/user_utils';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';
import {permalinkPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';
import {getSiteURL} from 'utils/url';

import type {GlobalState} from 'types/store';

// The message whose text is being rendered, when the Fusion UI renders it: links to messages in it become chips.
// Elsewhere (classic panels and dialogs), they stay links.
export const MessageLinkContext = createContext<Post | null>(null);

const escape = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

// parseMessageLink reads a link to a message of this server: <site>/<team>/pl/<post id>.
export function parseMessageLink(href: string): {team: string; postId: string} | null {
    const site = getSiteURL();
    if (!site) {
        return null;
    }
    const m = new RegExp(`^${escape(site)}/([a-z0-9_-]+)/pl/([a-z0-9]{26})/?(?:[?#].*)?$`, 'i').exec(href);
    return m ? {team: m[1], postId: m[2]} : null;
}

// Missing posts are asked for once per page.
const requested = new Set<string>();

function Chip({team, postId, host}: {team: string; postId: string; host: Post}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const post = useSelector((state: GlobalState) => getPost(state, postId));
    const channel = useSelector((state: GlobalState) => (post ? getDirectChannel(state, post.channel_id) : undefined));
    const author = useSelector((state: GlobalState) => (post ? getUser(state, post.user_id) : undefined));
    const nameDisplay = useSelector(getTeammateNameDisplaySetting);
    const [missing, setMissing] = useState(false);

    // The message's own preview (when it's the first link of the message) knows where it is, even unloaded.
    const embed = host.metadata?.embeds?.find((e) => e.type === 'permalink' && (e.data as PostPreviewMetadata | undefined)?.post_id === postId)?.data as PostPreviewMetadata | undefined;

    useEffect(() => {
        if (post || embed || requested.has(postId)) {
            return;
        }
        requested.add(postId);
        (async () => {
            const result = await dispatch(fetchPost(postId));
            if (result && 'error' in result && result.error) {
                setMissing(true);
            }
        })();
    }, [post, embed, postId, dispatch]);

    if (missing) {
        return (
            <span className={am('msg-link', 'missing')}>
                <Icon name='chat'/>
                {formatMessage({id: 'fusion.msgLink.unknown', defaultMessage: 'Unknown message'})}
            </span>
        );
    }

    const type = channel?.type || embed?.channel_type;
    const direct = type === 'D' || type === 'G';
    const name = channel?.display_name || embed?.channel_display_name || '';
    let label = name;
    if (name && direct) {
        label = '@' + name;
    }
    const who = author ? displayUsername(author, nameDisplay) : '';

    return (
        <button
            type='button'
            className={am('msg-link')}
            title={who ? formatMessage({id: 'fusion.msgLink.title', defaultMessage: "Jump to {who}'s message in {where}"}, {who, where: direct ? label : '#' + label}) : formatMessage({id: 'fusion.msgLink.titleNoAuthor', defaultMessage: 'Jump to the message'})}
            onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                getHistory().push(permalinkPath(team, postId));
            }}
        >
            {!direct && name && <Icon name='hash'/>}
            {label || formatMessage({id: 'fusion.msgLink.message', defaultMessage: 'Message'})}
            <span className={am('sep')}>{'›'}</span>
            <Icon name='chat'/>
        </button>
    );
}

type Props = {
    attribs: Record<string, unknown>;
    children?: React.ReactNode;
};

// MessageLink is a link to a message, from the markdown renderer: the mockup's "#channel › 💬" chip in Fusion
// messages, the link as it was anywhere else (and for links with their own text).
export function MessageLink({attribs, children}: Props) {
    const host = useContext(MessageLinkContext);
    const href = typeof attribs.href === 'string' ? attribs.href : '';
    const parsed = href ? parseMessageLink(href) : null;

    // Only bare links become chips; a link with its own text keeps it.
    const text = React.Children.toArray(children).filter((c) => typeof c === 'string').join('').trim();
    const bare = !text || text === href || href.endsWith(text);
    if (!host || !parsed || !bare) {
        return <a {...attribs}>{children}</a>;
    }
    return (
        <Chip
            team={parsed.team}
            postId={parsed.postId}
            host={host}
        />
    );
}
