// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useSelector, useStore} from 'react-redux';

import {getChannel} from 'mattermost-redux/selectors/entities/channels';
import {getTeam} from 'mattermost-redux/selectors/entities/teams';

import Icon from 'fusion/components/icon';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {useLayout} from 'fusion/shell/layout_context';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {goToChannel, useCallActions, useChannelLabel} from './actions';
import CallVideo from './call_video';
import type {VoiceParticipant} from './calls_api';
import {useToggleLabels} from './controls';
import {useCallMedia, useMyCall, useParticipants} from './hooks';
import {FakeScreen} from './stage';
import {setCallWindowOpen} from './ui_state';

// Where the call is: its team, or "Direct message".
function useWhere(channelId: string): string {
    const {formatMessage} = useIntl();
    const team = useSelector((state: GlobalState) => {
        const channel = getChannel(state, channelId);
        if (!channel || channel.type === 'D' || channel.type === 'G') {
            return null;
        }
        return getTeam(state, channel.team_id)?.display_name || '';
    });
    return team ?? formatMessage({id: 'fusion.calls.directMessage', defaultMessage: 'Direct message'});
}

function MiniScreen({sharer, stream}: {sharer: VoiceParticipant; stream: MediaStream | null}) {
    const {formatMessage} = useIntl();
    const user = useUser(sharer.userId);
    const name = useDisplayName(user);
    const first = user?.first_name || name;
    return (
        <button
            className={am('mini-screen')}
            aria-label={formatMessage({id: 'fusion.calls.miniScreenLabel', defaultMessage: '{name} is sharing their screen — open the call in a new window'}, {name})}
            onClick={() => setCallWindowOpen(true)}
        >
            {stream ? (
                <CallVideo
                    stream={stream}
                    fit='contain'
                />
            ) : (
                <FakeScreen
                    lines={7}
                    tree={3}
                />
            )}
            <span className={am('who')}>
                {sharer.isMe ? formatMessage({id: 'fusion.calls.youAreSharing', defaultMessage: 'You are sharing'}) : formatMessage({id: 'fusion.calls.isSharing', defaultMessage: '{name} is sharing'}, {name: first})}
            </span>
            <span className={am('pop-ic')}><Icon name='popout'/></span>
        </button>
    );
}

// VoiceConnectionPanel is the mockup's voice connection panel at the bottom of the sidebar, while you are in a call
// or a voice channel: where you are, the shared screen, and your mic, deafen, camera and screen toggles.
export default function VoiceConnectionPanel() {
    const {formatMessage} = useIntl();
    const store = useStore<GlobalState>();
    const layout = useLayout();
    const actions = useCallActions();
    const my = useMyCall();
    const channelId = my?.call.channelId || '';
    const participants = useParticipants(channelId);
    const media = useCallMedia(channelId);
    const label = useChannelLabel(channelId);
    const where = useWhere(channelId);
    const labels = useToggleLabels(my);

    if (!my) {
        return null;
    }

    const sharer = participants.find((p) => p.screenSharing);
    let status;
    if (my.call.state === 'connecting') {
        status = formatMessage({id: 'fusion.calls.connecting', defaultMessage: 'Connecting…'});
    } else if (my.voice) {
        status = formatMessage({id: 'fusion.calls.voiceConnected', defaultMessage: 'Voice connected'});
    } else {
        status = formatMessage({id: 'fusion.calls.callConnected', defaultMessage: 'Call connected'});
    }

    return (
        <div
            className={am('voice-conn')}
            aria-label={my.voice ? formatMessage({id: 'fusion.calls.voiceConnection', defaultMessage: 'Voice connection'}) : formatMessage({id: 'fusion.calls.callConnection', defaultMessage: 'Call connection'})}
        >
            {sharer && (
                <MiniScreen
                    sharer={sharer}
                    stream={media.screen}
                />
            )}
            <div className={am('top')}>
                <div className={am('state')}>
                    <b>
                        <Icon
                            name={my.voice ? 'speaker' : 'phone'}
                            size='xs'
                        />
                        {status}
                    </b>
                    <button
                        onClick={() => {
                            layout.setNavOpen(false);
                            goToChannel(store.getState(), channelId);
                        }}
                    >
                        {`${label} · ${where}`}
                    </button>
                </div>
                <button
                    className={am('icon-btn')}
                    title={labels.popout}
                    aria-label={labels.popoutLabel}
                    onClick={() => setCallWindowOpen(true)}
                >
                    <Icon
                        name='popout'
                        size='sm'
                    />
                </button>
                <button
                    className={am('icon-btn')}
                    title={labels.disconnect}
                    aria-label={labels.disconnect}
                    style={{color: 'var(--am-danger)'}}
                    onClick={actions.leaveCall}
                >
                    <Icon name='hangup'/>
                </button>
            </div>
            {my.call.mediaInThisWindow && (
                <div
                    className={am('row')}
                    style={my.voice ? undefined : {gridTemplateColumns: 'repeat(3, 1fr)'}}
                >
                    <button
                        className={am({off: my.muted})}
                        aria-pressed={my.muted}
                        title={labels.mute}
                        aria-label={labels.mute}
                        onClick={() => actions.toggleMute(my)}
                    >
                        <Icon
                            name={my.muted ? 'mic-off' : 'mic'}
                            size='sm'
                        />
                    </button>
                    {my.voice && (
                        <button
                            className={am({off: my.deafened})}
                            aria-pressed={my.deafened}
                            title={labels.deafen}
                            aria-label={labels.deafen}
                            onClick={() => actions.toggleDeafen(my)}
                        >
                            <Icon
                                name={my.deafened ? 'headphones-off' : 'headphones'}
                                size='sm'
                            />
                        </button>
                    )}
                    <button
                        className={am({on: my.video})}
                        aria-pressed={my.video}
                        title={labels.video}
                        aria-label={labels.video}
                        disabled={!my.videoAllowed && !my.video}
                        onClick={() => actions.toggleVideo(my)}
                    >
                        <Icon
                            name='video'
                            size='sm'
                        />
                    </button>
                    <button
                        className={am({on: my.screen})}
                        aria-pressed={my.screen}
                        title={labels.screen}
                        aria-label={labels.screen}
                        disabled={!my.screen && (!my.screenAllowed || my.otherSharing)}
                        onClick={() => actions.toggleScreen(my)}
                    >
                        <Icon
                            name='screen'
                            size='sm'
                        />
                    </button>
                </div>
            )}
        </div>
    );
}
