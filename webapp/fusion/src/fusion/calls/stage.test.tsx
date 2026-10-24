// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {fireEvent, renderWithContext, screen, userEvent, within} from 'tests/react_testing_utils';

import type {VoiceParticipant} from './calls_api';
import Stage from './stage';

let mockParticipants: VoiceParticipant[] = [];
let mockStage: {speakers: string[]} | null = null;
let mockModerate = false;
jest.mock('./hooks', () => ({
    useParticipants: () => mockParticipants,
    useLocalCall: () => null,
    useCallMedia: () => ({localVideo: null, remoteVideos: {}, screen: null}),
    useIsVoiceChannel: () => true,
    useStage: () => mockStage,
    useCanModerateCall: () => mockModerate,
}));

const mockActions = {inviteToSpeak: jest.fn()};
jest.mock('./actions', () => ({useCallActions: () => mockActions}));

const mockOpenMenu = jest.fn();
jest.mock('./participant_menu', () => ({useParticipantMenu: () => ({open: mockOpenMenu, element: null})}));
jest.mock('./recording', () => ({RecordingBadge: () => null}));
jest.mock('./transcription', () => ({TranscriptionOverlay: () => null}));
jest.mock('./extras', () => ({...jest.requireActual('./extras'), useRecentReactions: () => []}));
jest.mock('mattermost-redux/actions/users', () => ({getMissingProfilesByIds: () => ({type: 'MOCK_GET_PROFILES'})}));

const state = {
    entities: {
        users: {
            profiles: {
                mod: {id: 'mod', username: 'maya'},
                guest: {id: 'guest', username: 'ada'},
                ana: {id: 'ana', username: 'ana'},
                ben: {id: 'ben', username: 'ben'},
                cy: {id: 'cy', username: 'cy'},
            },
        },
    },
} as never;

const participant = (userId: string, over: Partial<VoiceParticipant> = {}): VoiceParticipant => ({
    sessionId: 's-' + userId,
    userId,
    muted: true,
    deafened: false,
    video: false,
    screenSharing: false,
    speaking: false,
    raisedHand: false,
    isHost: false,
    isMe: false,
    modMuted: false,
    modDeafened: false,
    audience: false,
    ...over,
});

// The stage of a voice channel with the stage preset (design CHANNEL_TYPES_DESIGN.md §4.4): its speakers' tiles,
// large; its audience as a compact grid of faces, with their count; and for moderators, the raised hands first, with
// Invite to speak.
describe('fusion/calls/Stage in a stage', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        mockStage = {speakers: ['mod', 'guest']};
        mockModerate = false;
        mockParticipants = [
            participant('mod', {isHost: true, muted: false}),
            participant('ana', {audience: true, raisedHand: true, raisedHandAt: 20} as Partial<VoiceParticipant>),
            participant('guest'),
            participant('ben', {audience: true}),
            participant('cy', {audience: true, raisedHand: true, raisedHandAt: 10} as Partial<VoiceParticipant>),
        ];
    });

    const tilesOf = (container: HTMLElement) => Array.from(container.querySelectorAll('.am-tile')).map((t) => t.getAttribute('data-talk'));

    test('the speakers have the tiles, the audience a grid of faces with their count', () => {
        const {container} = renderWithContext(<Stage channelId='stage'/>, state);

        expect(container.querySelector('.am-stage')).toHaveClass('am-stage-room');
        expect(tilesOf(container)).toEqual(['mod', 'guest']);

        const audience = screen.getByRole('region', {name: 'Audience'});
        expect(within(audience).getByText('3')).toBeInTheDocument();
        expect(within(audience).getAllByRole('listitem').map((li) => li.getAttribute('data-talk'))).toEqual(['ana', 'ben', 'cy']);

        // Their raised hands show, in order.
        expect(within(audience).getAllByLabelText('Hand raised')).toHaveLength(2);
    });

    test('a member of the audience has the participant menu', () => {
        renderWithContext(<Stage channelId='stage'/>, state);

        const audience = screen.getByRole('region', {name: 'Audience'});
        fireEvent.contextMenu(within(audience).getAllByRole('listitem')[1]);
        expect(mockOpenMenu).toHaveBeenCalledWith(expect.objectContaining({userId: 'ben'}), expect.anything());
    });

    test('moderators see the raised hands first, the first raised first, and invite them to speak', async () => {
        mockModerate = true;
        renderWithContext(<Stage channelId='stage'/>, state);

        const hands = screen.getByRole('region', {name: 'Raised hands'});
        const rows = within(hands).getAllByRole('listitem');
        expect(rows.map((r) => r.textContent)).toEqual([expect.stringContaining('cy'), expect.stringContaining('ana')]);

        // Before the audience.
        const audience = screen.getByRole('region', {name: 'Audience'});
        expect(hands.compareDocumentPosition(audience) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

        await userEvent.click(within(rows[0]).getByRole('button', {name: 'Invite to speak'}));
        expect(mockActions.inviteToSpeak).toHaveBeenCalledWith(expect.objectContaining({userId: 'cy'}), 'stage');
    });

    test('the raised hands are for moderators only', () => {
        renderWithContext(<Stage channelId='stage'/>, state);

        expect(screen.queryByRole('region', {name: 'Raised hands'})).not.toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Invite to speak'})).not.toBeInTheDocument();
    });

    test('without raised hands, moderators have no list either', () => {
        mockModerate = true;
        mockParticipants = mockParticipants.map((p) => ({...p, raisedHand: false}));
        renderWithContext(<Stage channelId='stage'/>, state);

        expect(screen.queryByRole('region', {name: 'Raised hands'})).not.toBeInTheDocument();
    });

    test('with nobody speaking, the stage says so and the audience is still shown', () => {
        mockParticipants = mockParticipants.filter((p) => p.audience);
        const {container} = renderWithContext(<Stage channelId='stage'/>, state);

        expect(tilesOf(container)).toEqual([]);
        expect(screen.getByText('Nobody is speaking yet. Moderators speak here, and invite others to.')).toBeInTheDocument();
        expect(screen.getByRole('region', {name: 'Audience'})).toBeInTheDocument();
    });

    test('a speaker sharing their screen takes the stage, the audience stays under it', () => {
        mockParticipants = mockParticipants.map((p) => (p.userId === 'guest' ? {...p, screenSharing: true} : p));
        const {container} = renderWithContext(<Stage channelId='stage'/>, state);

        expect(container.querySelector('.am-tile.am-screen')).toBeInTheDocument();
        expect(tilesOf(container).filter(Boolean)).toEqual(['mod', 'guest']);
        expect(screen.getByRole('region', {name: 'Audience'})).toBeInTheDocument();
    });
});

describe('fusion/calls/Stage in a talk call', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        mockModerate = true;
        mockParticipants = [participant('mod'), participant('ana', {raisedHand: true})];
    });

    // A Calls version without stages, or a talk channel: getStage gives nothing, and everyone has a tile.
    test('everyone has a tile, with no audience or raised hands list', () => {
        mockStage = null;
        const {container} = renderWithContext(<Stage channelId='talk'/>, state);

        expect(container.querySelector('.am-stage')).not.toHaveClass('am-stage-room');
        expect(Array.from(container.querySelectorAll('.am-tile')).map((t) => t.getAttribute('data-talk'))).toEqual(['mod', 'ana']);
        expect(screen.queryByRole('region', {name: 'Audience'})).not.toBeInTheDocument();
        expect(screen.queryByRole('region', {name: 'Raised hands'})).not.toBeInTheDocument();
    });
});
