// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Post, PostPreviewMetadata} from '@mattermost/types/posts';

import {General} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getOpenGraphMetadataForUrl} from 'mattermost-redux/selectors/entities/posts';
import {getBool} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentRelativeTeamUrl} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId, getUser} from 'mattermost-redux/selectors/entities/users';

import {editPost} from 'actions/views/posts';

import ExternalImage from 'components/external_image';
import ExternalLink from 'components/external_link';
import {getBestImage} from 'components/post_view/post_attachment_opengraph/post_attachment_opengraph';
import PostMessageView from 'components/post_view/post_message_view';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {useDisplayName} from 'fusion/hooks/users';
import {am} from 'fusion/utils/class_names';
import {getHistory} from 'utils/browser_history';
import {PostTypes, Preferences} from 'utils/constants';
import {isSystemMessage} from 'utils/post_utils';
import {makeUrlSafe} from 'utils/url';
import {getVideoId, handleYoutubeTime} from 'utils/youtube';

import type {GlobalState} from 'types/store';

import {imageCaption, useLightbox} from './lightbox';

// OpenGraph is a link preview: the mockup's .og card (site, title, two lines of description, thumbnail).
export function OpenGraph({post, link}: {post: Post; link: string}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const og = useSelector((state: GlobalState) => getOpenGraphMetadataForUrl(state, post.id, link));
    const enabled = useSelector((state: GlobalState) => getConfig(state).EnableLinkPreviews === 'true');
    const wanted = useSelector((state: GlobalState) => getBool(state, Preferences.CATEGORY_DISPLAY_SETTINGS, Preferences.LINK_PREVIEW_DISPLAY, true));
    const me = useSelector(getCurrentUserId);

    if (!og || !enabled || !wanted || isSystemMessage(post) || post.props?.[PostTypes.REMOVE_LINK_PREVIEW] === 'true') {
        return null;
    }
    const image = getBestImage(og, post.metadata?.images);
    const imageUrl = image ? image.secure_url || image.url : '';
    const title = og.title || og.url || link;

    // The author can drop the preview, as in the classic web app.
    const remove = () => dispatch(editPost({id: post.id, props: {...post.props, [PostTypes.REMOVE_LINK_PREVIEW]: 'true'}} as Post));

    return (
        <div
            className={am('og', {'no-thumb': !imageUrl})}
            style={{'--am-oc': 'var(--am-anti)'} as React.CSSProperties}
        >
            <div>
                {og.site_name && <span className={am('site')}>{og.site_name}</span>}
                <ExternalLink
                    className={am('title')}
                    href={makeUrlSafe(og.url || link)}
                    location='fusion_opengraph'
                >
                    {title}
                </ExternalLink>
                {og.description && <span className={am('desc')}>{og.description}</span>}
            </div>
            {imageUrl && (
                <ExternalImage
                    src={imageUrl}
                    imageMetadata={post.metadata?.images?.[imageUrl]}
                >
                    {(src) => (
                        <span className={am('thumb')}>
                            <img
                                src={src}
                                alt=''
                                loading='lazy'
                            />
                        </span>
                    )}
                </ExternalImage>
            )}
            {post.user_id === me && (
                <button
                    type='button'
                    className={am('og-remove')}
                    title={formatMessage({id: 'fusion.embed.removePreview', defaultMessage: 'Remove link preview'})}
                    aria-label={formatMessage({id: 'fusion.embed.removePreview', defaultMessage: 'Remove link preview'})}
                    onClick={remove}
                >
                    <Icon
                        name='x'
                        size='xs'
                    />
                </button>
            )}
        </div>
    );
}

// YouTube is a YouTube link: the mockup's .embed.yt with its thumbnail, playing inline once clicked.
export function YouTube({post, link}: {post: Post; link: string}) {
    const {formatMessage} = useIntl();
    const og = useSelector((state: GlobalState) => getOpenGraphMetadataForUrl(state, post.id, link));
    const referrer = useSelector((state: GlobalState) => getConfig(state).YoutubeReferrerPolicy === 'true');
    const [playing, setPlaying] = useState(false);
    const [maxRes, setMaxRes] = useState(true);
    const id = getVideoId(link);
    if (!id) {
        return null;
    }
    const title = og?.title || link;
    const thumb = `https://img.youtube.com/vi/${id}/${maxRes ? 'maxresdefault' : 'hqdefault'}.jpg`;

    return (
        <div className={am('embed', 'yt')}>
            <span className={am('e-site')}>{'YouTube'}</span>
            <ExternalLink
                className={am('e-title')}
                href={link}
                location='fusion_youtube'
            >
                {title}
            </ExternalLink>
            {og?.description && <span className={am('e-meta')}>{og.description}</span>}
            {playing ? (
                <div className={am('yt-player')}>
                    <iframe
                        src={`https://www.youtube.com/embed/${id}?autoplay=1&rel=0&fs=1&enablejsapi=1${handleYoutubeTime(link)}`}
                        title={title}
                        allow='accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture'
                        allowFullScreen={true}
                        referrerPolicy={referrer ? 'origin' : undefined}
                        sandbox='allow-scripts allow-same-origin allow-popups allow-presentation'
                    />
                </div>
            ) : (
                <button
                    type='button'
                    className={am('yt-thumb')}
                    aria-label={formatMessage({id: 'fusion.embed.play', defaultMessage: 'Play {title}'}, {title})}
                    onClick={() => setPlaying(true)}
                >
                    <ExternalImage src={thumb}>
                        {(src) => (
                            <img
                                src={src}
                                alt=''
                                loading='lazy'
                                onError={() => setMaxRes(false)}
                            />
                        )}
                    </ExternalImage>
                    <span
                        className={am('yt-play')}
                        aria-hidden='true'
                    />
                </button>
            )}
        </div>
    );
}

// Permalink previews a linked message: the mockup's .permalink card (author, "in #channel · time", Jump).
export function Permalink({data}: {data: PostPreviewMetadata}) {
    const intl = useIntl();
    const teamUrl = useSelector(getCurrentRelativeTeamUrl);
    const preview = data.post;
    const user = useSelector((state: GlobalState) => (preview ? getUser(state, preview.user_id) : undefined));
    const name = useDisplayName(user);
    if (!preview) {
        return null;
    }
    const direct = data.channel_type === General.DM_CHANNEL || data.channel_type === General.GM_CHANNEL;
    const where = direct ? data.channel_display_name : '#' + data.channel_display_name;
    const jump = () => getHistory().push(`${direct ? teamUrl : '/' + data.team_name}/pl/${data.post_id}`);
    const author = (preview.props?.override_username as string | undefined) || name;

    return (
        <div
            className={am('permalink')}
            onClick={(e) => {
                // Links in the quoted message keep working.
                if (!(e.target as Element).closest('a')) {
                    jump();
                }
            }}
        >
            <span className={am('who')}>
                <Avatar
                    userId={preview.user_id}
                    size='xs'
                />
                <span>{author}</span>
                <span className={am('where')}>
                    {intl.formatMessage({id: 'fusion.embed.permalinkWhere', defaultMessage: 'in {where} · {time}'}, {where, time: intl.formatDate(preview.create_at, {month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit'})})}
                </span>
                <span className={am('grow')}/>
                <button
                    type='button'
                    className={am('jump')}
                    onClick={(e) => {
                        e.stopPropagation();
                        jump();
                    }}
                >
                    {intl.formatMessage({id: 'fusion.embed.jump', defaultMessage: 'Jump'})}
                </button>
            </span>
            <span className={am('ptext')}>
                <PostMessageView
                    post={preview}
                    isRHS={false}
                    isChannelAutotranslated={false}
                    userLanguage={intl.locale}
                    disableInteractions={true}
                    showPostEditedIndicator={false}
                />
            </span>
        </div>
    );
}

// InlineImage is an image linked in a message: a .img-att tile that opens the lightbox.
export function InlineImage({post, link}: {post: Post; link: string}) {
    const {formatMessage} = useIntl();
    const [lightbox, openImage] = useLightbox();
    const meta = post.metadata?.images?.[link];
    const name = decodeURIComponent(link.split(/[?#]/)[0].split('/').pop() || '') || link;
    const ratio = meta && meta.width > 0 && meta.height > 0 ? `${meta.width} / ${meta.height}` : undefined;

    return (
        <ExternalImage
            src={link}
            imageMetadata={meta}
        >
            {(src) => (src ? (
                <>
                    <button
                        type='button'
                        className={am('img-att')}
                        style={ratio ? {aspectRatio: ratio} : undefined}
                        aria-label={formatMessage({id: 'fusion.files.openImage', defaultMessage: 'Open image {name}'}, {name})}
                        onClick={() => openImage({src, name, width: meta?.width, height: meta?.height})}
                    >
                        <img
                            src={src}
                            alt=''
                            loading='lazy'
                        />
                        <span>{imageCaption({src, name})}</span>
                    </button>
                    {lightbox}
                </>
            ) : null)}
        </ExternalImage>
    );
}
