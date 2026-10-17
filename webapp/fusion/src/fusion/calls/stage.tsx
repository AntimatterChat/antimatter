// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';

import RenderEmoji from 'components/emoji/render_emoji';

import Avatar, {hueFor} from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {showToast} from 'fusion/components/toast';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {am} from 'fusion/utils/class_names';

import CallVideo from './call_video';
import {isSpeaking} from './calls_api';
import type {VoiceParticipant} from './calls_api';
import {raisedHandOrder, useRecentReactions} from './extras';
import {useCallMedia, useIsVoiceChannel, useLocalCall, useParticipants} from './hooks';
import {useParticipantMenu} from './participant_menu';
import {RecordingBadge} from './recording';
import {TranscriptionOverlay} from './transcription';

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

    // Its place among the raised hands (0 when the hand isn't up), and whether it's pinned.
    handOrder: number;
    pinned: boolean;
    onPin: () => void;
};

// pictureInPicture floats a tile's video in the system's picture-in-picture window.
function pictureInPicture(tile: HTMLElement | null, unavailable: () => void) {
    const video = tile?.querySelector('video');
    if (!video || !document.pictureInPictureEnabled) {
        unavailable();
        return;
    }
    video.requestPictureInPicture().catch(unavailable);
}

function Tile({participant: p, stream, media, onContextMenu, handOrder, pinned, onPin}: TileProps) {
    const {formatMessage} = useIntl();
    const name = useDisplayName(useUser(p.userId));
    const cam = p.video && media;
    const ref = useRef<HTMLDivElement>(null);

    // A moderator mute shows over deafen and mute: it's the one they can't undo.
    let audio = null;
    if (p.modMuted) {
        audio = (
            <span
                className={am('mod-muted')}
                title={formatMessage({id: 'fusion.calls.modMuted', defaultMessage: 'Muted by a moderator'})}
                aria-label={formatMessage({id: 'fusion.calls.modMuted', defaultMessage: 'Muted by a moderator'})}
            >
                <Icon name='mic-lock'/>
            </span>
        );
    } else if (p.deafened) {
        audio = <Icon name='headphones-off'/>;
    } else if (p.muted) {
        audio = <Icon name='mic-off'/>;
    }

    return (
        <div
            ref={ref}
            className={am('tile', {cam, speaking: isSpeaking(p), pinned})}
            data-talk={p.userId}
            onDoubleClick={onPin}
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
            {/* A raised hand: a badge in the tile's corner, with its place in the queue, apart from the name. */}
            {p.raisedHand && (
                <span
                    className={am('tile-hand')}
                    title={handOrder > 0 ? formatMessage({id: 'fusion.calls.handRaisedOrder', defaultMessage: 'Hand raised, {order, selectordinal, one {#st} two {#nd} few {#rd} other {#th}} in line'}, {order: handOrder}) : formatMessage({id: 'fusion.calls.handRaised', defaultMessage: 'Hand raised'})}
                >
                    <Icon
                        name='hand'
                        size='sm'
                    />
                    {handOrder > 0 && <b>{handOrder}</b>}
                </span>
            )}
            <span className={am('label')}>
                {audio}
                {p.isMe ? formatMessage({id: 'fusion.calls.you', defaultMessage: '{name} (you)'}, {name}) : name}
            </span>
            <span className={am('tile-tools')}>
                {cam && (
                    <button
                        type='button'
                        title={formatMessage({id: 'fusion.calls.pip', defaultMessage: 'Picture in picture'})}
                        aria-label={formatMessage({id: 'fusion.calls.pip', defaultMessage: 'Picture in picture'})}
                        onClick={() => pictureInPicture(ref.current, () => showToast(formatMessage({id: 'fusion.calls.noPip', defaultMessage: 'Picture in picture is not available here'})))}
                    >
                        <Icon
                            name='popout'
                            size='sm'
                        />
                    </button>
                )}
                <button
                    type='button'
                    className={am({on: pinned})}
                    title={pinned ? formatMessage({id: 'fusion.calls.unpin', defaultMessage: 'Unpin'}) : formatMessage({id: 'fusion.calls.pin', defaultMessage: 'Pin to the stage'})}
                    aria-label={pinned ? formatMessage({id: 'fusion.calls.unpin', defaultMessage: 'Unpin'}) : formatMessage({id: 'fusion.calls.pin', defaultMessage: 'Pin to the stage'})}
                    aria-pressed={pinned}
                    onClick={onPin}
                >
                    <Icon
                        name='pin'
                        size='sm'
                    />
                </button>
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
            {stream && (
                <button
                    className={am('icon-btn')}
                    style={{position: 'absolute', top: 8, right: fullscreen ? 50 : 8, background: 'rgba(0,0,0,.5)', color: '#fff'}}
                    title={formatMessage({id: 'fusion.calls.pip', defaultMessage: 'Picture in picture'})}
                    aria-label={formatMessage({id: 'fusion.calls.pip', defaultMessage: 'Picture in picture'})}
                    onClick={() => pictureInPicture(ref.current, () => showToast(formatMessage({id: 'fusion.calls.noPip', defaultMessage: 'Picture in picture is not available here'})))}
                >
                    <Icon
                        name='popout'
                        size='sm'
                    />
                </button>
            )}
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

// Reactions float the call's recent reactions over the stage, with who sent them.
function Reactions({channelId}: {channelId: string}) {
    const reactions = useRecentReactions(channelId);
    const [, setTick] = useState(0);

    // Re-read as reactions get old, to let them go.
    useEffect(() => {
        if (!reactions.length) {
            return undefined;
        }
        const timer = setInterval(() => setTick((t) => t + 1), 1000);
        return () => clearInterval(timer);
    }, [reactions.length]);
    if (!reactions.length) {
        return null;
    }
    return (
        <div
            className={am('reactions-float')}
            aria-live='polite'
        >
            {reactions.slice(-8).map((r) => (
                <ReactionBubble
                    key={`${r.sessionId}:${r.timestamp}`}
                    userId={r.userId}
                    emojiName={r.emojiName}
                />
            ))}
        </div>
    );
}

function ReactionBubble({userId, emojiName}: {userId: string; emojiName: string}) {
    const name = useDisplayName(useUser(userId));
    return (
        <span className={am('reaction-bubble')}>
            <RenderEmoji
                emojiName={emojiName}
                size={22}
            />
            <span>{name}</span>
        </span>
    );
}

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
    const [pinnedId, setPinnedId] = useState('');
    const hands = raisedHandOrder(participants);
    const reactions = (
        <>
            <Reactions channelId={channelId}/>
            <RecordingBadge channelId={channelId}/>
            <TranscriptionOverlay channelId={channelId}/>
        </>
    );

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
            handOrder={hands[p.sessionId] || 0}
            pinned={p.sessionId === pinnedId}
            onPin={() => setPinnedId(p.sessionId === pinnedId ? '' : p.sessionId)}
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
                {reactions}
                {menu.element}
            </div>
        );
    }

    // A pinned participant takes the stage, the others in the strip underneath.
    const pinned = participants.findIndex((p) => p.sessionId === pinnedId);
    if (pinned >= 0 && participants.length > 1) {
        return (
            <div className={am('stage')}>
                <div className={am('stage-focus')}>{tiles[pinned]}</div>
                <div className={am('stage-strip')}>{tiles.filter((_, i) => i !== pinned)}</div>
                {reactions}
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
            {reactions}
            {menu.element}
        </div>
    );
}
