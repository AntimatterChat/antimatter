// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {createMemoryHistory} from 'history';
import React from 'react';
import {Route} from 'react-router-dom';

import {renderWithContext, waitFor} from 'tests/react_testing_utils';

import AppLinkRedirect, {APP_LINK_PATHS} from './app_link_redirect';

// Antimatter: Playbooks left its full page for the side panel; its old links open in a community, where the app
// opens the panel on the page they point to.
describe('components/root/AppLinkRedirect', () => {
    const team = (id: string, name: string, displayName: string) => ({id, name, display_name: displayName, delete_at: 0});
    const state = {
        entities: {
            teams: {
                teams: {
                    t1: team('t1', 'zeta', 'Zeta'),
                    t2: team('t2', 'alpha', 'Alpha'),
                },
                myMembers: {
                    t1: {team_id: 't1', user_id: 'u1'},
                    t2: {team_id: 't2', user_id: 'u1'},
                },
            },
        },
    };

    const renderAt = (path: string, teamName: string, initialState: object = state) => {
        const history = createMemoryHistory({initialEntries: [path]});
        renderWithContext(
            <Route path={APP_LINK_PATHS}>
                <AppLinkRedirect teamName={teamName}/>
            </Route>,
            initialState,
            {history},
        );
        return history;
    };

    test.each([
        ['/playbooks/runs/abcdefghijklmnopqrstuvwxyz', '/lumen/playbooks/runs/abcdefghijklmnopqrstuvwxyz', '', ''],
        ['/playbooks/runs/abcdefghijklmnopqrstuvwxyz', '/lumen/playbooks/runs/abcdefghijklmnopqrstuvwxyz', '?from=digest_assignedtask', '#playbook-run-retrospective'],
        ['/playbooks/playbooks/abcdefghijklmnopqrstuvwxyz/outline', '/lumen/playbooks/playbooks/abcdefghijklmnopqrstuvwxyz/outline', '', ''],
        ['/playbooks', '/lumen/playbooks', '', ''],
    ])('opens %s in the last community, keeping its query and hash', async (from, to, search, hash) => {
        const history = renderAt(from + search + hash, 'lumen');

        await waitFor(() => {
            expect(history.location.pathname).toBe(to);
        });
        expect(history.location.search).toBe(search);
        expect(history.location.hash).toBe(hash);
    });

    test('uses the first of your communities when none was viewed yet', async () => {
        const history = renderAt('/playbooks/runs', '');

        await waitFor(() => {
            expect(history.location.pathname).toBe('/alpha/playbooks/runs');
        });
    });

    test('goes home when you are in no community', async () => {
        const history = renderAt('/playbooks/runs', '', {entities: {teams: {teams: {}, myMembers: {}}}});

        await waitFor(() => {
            expect(history.location.pathname).toBe('/');
        });
    });

    test('only answers the app paths', () => {
        expect(APP_LINK_PATHS).toEqual(['/playbooks']);
    });
});
