// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useRef} from 'react';
import {useIntl} from 'react-intl';

import Avatar, {hueFor} from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {showToast} from 'fusion/components/toast';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {am} from 'fusion/utils/class_names';

import CallVideo from './call_video';
import {isSpeaking} from './calls_api';
import type {VoiceParticipant} from './calls_api';
import {useCallMedia, useIsVoiceChannel, useLocalCall, useParticipants} from './hooks';
import {useParticipantMenu} from './participant_menu';

// FakeScreen is the mockup's drawing of a shared screen, shown until (or where) the real one can't be.
export function FakeScreen({lines = 12, tree = 5}: {lines?: number; tree?: number}) {
    return (
        <div
            className={am('fake-screen')}
            aria-hidden='true'
        >
            <div className={am('tree')}>{Array.from({length: tree}, (_, i) => <i key={i}/>)}</div>
            <div className={am('code')}>{Array.from({length: lines}, (_, i) => <i key={i}/>)}</div>
        </div>
    );
}

type TileProps = {
    participant: VoiceParticipant;
    stream: MediaStream | null;

    // The camera is shown when the call's media runs in this window.
    media: boolean;
    onContextMenu: (e: React.MouseEvent<HTMLElement>) => void;
};

function Tile({participant: p, stream, media, onContextMenu}: TileProps) {
    const {formatMessage} = useIntl();
    const name = useDisplayName(useUser(p.userId));
    const cam = p.video && media;

    let audio = null;
    if (p.deafened) {
        audio = <Icon name='headphones-off'/>;
    } else if (p.muted) {
        audio = <Icon name='mic-off'/>;
    }

    return (
        <div
            className={am('tile', {cam, speaking: isSpeaking(p)})}
            data-talk={p.userId}
            style={{'--am-h': hueFor(p.userId)} as React.CSSProperties}
            onContextMenu={onContextMenu}
        >
            {cam ? (
                <CallVideo
                    stream={stream}
                    mirror={p.isMe}
                />
            ) : (
                <Avatar
                    userId={p.userId}
                    size='xl'
                />
            )}
            <span className={am('label')}>
                {p.raisedHand && (
                    <span
                        className={am('hand')}
                        title={formatMessage({id: 'fusion.calls.handRaised', defaultMessage: 'Hand raised'})}
                    >
                        <Icon name='hand'/>
                    </span>
                )}
                {audio}
                {p.isMe ? formatMessage({id: 'fusion.calls.you', defaultMessage: '{name} (you)'}, {name}) : name}
            </span>
        </div>
    );
}

type ScreenProps = {
    sharer: VoiceParticipant;
    stream: MediaStream | null;
    fullscreen: boolean;
};

function ScreenTile({sharer, stream, fullscreen}: ScreenProps) {
    const {formatMessage} = useIntl();
    const name = useDisplayName(useUser(sharer.userId));
    const ref = useRef<HTMLDivElement>(null);

    const goFullscreen = () => {
        const tile = ref.current;
        const unavailable = () => showToast(formatMessage({id: 'fusion.calls.noFullscreen', defaultMessage: 'Full screen is not available here'}));
        if (!tile?.requestFullscreen) {
            unavailable();
            return;
        }
        tile.requestFullscreen().catch(unavailable);
    };

    return (
        <div
            ref={ref}
            className={am('tile', 'screen')}
        >
            {stream ? (
                <CallVideo
                    stream={stream}
                    fit='contain'
                />
            ) : <FakeScreen/>}
            <span className={am('label')}>
                <Icon name='screen'/>
                {formatMessage({id: 'fusion.calls.screenOf', defaultMessage: '{name}\'s screen'}, {name})}
            </span>
            {fullscreen && (
                <button
                    className={am('icon-btn')}
                    style={{position: 'absolute', top: 8, right: 8, background: 'rgba(0,0,0,.5)', color: '#fff'}}
                    aria-label={formatMessage({id: 'fusion.calls.fullscreen', defaultMessage: 'Full screen'})}
                    onClick={goFullscreen}
                >
                    <Icon
                        name='fullscreen'
                        size='sm'
                    />
                </button>
            )}
        </div>
    );
}

type Props = {
    channelId: string;

    // The voice channel view offers full screen on the shared screen; the call window doesn't.
    fullscreen?: boolean;
};

// Stage is the mockup's stage of a call: a grid of tiles that always fits its space, or the shared screen with the
// tiles in a strip underneath.
export default function Stage({channelId, fullscreen = false}: Props) {
    const {formatMessage} = useIntl();
    const participants = useParticipants(channelId);
    const local = useLocalCall();
    const media = useCallMedia(channelId);
    const menu = useParticipantMenu(channelId);
    const voice = useIsVoiceChannel(channelId);
    const withMedia = local?.channelId === channelId && local.mediaInThisWindow;

    if (!participants.length) {
        return (
            <div className={am('stage')}>
                <div className={am('empty')}>
                    {voice ? formatMessage({id: 'fusion.calls.emptyRoom', defaultMessage: 'This room is always open. Join and others will see you in the sidebar.'}) : formatMessage({id: 'fusion.calls.connecting', defaultMessage: 'Connecting…'})}
                </div>
            </div>
        );
    }

    const tiles = participants.map((p) => (
        <Tile
            key={p.sessionId}
            participant={p}
            stream={p.isMe ? media.localVideo : media.remoteVideos[p.sessionId] || null}
            media={withMedia}
            onContextMenu={(e) => menu.open(p, e)}
        />
    ));

    // Calls has one screen share per call.
    const sharer = participants.find((p) => p.screenSharing);
    if (sharer) {
        return (
            <div className={am('stage')}>
                <ScreenTile
                    sharer={sharer}
                    stream={withMedia ? media.screen : null}
                    fullscreen={fullscreen}
                />
                <div className={am('stage-strip')}>{tiles}</div>
                {menu.element}
            </div>
        );
    }

    const cols = Math.ceil(Math.sqrt(tiles.length));
    const rows = Math.ceil(tiles.length / cols);
    return (
        <div className={am('stage')}>
            <div
                className={am('stage-grid')}
                style={{'--am-cols': cols, '--am-rows': rows} as React.CSSProperties}
            >
                {tiles}
            </div>
            {menu.element}
        </div>
    );
}
