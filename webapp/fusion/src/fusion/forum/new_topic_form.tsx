// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useRef, useState} from 'react';
import {useIntl} from 'react-intl';

import type {Channel, ChannelKindMode} from '@mattermost/types/channels';

import Composer from 'fusion/composer/composer';
import {THREAD_TITLE_PROP} from 'fusion/messages/thread_title';
import {am} from 'fusion/utils/class_names';

import {topicTitlePlaceholder} from './labels';

// A topic's title (design §3.1): 1 to 100 runes, trimmed.
export const TOPIC_TITLE_MAX = 100;

type Props = {
    channel: Channel;
    mode: ChannelKindMode;
    onClose: () => void;
};

// NewTopicForm opens at the top of the topics list (design §4.4): a title, Enter in it moves to the body, then the
// full composer (formatting, attachments, emoji). The server is the one place that enforces the title (strict
// everywhere, design §3.1): a title left empty still sends, and the composer shows the server's error inline.
export default function NewTopicForm({channel, mode, onClose}: Props) {
    const {formatMessage} = useIntl();
    const [title, setTitle] = useState('');
    const [titleError, setTitleError] = useState('');
    const titleRef = useRef<HTMLInputElement>(null);
    const bodyRef = useRef<HTMLDivElement>(null);
    const sendRef = useRef<() => void>(undefined);

    const focusBody = () => {
        bodyRef.current?.querySelector<HTMLElement>('textarea, [contenteditable="true"]')?.focus();
    };

    const sendFromTitle = () => {
        if (!title.trim()) {
            setTitleError(formatMessage({id: 'fusion.forum.titleRequired', defaultMessage: 'A topic needs a title.', description: 'The New topic form\'s title field, inline validation shown when it\'s left empty and Ctrl/Cmd+Enter is pressed there.'}));
            titleRef.current?.focus();
            return;
        }
        sendRef.current?.();
    };

    return (
        <div className={am('new-topic')}>
            <input
                ref={titleRef}
                type='text'
                className={am('new-topic-title')}
                value={title}
                maxLength={TOPIC_TITLE_MAX}
                required={true}
                placeholder={formatMessage(topicTitlePlaceholder(mode))}
                aria-label={formatMessage({id: 'fusion.forum.titleLabel', defaultMessage: 'Title', description: 'Label of the New topic form\'s title field.'})}
                onChange={(e) => {
                    setTitle(e.target.value);
                    if (titleError) {
                        setTitleError('');
                    }
                }}
                onKeyDown={(e) => {
                    if (e.key !== 'Enter' || e.nativeEvent.isComposing) {
                        return;
                    }
                    e.preventDefault();
                    if (e.ctrlKey || e.metaKey) {
                        sendFromTitle();
                    } else {
                        focusBody();
                    }
                }}
            />
            {titleError && (
                <div
                    className={am('field-error')}
                    role='alert'
                >
                    {titleError}
                </div>
            )}
            <div ref={bodyRef}>
                <Composer
                    channelId={channel.id}
                    placeholder={formatMessage({id: 'fusion.forum.bodyPlaceholder', defaultMessage: 'Write the first post…', description: 'Placeholder of the New topic form\'s message box.'})}
                    extraProps={{[THREAD_TITLE_PROP]: title.trim()}}
                    onReady={(submit) => {
                        sendRef.current = submit;
                    }}
                    afterSubmit={onClose}
                />
            </div>
            <button
                type='button'
                className={am('link-btn')}
                onClick={onClose}
            >
                {formatMessage({id: 'fusion.forum.cancelTopic', defaultMessage: 'Cancel', description: 'Closes the New topic form without posting.'})}
            </button>
        </div>
    );
}
