// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {ChannelKindMode} from '@mattermost/types/channels';

import {Client4} from 'mattermost-redux/client';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import TopicsView from './topics_view';

// Overriding a prototype method (Avatar's getUsersRoute among them) by spreading Client4 would drop it: mutate
// the real singleton's own getForumTopics instead, keeping the rest working.
jest.mock('mattermost-redux/client', () => {
    const actual = jest.requireActual('mattermost-redux/client');
    actual.Client4.getForumTopics = jest.fn();
    return actual;
});
jest.mock('mattermost-redux/actions/preferences', () => ({savePreferences: jest.fn(() => ({type: 'MOCK_SAVE'}))}));

let newTopicFormProps: Record<string, unknown> | undefined;
jest.mock('./new_topic_form', () => ({
    __esModule: true,
    default: (props: Record<string, unknown>) => {
        newTopicFormProps = props;
        return <div>{'New topic form'}</div>;
    },
}));
jest.mock('./imageboard_view', () => ({__esModule: true, default: () => <div>{'Image board view'}</div>}));

describe('fusion/forum/TopicsView', () => {
    const channel = TestHelper.getChannelMock({id: 'c1', team_id: 'team', purpose: 'Keep it on topic.'});
    const asker = TestHelper.getUserMock({id: 'ada', username: 'ada'});
    const topic = (override: Record<string, unknown> = {}) => ({
        post: TestHelper.getPostMock({id: 'p1', user_id: 'ada', message: 'How do I reset the beam?', create_at: 1000}),
        reply_count: 3,
        last_reply_at: 2000,
        participant_ids: ['ada', 'maya'],
        votes: 2,
        on_top: false,
        ...override,
    });

    const baseState = (mode: ChannelKindMode, extra: Record<string, unknown> = {}) => ({
        entities: {
            users: {currentUserId: 'me', profiles: {ada: asker}},
            teams: {currentTeamId: 'team', teams: {team: TestHelper.getTeamMock({id: 'team', name: 'lumen-labs'})}},
            channels: {
                currentChannelId: 'c1',
                channels: {c1: channel},
                myMembers: {c1: TestHelper.getChannelMembershipMock({channel_id: 'c1', user_id: 'me', last_viewed_at: 1500}, {})},
                channelTypes: {c1: {type: 'forum', mode, root_posts: 'members', view: 'topics', preset: ''}},
            },
            threads: {threads: {}},
            preferences: {myPreferences: {}},
        },
        ...extra,
    } as never);

    // A state with the row list's own channelTypes.view default and/or the person's own fusion_forum_view
    // preference (view_preference.ts, already unit-tested there): only what the view switch needs to pick between.
    const stateWithView = (channelView: 'topics' | 'imageboard', ownPreference?: 'topics' | 'imageboard') => ({
        entities: {
            users: {currentUserId: 'me', profiles: {ada: asker}},
            teams: {currentTeamId: 'team', teams: {team: TestHelper.getTeamMock({id: 'team', name: 'lumen-labs'})}},
            channels: {
                currentChannelId: 'c1',
                channels: {c1: channel},
                myMembers: {c1: TestHelper.getChannelMembershipMock({channel_id: 'c1', user_id: 'me', last_viewed_at: 1500}, {})},
                channelTypes: {c1: {type: 'forum', mode: 'discussion', root_posts: 'members', view: channelView, preset: ''}},
            },
            threads: {threads: {}},
            preferences: {myPreferences: ownPreference ? {'fusion_forum_view--c1': {category: 'fusion_forum_view', name: 'c1', value: ownPreference}} : {}},
        },
    } as never);

    beforeEach(() => {
        newTopicFormProps = undefined;
        jest.mocked(Client4.getForumTopics).mockReset();
        jest.mocked(Client4.getForumTopics).mockResolvedValue({topics: [], next_cursor: ''});
    });

    test('discussion mode: a row with its title, first line, author, replies and activity', async () => {
        jest.mocked(Client4.getForumTopics).mockResolvedValue({topics: [topic()], next_cursor: ''});
        renderWithContext(<TopicsView channel={channel}/>, baseState('discussion'));

        expect(await screen.findByRole('button', {name: 'How do I reset the beam?'})).toBeInTheDocument();
        expect(screen.getByText('3 replies')).toBeInTheDocument();
        expect(screen.queryByText('Unanswered')).not.toBeInTheDocument();
    });

    test('Q&A mode: the answered mark and votes show, the Unanswered filter and sort are offered', async () => {
        jest.mocked(Client4.getForumTopics).mockResolvedValue({topics: [topic({accepted_answer_id: 'r1'})], next_cursor: ''});
        renderWithContext(<TopicsView channel={channel}/>, baseState('qa'));

        expect(await screen.findByText('Answered')).toBeInTheDocument();
        expect(screen.getByRole('radio', {name: 'Unanswered'})).toBeInTheDocument();
        expect(screen.getByRole('option', {name: 'Unanswered first'})).toBeInTheDocument();
    });

    test('Ideas mode: the status chip shows, and a filter chip per status is offered', async () => {
        jest.mocked(Client4.getForumTopics).mockResolvedValue({topics: [topic({status: 'planned'})], next_cursor: ''});
        renderWithContext(<TopicsView channel={channel}/>, baseState('ideas'));

        expect(await screen.findByText('Planned')).toBeInTheDocument();
        expect(screen.getByRole('radio', {name: 'Done'})).toBeInTheDocument();
    });

    test('changing the sort refetches with it', async () => {
        renderWithContext(<TopicsView channel={channel}/>, baseState('discussion'));
        await waitFor(() => expect(Client4.getForumTopics).toHaveBeenCalledWith('c1', {sort: 'activity'}));

        await userEvent.selectOptions(screen.getByLabelText('Sort'), 'Newest');

        await waitFor(() => expect(Client4.getForumTopics).toHaveBeenLastCalledWith('c1', {sort: 'newest'}));
    });

    test('clicking the Mine filter refetches with mine:true', async () => {
        renderWithContext(<TopicsView channel={channel}/>, baseState('discussion'));
        await waitFor(() => expect(Client4.getForumTopics).toHaveBeenCalled());

        await userEvent.click(screen.getByRole('radio', {name: 'Mine'}));

        await waitFor(() => expect(Client4.getForumTopics).toHaveBeenLastCalledWith('c1', {sort: 'activity', mine: true}));
    });

    test('typing in the search box refetches with q', async () => {
        renderWithContext(<TopicsView channel={channel}/>, baseState('discussion'));
        await waitFor(() => expect(Client4.getForumTopics).toHaveBeenCalled());

        await userEvent.type(screen.getByRole('searchbox', {name: 'Search topics…'}), 'beam');

        await waitFor(() => expect(Client4.getForumTopics).toHaveBeenLastCalledWith('c1', {sort: 'activity', q: 'beam'}));
    });

    test('empty state: no topics, no search', async () => {
        renderWithContext(<TopicsView channel={channel}/>, baseState('discussion'));

        expect(await screen.findByText('No topics yet.')).toBeInTheDocument();
    });

    test('empty state: no results for a search term', async () => {
        renderWithContext(<TopicsView channel={channel}/>, baseState('discussion'));
        await userEvent.type(screen.getByRole('searchbox', {name: 'Search topics…'}), 'nothing');

        expect(await screen.findByText('No topics found for "nothing".')).toBeInTheDocument();
    });

    test('a topic newer than the channel\'s last view is marked unread', async () => {
        jest.mocked(Client4.getForumTopics).mockResolvedValue({topics: [topic({post: TestHelper.getPostMock({id: 'p2', user_id: 'ada', message: 'New one', create_at: 9000})})], next_cursor: ''});
        const {container} = renderWithContext(<TopicsView channel={channel}/>, baseState('discussion'));

        await screen.findByRole('button', {name: 'New one'});
        expect(container.querySelector('.am-topic-row.am-unread')).toBeInTheDocument();
    });

    test('a topic older than the channel\'s last view isn\'t marked unread', async () => {
        jest.mocked(Client4.getForumTopics).mockResolvedValue({topics: [topic()], next_cursor: ''});
        const {container} = renderWithContext(<TopicsView channel={channel}/>, baseState('discussion'));

        await screen.findByRole('button', {name: 'How do I reset the beam?'});
        expect(container.querySelector('.am-topic-row.am-unread')).not.toBeInTheDocument();
    });

    test('the New topic button opens the form, labelled by mode', async () => {
        renderWithContext(<TopicsView channel={channel}/>, baseState('qa'));

        await userEvent.click(await screen.findByRole('button', {name: 'Ask a question'}));

        expect(screen.getByText('New topic form')).toBeInTheDocument();
    });

    test('the view switch saves the person\'s own preference', async () => {
        const {savePreferences} = jest.requireMock('mattermost-redux/actions/preferences');
        renderWithContext(<TopicsView channel={channel}/>, baseState('discussion'));

        await userEvent.click(screen.getByRole('radio', {name: 'Image board'}));

        expect(savePreferences).toHaveBeenCalledWith('me', [{user_id: 'me', category: 'fusion_forum_view', name: 'c1', value: 'imageboard'}]);
    });

    test('shows the channel\'s purpose as guidelines', async () => {
        renderWithContext(<TopicsView channel={channel}/>, baseState('discussion'));

        expect(await screen.findByText('Guidelines: Keep it on topic.')).toBeInTheDocument();
    });

    test('renders the image board when the channel\'s own default picks it', async () => {
        renderWithContext(<TopicsView channel={channel}/>, stateWithView('imageboard'));

        expect(await screen.findByText('Image board view')).toBeInTheDocument();
        expect(screen.queryByRole('searchbox')).not.toBeInTheDocument();
    });

    test('renders the row list when the channel\'s own default picks it', async () => {
        renderWithContext(<TopicsView channel={channel}/>, stateWithView('topics'));

        expect(await screen.findByText('No topics yet.')).toBeInTheDocument();
        expect(screen.queryByText('Image board view')).not.toBeInTheDocument();
    });

    test('the person\'s own preference overrides the channel\'s default', async () => {
        renderWithContext(<TopicsView channel={channel}/>, stateWithView('topics', 'imageboard'));

        expect(await screen.findByText('Image board view')).toBeInTheDocument();
    });

    test('the image board view leads the New topic form with the attachment area', async () => {
        renderWithContext(<TopicsView channel={channel}/>, stateWithView('imageboard'));

        await userEvent.click(await screen.findByRole('button', {name: 'New topic'}));

        expect(screen.getByText('New topic form')).toBeInTheDocument();
        expect(newTopicFormProps?.leadWithAttachment).toBe(true);
    });

    test('the topics view doesn\'t lead the New topic form with the attachment area', async () => {
        renderWithContext(<TopicsView channel={channel}/>, stateWithView('topics'));

        await userEvent.click(await screen.findByRole('button', {name: 'New topic'}));

        expect(newTopicFormProps?.leadWithAttachment).toBe(false);
    });
});
