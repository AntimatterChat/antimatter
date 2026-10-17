// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {getDesktopVersion} from '@mattermost/shared/utils/user_agent';

import {getDisplayDesktopVersion} from './desktop_version';

const ELECTRON = 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.7339.41 Electron/38.1.0 Safari/537.36';

function mockUserAgent(userAgent: string) {
    jest.spyOn(window.navigator, 'userAgent', 'get').mockReturnValue(userAgent);
    jest.spyOn(window.navigator, 'appVersion', 'get').mockReturnValue(userAgent.replace(/^Mozilla\//, ''));
}

describe('utils/desktop_version', () => {
    afterEach(() => {
        jest.restoreAllMocks();
    });

    describe('getDisplayDesktopVersion', () => {
        it('shows the Antimatter version, not the Mattermost-compatible one', () => {
            mockUserAgent(`${ELECTRON} Antimatter/0.9.0 Mattermost/6.4.0`);
            expect(getDisplayDesktopVersion()).toBe('0.9.0');

            // Comparisons keep the compatible version
            expect(getDesktopVersion()).toBe('6.4.0');
        });

        it.each([
            '0.9.0-rc-1',
            '0.9.0-dev.12',
            '0.9.0-develop.1',
            '1.2.3',
        ])('keeps the whole version %s', (version) => {
            mockUserAgent(`${ELECTRON} Antimatter/${version} Mattermost/6.4.0`);
            expect(getDisplayDesktopVersion()).toBe(version);
        });

        it('falls back to the Mattermost token for the Mattermost Desktop App', () => {
            mockUserAgent(`${ELECTRON} Mattermost/5.10.2`);
            expect(getDisplayDesktopVersion()).toBe('5.10.2');
        });

        it('ignores a token that only ends with Antimatter', () => {
            mockUserAgent(`${ELECTRON} NotAntimatter/9.9.9 Mattermost/6.4.0`);
            expect(getDisplayDesktopVersion()).toBe('6.4.0');
        });

        it('ignores an Antimatter token that is not a version', () => {
            mockUserAgent(`${ELECTRON} Antimatter/x.y Mattermost/6.4.0`);
            expect(getDisplayDesktopVersion()).toBe('6.4.0');
        });

        it('is empty outside a desktop app', () => {
            mockUserAgent(ELECTRON);
            expect(getDisplayDesktopVersion()).toBe('');
        });
    });
});
