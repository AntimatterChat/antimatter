// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {navigateTo, reloadPage} from 'utils/browser_utils';

import {
    CURRENT_WEB_UI,
    WEB_UI_COOKIE,
    WebUIs,
    captureRequestedWebUI,
    followPreferredWebUI,
    loadWebUI,
    takeRequestedWebUI,
} from './web_ui';

jest.mock('utils/browser_utils', () => ({
    navigateTo: jest.fn(),
    reloadPage: jest.fn(),
}));

const OTHER_WEB_UI = CURRENT_WEB_UI === WebUIs.CLASSIC ? WebUIs.FUSION : WebUIs.CLASSIC;

function webUICookie() {
    return document.cookie.split('; ').find((c) => c.startsWith(WEB_UI_COOKIE + '='))?.split('=')[1];
}

describe('utils/web_ui', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        jest.restoreAllMocks();
        window.sessionStorage.clear();
        document.cookie = `${WEB_UI_COOKIE}=;path=/;max-age=0`;
    });

    describe('followPreferredWebUI', () => {
        test('keeps the page when it is served the preferred web UI', () => {
            expect(followPreferredWebUI(CURRENT_WEB_UI)).toBe(false);
            expect(reloadPage).not.toHaveBeenCalled();
            expect(webUICookie()).toBe(CURRENT_WEB_UI);
        });

        test('reloads the page in the preferred web UI', () => {
            expect(followPreferredWebUI(OTHER_WEB_UI)).toBe(true);
            expect(reloadPage).toHaveBeenCalledTimes(1);
            expect(webUICookie()).toBe(OTHER_WEB_UI);
        });

        test('does not reload again for the same web UI within a minute', () => {
            const now = Date.now();
            jest.spyOn(Date, 'now').mockReturnValue(now);
            expect(followPreferredWebUI(OTHER_WEB_UI)).toBe(true);

            jest.spyOn(Date, 'now').mockReturnValue(now + (30 * 1000));
            expect(followPreferredWebUI(OTHER_WEB_UI)).toBe(false);
            expect(reloadPage).toHaveBeenCalledTimes(1);

            jest.spyOn(Date, 'now').mockReturnValue(now + (61 * 1000));
            expect(followPreferredWebUI(OTHER_WEB_UI)).toBe(true);
            expect(reloadPage).toHaveBeenCalledTimes(2);
        });

        test('does not reload when it cannot remember reloading', () => {
            jest.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
                throw new Error('storage disabled');
            });
            expect(followPreferredWebUI(OTHER_WEB_UI)).toBe(false);
            expect(reloadPage).not.toHaveBeenCalled();
        });
    });

    test('loadWebUI loads the current page in the given web UI', () => {
        window.history.replaceState(null, '', '/team/channels/town-square?q=1');
        loadWebUI(OTHER_WEB_UI);
        expect(navigateTo).toHaveBeenCalledWith(`${window.location.origin}/team/channels/town-square?q=1&webui=${OTHER_WEB_UI}`);
    });

    describe('captureRequestedWebUI', () => {
        test('keeps the requested web UI once and removes it from the address', () => {
            window.history.replaceState(null, '', `/team/channels/town-square?webui=${CURRENT_WEB_UI}&q=1`);
            captureRequestedWebUI();
            expect(window.location.search).toBe('?q=1');
            expect(takeRequestedWebUI()).toBe(CURRENT_WEB_UI);
            expect(takeRequestedWebUI()).toBeNull();
        });

        test('ignores a web UI other than the current one', () => {
            window.history.replaceState(null, '', `/team/channels/town-square?webui=${OTHER_WEB_UI}`);
            captureRequestedWebUI();
            expect(window.location.search).toBe('');
            expect(takeRequestedWebUI()).toBeNull();
        });
    });
});
