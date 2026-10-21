// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {mapStateToProps} from './index';

jest.mock('mattermost-redux/selectors/entities/channels', () => ({
    getCurrentChannel: () => undefined,
    getUnreadStatus: () => 3,
}));
jest.mock('mattermost-redux/selectors/entities/general', () => ({getConfig: () => ({SiteName: 'Antimatter'})}));
jest.mock('mattermost-redux/selectors/entities/teams', () => ({getCurrentTeam: () => undefined}));

// Fusion's focus mode hides your unread marks: the tab's title and icon, and the desktop app's badge, which all come
// from unreadStatus.
describe('components/unreads_status_handler mapStateToProps', () => {
    const location = {pathname: '/team/channels/town-square'} as never;

    test('gives the unread status (3 mentions)', () => {
        expect(mapStateToProps({} as never, {location}).unreadStatus).toBe(3);
    });

    test('gives none while the unread marks hide, and the mentions again after', () => {
        expect(mapStateToProps({} as never, {location, hideUnreads: true}).unreadStatus).toBe(false);
        expect(mapStateToProps({} as never, {location, hideUnreads: false}).unreadStatus).toBe(3);
    });
});
