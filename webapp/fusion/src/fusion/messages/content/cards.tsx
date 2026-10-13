// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {PostAction} from '@mattermost/types/integration_actions';
import type {MessageAttachment, MessageAttachmentChart, MessageAttachmentStage} from '@mattermost/types/message_attachments';
import {isMessageAttachmentArray} from '@mattermost/types/message_attachments';
import type {Post} from '@mattermost/types/posts';

import {doPostActionWithCookie} from 'mattermost-redux/actions/posts';
import {secureGetFromRecord} from 'mattermost-redux/utils/post_utils';

import {selectAttachmentMenuAction} from 'actions/views/posts';

import ExternalImage from 'components/external_image';
import ExternalLink from 'components/external_link';
import Markdown from 'components/markdown';
import ActionMenu from 'components/post_view/message_attachments/action_menu';

import Icon from 'fusion/components/icon/icon';
import type {IconName} from 'fusion/components/icon/icon';
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

// A pipeline's stages as pills, as in the mockup's CI cards. Stages of unknown states show as pending.
const STAGE_ICONS: Record<string, IconName> = {passed: 'check', failed: 'x', running: 'clock', pending: 'clock', skipped: 'chev', canceled: 'x'};
const STAGES_MAX = 20;

function Stages({stages}: {stages: MessageAttachmentStage[]}) {
    const {formatMessage} = useIntl();
    const shown = stages.filter((s) => s && typeof s.name === 'string' && s.name).slice(0, STAGES_MAX);
    if (!shown.length) {
        return null;
    }
    return (
        <div
            className={am('card-stages')}
            role='list'
            aria-label={formatMessage({id: 'fusion.card.stages', defaultMessage: 'Stages'})}
        >
            {shown.map((stage, i) => {
                const status = STAGE_ICONS[stage.status] ? stage.status : 'pending';
                return (
                    <span
                        key={i}
                        className={am('stage-pill', status)}
                        role='listitem'
                        title={`${stage.name}: ${status}`}
                    >
                        <Icon name={STAGE_ICONS[status]}/>
                        {stage.name}
                    </span>
                );
            })}
        </div>
    );
}

// sparkline gives the points of a chart drawn in a w×h box: its line, the area under it, and where the threshold
// sits. The scale includes the threshold, with a margin, and starts at 0 for values that never go below it.
export function sparkline(values: number[], threshold: number | undefined, w: number, h: number) {
    const all = threshold === undefined ? values : [...values, threshold];
    const hi = Math.max(...all);
    const lo = Math.min(...all);
    const pad = (hi - lo) * 0.15 || 1;
    const max = hi + pad;
    const min = lo >= 0 ? Math.max(0, lo - pad) : lo - pad;
    const x = (i: number) => ((i / (values.length - 1)) * (w - 16)) + 8;
    const y = (v: number) => h - 8 - (((v - min) / (max - min)) * (h - 16));
    const points = values.map((v, i) => `${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(' ');
    return {
        points,
        area: `8,${h - 8} ${points} ${x(values.length - 1).toFixed(1)},${h - 8}`,
        last: {x: x(values.length - 1), y: y(values[values.length - 1])},
        threshold: threshold === undefined ? undefined : y(threshold),
    };
}

const CHART_W = 460;
const CHART_H = 72;
const CHART_VALUES_MAX = 200;

// Sparkline draws a chart of recent values in the card's colour, with the threshold as a dashed red line, as in the
// mockup's monitoring cards.
function Sparkline({chart, color}: {chart: MessageAttachmentChart; color: string}) {
    const {formatMessage} = useIntl();
    const values = Array.isArray(chart.values) ? chart.values.filter((v) => typeof v === 'number' && Number.isFinite(v)).slice(-CHART_VALUES_MAX) : [];
    if (values.length < 2) {
        return null;
    }
    const threshold = typeof chart.threshold === 'number' && Number.isFinite(chart.threshold) ? chart.threshold : undefined;
    const line = sparkline(values, threshold, CHART_W, CHART_H);
    const label = typeof chart.label === 'string' && chart.label ? chart.label : formatMessage({id: 'fusion.card.chart', defaultMessage: 'Chart'});
    return (
        <svg
            className={am('card-chart')}
            viewBox={`0 0 ${CHART_W} ${CHART_H}`}
            preserveAspectRatio='none'
            role='img'
            aria-label={label}
        >
            <title>{label}</title>
            <polygon
                points={line.area}
                style={{fill: color}}
                fillOpacity='.14'
            />
            {line.threshold !== undefined && (
                <line
                    x1='8'
                    x2={CHART_W - 8}
                    y1={line.threshold}
                    y2={line.threshold}
                    className={am('card-threshold')}
                    strokeWidth='1.5'
                    strokeDasharray='5 4'
                    vectorEffect='non-scaling-stroke'
                />
            )}
            <polyline
                points={line.points}
                fill='none'
                style={{stroke: color}}
                strokeWidth='2.2'
                strokeLinejoin='round'
                vectorEffect='non-scaling-stroke'
            />
            <circle
                cx={line.last.x}
                cy={line.last.y}
                r='3.5'
                style={{fill: color}}
            />
        </svg>
    );
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

    // A field shows when it has a title or a value: a title with an empty value still shows, an empty field doesn't.
    const fieldValue = (value: unknown) => (value === undefined || value === null ? '' : String(value));
    const fields = (attachment.fields || []).filter((f) => f && (f.title || fieldValue(f.value)));
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
                                {fieldValue(field.value) && (
                                    <dd>
                                        <Markdown
                                            message={fieldValue(field.value)}
                                            postId={post.id}
                                        />
                                    </dd>
                                )}
                            </div>
                        ))}
                    </dl>
                )}
                {Array.isArray(attachment.stages) && <Stages stages={attachment.stages}/>}
                {attachment.chart && typeof attachment.chart === 'object' && (
                    <Sparkline
                        chart={attachment.chart}
                        color={cardColor(attachment.color)}
                    />
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
                                onClick={() => openImage({src, name: attachment.title || attachment.fallback || '', width: imageMeta?.width, height: imageMeta?.height, userId: post.user_id, at: post.create_at})}
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
