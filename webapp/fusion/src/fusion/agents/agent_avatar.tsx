// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useRef, useState} from 'react';
import {useIntl} from 'react-intl';

import type {AIAgentDetails} from '@mattermost/types/ai_agents';

import {Client4} from 'mattermost-redux/client';

import {errorMessage} from 'fusion/ai/connections';
import {am} from 'fusion/utils/class_names';

const MAX_BYTES = 10 * 1024 * 1024;

// AgentAvatar shows an agent's picture, and sets or resets it with the bot account's picture API.
export default function AgentAvatar({agent}: {agent: AIAgentDetails}) {
    const {formatMessage} = useIntl();
    const input = useRef<HTMLInputElement>(null);
    const [version, setVersion] = useState(agent.last_icon_update || 0);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');

    const upload = async (file: File | undefined) => {
        if (!file) {
            return;
        }
        if (!file.type.startsWith('image/') || file.size > MAX_BYTES) {
            setError(formatMessage({id: 'fusion.agents.avatarBad', defaultMessage: 'Pick an image of up to 10 MB.'}));
            return;
        }
        setBusy(true);
        setError('');
        try {
            await Client4.uploadProfileImage(agent.user_id, file);
            setVersion(Date.now());
        } catch (err) {
            setError(errorMessage(err, formatMessage({id: 'fusion.agents.avatarFailed', defaultMessage: 'The picture couldn\'t be saved.'})));
        } finally {
            setBusy(false);
            if (input.current) {
                input.current.value = '';
            }
        }
    };

    const reset = async () => {
        setBusy(true);
        setError('');
        try {
            await Client4.setDefaultProfileImage(agent.user_id);
            setVersion(Date.now());
        } catch (err) {
            setError(errorMessage(err, formatMessage({id: 'fusion.agents.avatarFailed', defaultMessage: 'The picture couldn\'t be saved.'})));
        } finally {
            setBusy(false);
        }
    };

    return (
        <div className={am('field')}>
            <span>{formatMessage({id: 'fusion.agents.avatar', defaultMessage: 'Picture'})}</span>
            <div className={am('agent-avatar')}>
                <img
                    src={Client4.getProfilePictureUrl(agent.user_id, version)}
                    alt=''
                />
                <input
                    ref={input}
                    type='file'
                    accept='image/png,image/jpeg,image/gif,image/webp'
                    hidden={true}
                    aria-label={formatMessage({id: 'fusion.agents.avatarFile', defaultMessage: 'Picture of {name}'}, {name: agent.display_name || agent.username})}
                    onChange={(e) => upload(e.target.files?.[0])}
                />
                <button
                    type='button'
                    className={am('btn', 'small')}
                    disabled={busy}
                    onClick={() => input.current?.click()}
                >
                    {formatMessage({id: 'fusion.agents.avatarUpload', defaultMessage: 'Upload a picture'})}
                </button>
                <button
                    type='button'
                    className={am('btn', 'small', 'danger-ghost')}
                    disabled={busy}
                    onClick={reset}
                >
                    {formatMessage({id: 'fusion.agents.avatarReset', defaultMessage: 'Reset'})}
                </button>
            </div>
            {error && (
                <small
                    className={am('field-note', 'bad')}
                    role='alert'
                >
                    {error}
                </small>
            )}
        </div>
    );
}
