// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {screenPicker} from './screen_picker';

const UA = {
    chrome: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36',
    headless: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/141.0.7390.37 Safari/537.36',
    edge: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36 Edg/141.0.0.0',
    opera: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36 OPR/124.0.0.0',
    electron: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) some-app/1.0.0 Chrome/140.0.7339.41 Electron/38.1.0 Safari/537.36',
    desktopApp: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.7339.41 Electron/38.1.0 Safari/537.36 Antimatter/6.0.0 Mattermost/6.0.0',
    firefox: 'Mozilla/5.0 (X11; Linux x86_64; rv:144.0) Gecko/20100101 Firefox/144.0',
    safari: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Safari/605.1.15',
    iosChrome: 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/141.0.7390.41 Mobile/15E148 Safari/604.1',
    oldEdge: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/70.0.3538.102 Safari/537.36 Edge/18.19582',
};

const env = (userAgent: string, displaySurface = true, desktop = false) => ({userAgent, displaySurface, desktop});

describe('screenPicker', () => {
    test.each([
        ['Chrome', UA.chrome],
        ['headless Chrome', UA.headless],
        ['Edge', UA.edge],
        ['Opera', UA.opera],
        ['Electron', UA.electron],
    ])('%s opens its picker on a screen, a window or a tab', (_, ua) => {
        expect(screenPicker(env(ua))).toBe('surfaces');
    });

    test('a Chromium too old for displaySurface shows its own picker', () => {
        expect(screenPicker(env(UA.chrome, false))).toBe('browser');
    });

    test.each([
        ['Firefox', UA.firefox],
        ['Safari', UA.safari],
        ['Chrome on iOS', UA.iosChrome],
        ['EdgeHTML', UA.oldEdge],
        ['an unknown browser', 'SomeBrowser/1.0'],
    ])('%s shows its own picker, even when it reports displaySurface', (_, ua) => {
        expect(screenPicker(env(ua))).toBe('browser');
    });

    test('the desktop app shows its own source picker', () => {
        expect(screenPicker(env(UA.desktopApp))).toBe('desktop');
        expect(screenPicker(env(UA.desktopApp, false))).toBe('desktop');

        // Once the desktop API handshake set window.desktop, whatever the user agent says.
        expect(screenPicker(env(UA.chrome, true, true))).toBe('desktop');
    });

    test('reads the browser it runs in by default', () => {
        // jsdom has no getDisplayMedia: its own picker, as for an unknown browser.
        expect(screenPicker()).toBe('browser');
    });
});
