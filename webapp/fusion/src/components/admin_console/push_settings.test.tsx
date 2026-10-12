// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {AdminConfig} from '@mattermost/types/config';

import {PushSettings, isMattermostPushServer} from 'components/admin_console/push_settings';

import {defaultIntl} from 'tests/helpers/intl-test-helper';
import {act, renderWithContext, screen} from 'tests/react_testing_utils';

function makeConfig(sendPushNotifications: boolean, pushNotificationServer: string) {
    return {
        EmailSettings: {
            PushNotificationServer: pushNotificationServer,
            SendPushNotifications: sendPushNotifications,
        },
        TeamSettings: {
            MaxNotificationsPerChannel: 1000,
        },
    } as AdminConfig;
}

function renderPushSettings(config: AdminConfig) {
    const ref = React.createRef<InstanceType<typeof PushSettings>>();
    const result = renderWithContext(
        <PushSettings
            intl={defaultIntl}
            config={config}
            ref={ref}
        />,
    );
    return {...result, ref};
}

describe('components/PushSettings', () => {
    test('should match snapshot, own push proxy', () => {
        const {container, ref} = renderPushSettings(makeConfig(true, 'https://push.example.com'));

        expect(ref.current!.state.pushNotificationServerType).toBe('custom');
        expect(ref.current!.state.pushNotificationServer).toBe('https://push.example.com');
        expect(screen.queryByText(/run by Mattermost, Inc./)).not.toBeInTheDocument();
        expect(container).toMatchSnapshot();
    });

    test('should match snapshot, no push notification server (the default)', () => {
        const {container, ref} = renderPushSettings(makeConfig(true, ''));

        expect(ref.current!.state.pushNotificationServerType).toBe('off');
        expect(container).toMatchSnapshot();
    });

    test('should only offer to turn push off or use your own push proxy', () => {
        renderPushSettings(makeConfig(false, ''));

        const options = Array.from(screen.getByTestId('pushNotificationServerTypedropdown').querySelectorAll('option')).map((option) => option.getAttribute('value'));
        expect(options).toEqual(['off', 'custom']);
    });

    test('should keep a Mattermost push service and warn about it', () => {
        const {ref} = renderPushSettings(makeConfig(true, 'https://push-test.mattermost.com'));

        expect(ref.current!.state.pushNotificationServerType).toBe('custom');
        expect(ref.current!.state.pushNotificationServer).toBe('https://push-test.mattermost.com');
        expect(screen.getByText(/run by Mattermost, Inc./)).toBeInTheDocument();

        const config = ref.current!.getConfigFromState(makeConfig(true, 'https://push-test.mattermost.com'));
        expect(config.EmailSettings.SendPushNotifications).toBe(true);
        expect(config.EmailSettings.PushNotificationServer).toBe('https://push-test.mattermost.com');
    });

    test('should need an address to use your own push proxy', () => {
        const {ref} = renderPushSettings(makeConfig(false, ''));
        expect(ref.current!.canSave()).toBe(true);

        act(() => {
            ref.current!.setState({pushNotificationServerType: 'custom', pushNotificationServer: ' '});
        });
        expect(ref.current!.canSave()).toBe(false);

        act(() => {
            ref.current!.setState({pushNotificationServer: 'http://push-proxy:8066'});
        });
        expect(ref.current!.canSave()).toBe(true);

        const config = ref.current!.getConfigFromState(makeConfig(false, ''));
        expect(config.EmailSettings.SendPushNotifications).toBe(true);
        expect(config.EmailSettings.PushNotificationServer).toBe('http://push-proxy:8066');
    });

    test('isMattermostPushServer', () => {
        expect(isMattermostPushServer('https://push-test.mattermost.com')).toBe(true);
        expect(isMattermostPushServer('https://global.push.mattermost.com/')).toBe(true);
        expect(isMattermostPushServer('HTTPS://Push.Mattermost.com:443')).toBe(true);
        expect(isMattermostPushServer('https://push.example.com')).toBe(false);
        expect(isMattermostPushServer('https://push.mattermost.com.example.com')).toBe(false);
        expect(isMattermostPushServer('https://notmattermost.com')).toBe(false);
        expect(isMattermostPushServer('')).toBe(false);
        expect(isMattermostPushServer('not a url')).toBe(false);
    });
});
