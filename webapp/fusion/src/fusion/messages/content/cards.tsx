// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {PostAction} from '@mattermost/types/integration_actions';
import type {MessageAttachment} from '@mattermost/types/message_attachments';
import {isMessageAttachmentArray} from '@mattermost/types/message_attachments';
import type {Post} from '@mattermost/types/posts';

import {doPostActionWithCookie} from 'mattermost-redux/actions/posts';
import {secureGetFromRecord} from 'mattermost-redux/utils/post_utils';

import {selectAttachmentMenuAction} from 'actions/views/posts';

import ExternalImage from 'components/external_image';
import ExternalLink from 'components/external_link';
import Markdown from 'components/markdown';
import ActionMenu from 'components/post_view/message_attachments/action_menu';

import {am} from 'fusion/utils/class_names';
import {decodeHtmlEntities} from 'utils/markdown/decode_html_entities';
import {isUrlSafe} from 'utils/url';

import type {GlobalState} from 'types/store';

import {useLightbox} from './lightbox';

// The colour bar of a card: a hex colour, or one of Mattermost's named colours.
const NAMED_COLORS: Record<string, string> = {
    good: 'var(--am-ok)',
    warning: 'var(--am-away)',
    danger: 'var(--am-danger)',
};
function cardColor(color?: string) {
    if (color && (/^#[0-9a-f]{3,8}$/i).test(color)) {
        return color;
    }
    return (color && NAMED_COLORS[color]) || 'var(--am-line)';
}

// The style of an action button: primary as in the mockup, Mattermost's status styles, or a hex colour.
function buttonStyle(style?: string): {className: string; color?: string} {
    switch (style) {
    case 'primary':
        return {className: am('primary')};
    case 'good':
    case 'success':
        return {className: am('good')};
    case 'warning':
        return {className: am('warning')};
    case 'danger':
        return {className: am('danger')};
    default:
        return {className: '', color: style && (/^#[0-9a-f]{3,6}$/i).test(style) ? style : undefined};
    }
}

function StaticSelect({post, action, disabled}: {post: Post; action: PostAction; disabled: boolean}) {
    const dispatch = useDispatch();
    const selected = useSelector((state: GlobalState) => (action.id ? secureGetFromRecord(state.views.posts.menuActions[post.id], action.id) : undefined));
    const initial = selected?.value ?? action.default_option ?? '';
    const id = `am-card-select-${post.id}-${action.id}`;
    return (
        <div className={am('card-select')}>
            <label htmlFor={id}>{action.name}</label>
            <select
                id={id}
                value={initial}
                disabled={disabled || action.disabled}
                onChange={(e) => {
                    const option = action.options?.find((o) => o.value === e.target.value);
                    if (option) {
                        dispatch(selectAttachmentMenuAction(post.id, action.id || '', action.cookie || '', action.data_source, option.text, option.value));
                    }
                }}
            >
                {!initial && (
                    <option
                        value=''
                        disabled={true}
                    >
                        {action.name}
                    </option>
                )}
                {action.options?.map((o) => (
                    <option
                        key={o.value}
                        value={o.value}
                    >
                        {o.text}
                    </option>
                ))}
            </select>
        </div>
    );
}

function Actions({post, actions}: {post: Post; actions: PostAction[]}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const [running, setRunning] = useState<string | null>(null);
    const [error, setError] = useState('');

    const run = async (action: PostAction) => {
        setRunning(action.id || '');
        setError('');
        const result = await dispatch(doPostActionWithCookie(post.id, action.id || '', action.cookie || ''));
        setRunning(null);
        if (result && 'error' in result && result.error) {
            setError((result.error as {message?: string}).message || formatMessage({id: 'fusion.card.failed', defaultMessage: 'The action failed.'}));
        }
    };

    const selects = actions.filter((a) => a.type === 'select' && a.id && a.name);
    const buttons = actions.filter((a) => a.type !== 'select' && a.id && a.name);
    return (
        <>
            {selects.map((action) => (action.data_source === 'users' || action.data_source === 'channels' ? (

                // People and channels are searched on the server, as in the classic web app.
                <div
                    key={action.id}
                    className={am('card-select')}
                >
                    <ActionMenu
                        postId={post.id}
                        action={action}
                        disabled={action.disabled}
                    />
                </div>
            ) : (
                <StaticSelect
                    key={action.id}
                    post={post}
                    action={action}
                    disabled={running !== null}
                />
            )))}
            {buttons.length > 0 && (
                <div className={am('card-actions')}>
                    {buttons.map((action) => {
                        const style = buttonStyle(action.style);
                        return (
                            <button
                                key={action.id}
                                type='button'
                                className={style.className || undefined}
                                style={style.color ? {color: style.color, borderColor: style.color} : undefined}
                                title={action.tooltip}
                                disabled={action.disabled || running !== null}
                                aria-busy={running === action.id}
                                onClick={() => run(action)}
                            >
                                {action.name}
                            </button>
                        );
                    })}
                </div>
            )}
            {error && (
                <div
                    className={am('card-note', 'error')}
                    role='alert'
                >
                    {error}
                </div>
            )}
        </>
    );
}

function Card({post, attachment}: {post: Post; attachment: MessageAttachment}) {
    const intl = useIntl();
    const [lightbox, openImage] = useLightbox();
    const images = post.metadata?.images;
    const author = attachment.author_name || attachment.author_icon ? (
        <>
            {attachment.author_icon && (
                <ExternalImage
                    src={attachment.author_icon}
                    imageMetadata={secureGetFromRecord(images, attachment.author_icon)}
                >
                    {(src) => (
                        <img
                            className={am('card-icon')}
                            src={src}
                            alt=''
                        />
                    )}
                </ExternalImage>
            )}
            {attachment.author_name && decodeHtmlEntities(attachment.author_name)}
        </>
    ) : null;

    let title = null;
    if (attachment.title) {
        title = attachment.title_link && isUrlSafe(attachment.title_link) ? (
            <ExternalLink
                className={am('card-title')}
                href={attachment.title_link}
                location='fusion_card'
            >
                {decodeHtmlEntities(attachment.title)}
            </ExternalLink>
        ) : (
            <span className={am('card-title', 'plain')}>{decodeHtmlEntities(attachment.title)}</span>
        );
    }

    const fields = (attachment.fields || []).filter((f) => f && (f.title || f.value !== undefined));
    const imageMeta = attachment.image_url ? secureGetFromRecord(images, attachment.image_url) : undefined;
    const footer = [attachment.footer, intl.formatTime(post.create_at, {hour: 'numeric', minute: '2-digit'})].filter(Boolean).join(' · ');

    return (
        <>
            {attachment.pretext && (
                <div className={am('card-pretext')}>
                    <Markdown
                        message={attachment.pretext}
                        postId={post.id}
                    />
                </div>
            )}
            <div
                className={am('card', {thumbed: Boolean(attachment.thumb_url)})}
                style={{'--am-cc': cardColor(attachment.color)} as React.CSSProperties}
            >
                {attachment.thumb_url && (
                    <ExternalImage
                        src={attachment.thumb_url}
                        imageMetadata={secureGetFromRecord(images, attachment.thumb_url)}
                    >
                        {(src) => (
                            <img
                                className={am('card-thumb')}
                                src={src}
                                alt=''
                            />
                        )}
                    </ExternalImage>
                )}
                {author && (
                    attachment.author_link && isUrlSafe(attachment.author_link) ? (
                        <ExternalLink
                            className={am('card-author')}
                            href={attachment.author_link}
                            location='fusion_card'
                        >
                            {author}
                        </ExternalLink>
                    ) : <div className={am('card-author')}>{author}</div>
                )}
                {title}
                {attachment.text && (
                    <div className={am('card-text')}>
                        <Markdown
                            message={attachment.text}
                            postId={post.id}
                        />
                    </div>
                )}
                {fields.length > 0 && (
                    <dl className={am('card-fields')}>
                        {fields.map((field, i) => (
                            <div
                                key={i}
                                className={am({wide: !field.short})}
                            >
                                <dt>{field.title}</dt>
                                <dd>
                                    <Markdown
                                        message={String(field.value ?? '')}
                                        postId={post.id}
                                    />
                                </dd>
                            </div>
                        ))}
                    </dl>
                )}
                {attachment.image_url && (
                    <ExternalImage
                        src={attachment.image_url}
                        imageMetadata={imageMeta}
                    >
                        {(src) => (
                            <button
                                type='button'
                                className={am('card-image')}
                                aria-label={intl.formatMessage({id: 'fusion.card.openImage', defaultMessage: 'Open image'})}
                                onClick={() => openImage({src, name: attachment.title || attachment.fallback || '', width: imageMeta?.width, height: imageMeta?.height})}
                            >
                                <img
                                    src={src}
                                    alt=''
                                    loading='lazy'
                                />
                            </button>
                        )}
                    </ExternalImage>
                )}
                {attachment.actions && attachment.actions.length > 0 && (
                    <Actions
                        post={post}
                        actions={attachment.actions}
                    />
                )}
                {(attachment.footer || attachment.footer_icon) && (
                    <div className={am('card-foot')}>
                        {attachment.footer_icon && (
                            <ExternalImage
                                src={attachment.footer_icon}
                                imageMetadata={secureGetFromRecord(images, attachment.footer_icon)}
                            >
                                {(src) => (
                                    <img
                                        className={am('card-icon')}
                                        src={src}
                                        alt=''
                                    />
                                )}
                            </ExternalImage>
                        )}
                        {footer}
                    </div>
                )}
            </div>
            {lightbox}
        </>
    );
}

// Cards draws a message's attachments (from integrations and webhooks) as the mockup's integration cards.
export default function Cards({post}: {post: Post}) {
    const raw = post.props?.attachments;
    const attachments = isMessageAttachmentArray(raw) ? raw : [];
    return (
        <>
            {attachments.map((attachment, i) => (
                <Card
                    key={i}
                    post={post}
                    attachment={attachment}
                />
            ))}
        </>
    );
}
