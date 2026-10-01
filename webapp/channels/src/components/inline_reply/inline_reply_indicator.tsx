// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import classNames from 'classnames';
import React from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useSelector} from 'react-redux';

import {CloseIcon, ReplyOutlineIcon} from '@mattermost/compass-icons/components';
import {WithTooltip} from '@mattermost/shared/components/tooltip';

import {getCurrentUserId, getUser} from 'mattermost-redux/selectors/entities/users';

import {getQuotedMessage} from 'utils/inline_replies';
import {getDisplayNameByUser} from 'utils/utils';

import type {GlobalState} from 'types/store';

import QuotedMessage from './quoted_message';

import './inline_reply.scss';

type Props = {
    postId: string;
    onCancel: () => void;

    // Whether the reply notifies the quoted message's author, and how to change it.
    mention: boolean;
    onMentionChange: (mention: boolean) => void;
};

// InlineReplyIndicator tells, above the message box, which message the message being written replies to.
export default function InlineReplyIndicator({postId, onCancel, mention, onMentionChange}: Props) {
    const {formatMessage} = useIntl();
    const cancelLabel = formatMessage({id: 'inline_reply.cancel', defaultMessage: 'Cancel reply'});

    return (
        <div
            className='InlineReplyIndicator'
            data-testid='inline-reply-indicator'
        >
            <div className='InlineReplyQuote'>
                <ReplyOutlineIcon
                    size={14}
                    className='InlineReplyQuote__icon'
                />
                <QuotedMessage postId={postId}/>
            </div>
            <MentionToggle
                postId={postId}
                mention={mention}
                onChange={onMentionChange}
            />
            <WithTooltip title={cancelLabel}>
                <button
                    type='button'
                    className='InlineReplyIndicator__cancel'
                    aria-label={cancelLabel}
                    onClick={(e) => {
                        e.preventDefault();
                        onCancel();
                    }}
                >
                    <CloseIcon size={14}/>
                </button>
            </WithTooltip>
        </div>
    );
}

type MentionToggleProps = {
    postId: string;
    mention: boolean;
    onChange: (mention: boolean) => void;
};

// MentionToggle is the "@ On" / "@ Off" switch telling whether the reply notifies the quoted message's author. It
// isn't shown when no one would be notified: replying to yourself, to a deleted message or to a webhook's message.
function MentionToggle({postId, mention, onChange}: MentionToggleProps) {
    const {formatMessage} = useIntl();
    const quoted = useSelector((state: GlobalState) => getQuotedMessage(state, postId), shallowEqual);
    const currentUserId = useSelector(getCurrentUserId);
    const name = useSelector((state: GlobalState) => (quoted.userId ? getDisplayNameByUser(state, getUser(state, quoted.userId)) : ''));

    if (!quoted.userId || quoted.userId === currentUserId || quoted.deleted || quoted.fromWebhook) {
        return null;
    }

    const label = mention ? formatMessage({id: 'inline_reply.mention', defaultMessage: 'Mention {name}'}, {name}) : formatMessage({id: 'inline_reply.noMention', defaultMessage: "Don't mention {name}"}, {name});
    const tooltip = mention ? formatMessage({id: 'inline_reply.mention.tooltip', defaultMessage: '{name} will be notified of your reply. Click to turn off.'}, {name}) : formatMessage({id: 'inline_reply.noMention.tooltip', defaultMessage: "{name} won't be notified of your reply. Click to turn on."}, {name});

    return (
        <WithTooltip title={tooltip}>
            <button
                type='button'
                className={classNames('InlineReplyIndicator__mention', {'InlineReplyIndicator__mention--off': !mention})}
                aria-label={label}
                onClick={(e) => {
                    e.preventDefault();
                    onChange(!mention);
                }}
            >
                {mention ? formatMessage({id: 'inline_reply.mention.on', defaultMessage: '@ On'}) : formatMessage({id: 'inline_reply.mention.off', defaultMessage: '@ Off'})}
            </button>
        </WithTooltip>
    );
}
