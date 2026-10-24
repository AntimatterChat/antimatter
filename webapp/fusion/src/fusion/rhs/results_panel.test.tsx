// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {RHSStates} from 'utils/constants';

import ResultsPanel from './results_panel';

// Antimatter: the Files tab (and its results) only show when ServiceSettings.EnableFileSearch (and
// FileSettings.EnableFileAttachments) are on, as the classic search results already hide.
describe('fusion/rhs/ResultsPanel: the Files tab', () => {
    const renderPanel = (config: Record<string, string>) => renderWithContext(
        <ResultsPanel rhsState={RHSStates.SEARCH}/>,
        {
            views: {
                rhs: {
                    rhsState: RHSStates.SEARCH,
                    searchType: 'messages',
                },
            },
            entities: {
                general: {config},
            },
        } as never,
    );

    test('shows the Files tab when file search is enabled', () => {
        renderPanel({EnableFileAttachments: 'true', EnableFileSearch: 'true'});
        expect(screen.getByText('Files')).toBeInTheDocument();
    });

    test('hides the Files tab when file search is disabled', () => {
        renderPanel({EnableFileAttachments: 'true', EnableFileSearch: 'false'});
        expect(screen.queryByText('Files')).not.toBeInTheDocument();
    });

    test('hides the Files tab when file attachments are disabled', () => {
        renderPanel({EnableFileAttachments: 'false', EnableFileSearch: 'true'});
        expect(screen.queryByText('Files')).not.toBeInTheDocument();
    });
});

// Antimatter forums (design §4.4): a hit in a forum shows its topic's title above it.
describe('fusion/rhs/ResultsPanel: forum hits', () => {
    const channel = {id: 'c1', team_id: 'team', type: 'O', name: 'beam-questions', display_name: 'Beam questions', delete_at: 0};
    const root = {id: 'r0', channel_id: 'c1', root_id: '', user_id: 'ada', message: 'It won\'t power on at all.', create_at: 0, props: {am_thread_title: 'How do I reset the beam?'}, metadata: {}};
    const reply = {id: 'r1', channel_id: 'c1', root_id: 'r0', user_id: 'maya', message: 'Hold the reset switch for 5 seconds.', create_at: 100, props: {}, metadata: {}};

    const renderWithHits = (postIds: string[]) => renderWithContext(
        <ResultsPanel rhsState={RHSStates.SEARCH}/>,
        {
            views: {
                rhs: {
                    rhsState: RHSStates.SEARCH,
                    searchType: 'messages',
                },
            },
            entities: {
                general: {config: {}},
                users: {currentUserId: 'me', profiles: {ada: {id: 'ada', username: 'ada'}, maya: {id: 'maya', username: 'maya'}}},
                teams: {currentTeamId: 'team', teams: {team: {id: 'team', name: 'lumen-labs', display_name: 'Lumen Labs', delete_at: 0}}},
                channels: {
                    channels: {c1: channel},
                    channelTypes: {c1: {type: 'forum', mode: 'qa', root_posts: 'members', view: 'topics', preset: ''}},
                },
                posts: {posts: {r0: root, r1: reply}},
                search: {results: postIds},
                preferences: {myPreferences: {}},
            },
        } as never,
    );

    test('a hit on the topic itself shows its own title', () => {
        renderWithHits(['r0']);
        expect(screen.getByText('How do I reset the beam?', {selector: 'div'})).toBeInTheDocument();
    });

    test('a hit on a reply shows the topic\'s title (its root\'s)', () => {
        renderWithHits(['r1']);
        expect(screen.getByText('How do I reset the beam?', {selector: 'div'})).toBeInTheDocument();
        expect(screen.getByText('Hold the reset switch for 5 seconds.')).toBeInTheDocument();
    });
});
