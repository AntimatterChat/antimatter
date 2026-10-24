// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {am} from 'fusion/utils/class_names';

import {useCallActions} from './actions';
import type {VoiceParticipant} from './calls_api';

type MemberProps = {
    participant: VoiceParticipant;

    // Its place among the raised hands (0 when the hand isn't up).
    handOrder: number;
    onContextMenu: (e: React.MouseEvent<HTMLElement>) => void;
};

function AudienceMember({participant: p, handOrder, onContextMenu}: MemberProps) {
    const {formatMessage} = useIntl();
    const name = useDisplayName(useUser(p.userId));
    const label = p.isMe ? formatMessage({id: 'fusion.calls.you', defaultMessage: '{name} (you)'}, {name}) : name;
    return (
        <li
            className={am('aud-member')}
            title={label}
            data-talk={p.userId}
            onContextMenu={onContextMenu}
        >
            <Avatar
                userId={p.userId}
                size='lg'
            />
            {p.raisedHand && (
                <span
                    className={am('aud-hand')}
                    aria-label={formatMessage({id: 'fusion.calls.handRaised', defaultMessage: 'Hand raised'})}
                >
                    <Icon
                        name='hand'
                        size='xs'
                    />
                    {handOrder > 0 && <b>{handOrder}</b>}
                </span>
            )}
            <span className={am('aud-name')}>{label}</span>
        </li>
    );
}

type GridProps = {
    participants: VoiceParticipant[];
    hands: Record<string, number>;
    onContextMenu: (p: VoiceParticipant, e: React.MouseEvent<HTMLElement>) => void;
};

// AudienceGrid is a stage's audience: a compact grid of faces, with how many they are.
export function AudienceGrid({participants, hands, onContextMenu}: GridProps) {
    const {formatMessage} = useIntl();
    const heading = formatMessage({id: 'fusion.calls.stage.audience', defaultMessage: 'Audience'});
    return (
        <section
            className={am('stage-audience')}
            aria-label={heading}
        >
            <h3>
                {heading}
                <span className={am('count')}>{participants.length}</span>
            </h3>
            {participants.length > 0 && (
                <ul>
                    {participants.map((p) => (
                        <AudienceMember
                            key={p.sessionId}
                            participant={p}
                            handOrder={hands[p.sessionId] || 0}
                            onContextMenu={(e) => onContextMenu(p, e)}
                        />
                    ))}
                </ul>
            )}
        </section>
    );
}

function RaisedHand({participant: p, order, channelId}: {participant: VoiceParticipant; order: number; channelId: string}) {
    const {formatMessage} = useIntl();
    const actions = useCallActions();
    const name = useDisplayName(useUser(p.userId));
    return (
        <li className={am('hand-row')}>
            <b className={am('hand-order')}>{order}</b>
            <Avatar
                userId={p.userId}
                size='sm'
            />
            <span className={am('grow')}>{name}</span>
            <button
                type='button'
                className={am('btn', 'primary')}
                onClick={() => actions.inviteToSpeak(p, channelId)}
            >
                {formatMessage({id: 'fusion.calls.stage.invite', defaultMessage: 'Invite to speak'})}
            </button>
        </li>
    );
}

type HandsProps = {
    channelId: string;

    // The audience, with the raised hands' order.
    participants: VoiceParticipant[];
    hands: Record<string, number>;
};

// RaisedHands lists, for a stage's moderators, the audience's raised hands (their requests to speak), the first raised
// first, each with Invite to speak. It shows only while there are some.
export function RaisedHands({channelId, participants, hands}: HandsProps) {
    const {formatMessage} = useIntl();
    const raised = participants.filter((p) => hands[p.sessionId] > 0).sort((a, b) => hands[a.sessionId] - hands[b.sessionId]);
    if (!raised.length) {
        return null;
    }
    const heading = formatMessage({id: 'fusion.calls.stage.raisedHands', defaultMessage: 'Raised hands'});
    return (
        <section
            className={am('stage-hands')}
            aria-label={heading}
        >
            <h3>
                {heading}
                <span className={am('count')}>{raised.length}</span>
            </h3>
            <ol>
                {raised.map((p) => (
                    <RaisedHand
                        key={p.sessionId}
                        participant={p}
                        order={hands[p.sessionId]}
                        channelId={channelId}
                    />
                ))}
            </ol>
        </section>
    );
}
