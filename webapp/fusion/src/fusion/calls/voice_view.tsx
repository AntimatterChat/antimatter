// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

import type {Channel} from '@mattermost/types/channels';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

import {useCallActions} from './actions';
import CallControls from './controls';
import {useMyCall, useParticipants} from './hooks';
import Stage from './stage';

// VoiceView is a voice channel's main view, the mockup's voiceHTML: the room's stage, with your controls once you're
// in it or a button to join it. Its text chat opens in the right-hand panel (see VoiceChatToggle).
export default function VoiceView({channel}: {channel: Channel}) {
    const {formatMessage} = useIntl();
    const actions = useCallActions();
    const my = useMyCall();
    const count = useParticipants(channel.id).length;
    const joined = my?.call.channelId === channel.id;

    return (
        <div className={am('view', 'voice-view')}>
            <div className={am('voice')}>
                <Stage
                    channelId={channel.id}
                    fullscreen={true}
                />
                {joined && my ? (
                    <CallControls
                        my={my}
                        withPopout={true}
                    />
                ) : (
                    <div className={am('voice-controls')}>
                        <button
                            className={am('btn', 'anti')}
                            style={{height: 44, padding: '0 22px'}}
                            onClick={() => actions.joinVoice(channel.id)}
                        >
                            <Icon
                                name='speaker'
                                size='sm'
                            />
                            {count ? formatMessage({id: 'fusion.calls.joinVoiceCount', defaultMessage: 'Join voice · {count} in the call'}, {count}) : formatMessage({id: 'fusion.calls.joinVoice', defaultMessage: 'Join voice'})}
                        </button>
                    </div>
                )}
            </div>
        </div>
    );
}
