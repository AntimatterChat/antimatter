// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

import {CloseIcon, ReplyOutlineIcon} from '@mattermost/compass-icons/components';
import {WithTooltip} from '@mattermost/shared/components/tooltip';

import QuotedMessage from './quoted_message';

import './inline_reply.scss';

type Props = {
    postId: string;
    onCancel: () => void;
};

// InlineReplyIndicator tells, above the message box, which message the message being written replies to.
export default function InlineReplyIndicator({postId, onCancel}: Props) {
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
