// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {isAppBinding} from '@mattermost/types/apps';
import type {AppBinding} from '@mattermost/types/apps';
import type {Post, PostEmbed, PostPreviewMetadata} from '@mattermost/types/posts';
import {isArrayOf} from '@mattermost/types/utilities';

import {Posts} from 'mattermost-redux/constants';
import {appsEnabled} from 'mattermost-redux/selectors/entities/apps';
import {getFeatureFlagValue} from 'mattermost-redux/selectors/entities/general';
import {validateBindings} from 'mattermost-redux/utils/apps';
import {getEmbedFromMetadata} from 'mattermost-redux/utils/post_utils';

import {toggleEmbedVisibility} from 'actions/post_actions';
import {isEmbedVisible} from 'selectors/posts';

import {hasInteractiveMessageProps} from 'components/block_renderer/translation';
import EmbeddedBindings from 'components/post_view/embedded_bindings/embedded_bindings';
import InteractiveMessages from 'components/post_view/interactive_messages';
import PostMessageView from 'components/post_view/post_message_view';

import webSocketClient from 'client/web_websocket_client';
import {am} from 'fusion/utils/class_names';
import PluggableErrorBoundary from 'plugins/pluggable/error_boundary';
import {ytRegex} from 'utils/youtube';

import type {GlobalState} from 'types/store';

import Cards from './cards';
import {InlineImage, OpenGraph, Permalink, YouTube} from './embeds';

type Props = {
    post: Post;
    inThread: boolean;
    autotranslated: boolean;
};

const isYouTube = (url: string) => Boolean(url.trim().match(ytRegex));

// Embed is what Mattermost attached to a message's text: a plugin's embed, integration cards, a link preview, a
// YouTube video, a linked message or an image, each drawn in the mockup's markup.
function Embed({post, embed}: {post: Post; embed: PostEmbed}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const visible = useSelector((state: GlobalState) => isEmbedVisible(state, post.id));
    const pluginEmbeds = useSelector((state: GlobalState) => state.plugins.components.PostWillRenderEmbedComponent);

    for (const c of pluginEmbeds || []) {
        if (c.match(embed)) {
            const Component = c.component;
            return visible ? (
                <PluggableErrorBoundary pluginId={c.pluginId}>
                    <Component
                        embed={embed}
                        webSocketClient={webSocketClient}
                    />
                </PluggableErrorBoundary>
            ) : null;
        }
    }

    // Images and videos follow the "collapse previews" setting; a click shows them.
    const collapsible = embed.type === 'image' || (embed.type === 'opengraph' && isYouTube(embed.url));
    if (collapsible && !visible) {
        return (
            <button
                type='button'
                className={am('embed-toggle')}
                onClick={() => dispatch(toggleEmbedVisibility(post.id))}
            >
                {formatMessage({id: 'fusion.embed.show', defaultMessage: 'Show preview'})}
            </button>
        );
    }

    switch (embed.type) {
    case 'message_attachment':
        return <Cards post={post}/>;
    case 'image':
        return (
            <InlineImage
                post={post}
                link={embed.url}
            />
        );
    case 'opengraph':
        return isYouTube(embed.url) ? (
            <YouTube
                post={post}
                link={embed.url}
            />
        ) : (
            <OpenGraph
                post={post}
                link={embed.url}
            />
        );
    case 'permalink':
        return embed.data && 'post_id' in embed.data && embed.data.post_id ? <Permalink data={embed.data as PostPreviewMetadata}/> : null;
    default:
        return null;
    }
}

// MessageContent is a message's body: its text, rendered by the classic markdown renderer (or by a plugin for its
// own message types), then what is attached to it.
export default function MessageContent({post, inThread, autotranslated}: Props) {
    const {locale} = useIntl();
    const pluginPostTypes = useSelector((state: GlobalState) => state.plugins.postTypes);
    const apps = useSelector(appsEnabled);
    const blocks = useSelector((state: GlobalState) => getFeatureFlagValue(state, 'MmBlocksEnabled') === 'true');

    const text = (
        <PostMessageView
            post={post}
            isRHS={inThread}
            isChannelAutotranslated={autotranslated}
            userLanguage={locale}
            showPostEditedIndicator={false}
        />
    );
    if (post.state === Posts.POST_DELETED || (post.type && Object.hasOwn(pluginPostTypes, post.type))) {
        return text;
    }

    // Mattermost's newer interactive messages and Apps bindings keep their own renderers.
    if (blocks && hasInteractiveMessageProps(post.props as Record<string, unknown>)) {
        return (
            <>
                {text}
                <InteractiveMessages post={post}/>
            </>
        );
    }
    const bindings = apps && isArrayOf<AppBinding>(post.props?.app_bindings, isAppBinding) ? validateBindings(post.props?.app_bindings as AppBinding[]) : [];
    if (bindings.length) {
        return (
            <>
                {text}
                <EmbeddedBindings
                    embeds={bindings}
                    post={post}
                />
            </>
        );
    }

    const embed = getEmbedFromMetadata(post.metadata);
    return (
        <>
            {text}
            {embed && (
                <Embed
                    post={post}
                    embed={embed}
                />
            )}
        </>
    );
}
