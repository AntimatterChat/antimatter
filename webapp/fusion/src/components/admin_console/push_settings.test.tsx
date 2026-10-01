// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {AdminConfig} from '@mattermost/types/config';

import {PushSettings} from 'components/admin_console/push_settings';

import {defaultIntl} from 'tests/helpers/intl-test-helper';
import {act, renderWithContext} from 'tests/react_testing_utils';
import {Constants} from 'utils/constants';

describe('components/PushSettings', () => {
    test('should match snapshot, TPNS selected', () => {
        const config = {
            EmailSettings: {
                PushNotificationServer: 'https://global.push.mattermost.com',
                SendPushNotifications: true,
            },
            TeamSettings: {
                MaxNotificationsPerChannel: 1000,
            },
        } as AdminConfig;

        const props = {
            intl: defaultIntl,
            config,
        };

        const ref = React.createRef<InstanceType<typeof PushSettings>>();
        const {container} = renderWithContext(
            <PushSettings
                {...props}
                ref={ref}
            />,
        );

        act(() => {
            ref.current!.handleDropdownChange('pushNotificationServerType', 'mtpns');
        });
        expect(ref.current!.state.pushNotificationServer).toBe(Constants.MTPNS);
        expect(container).toMatchSnapshot();
    });

    test('should match snapshot, custom push server', () => {
        const config = {
            EmailSettings: {
                PushNotificationServer: 'https://global.push.mattermost.com',
                SendPushNotifications: true,
            },
            TeamSettings: {
                MaxNotificationsPerChannel: 1000,
            },
        } as AdminConfig;

        const props = {
            intl: defaultIntl,
            config,
        };

        const {container} = renderWithContext(
            <PushSettings {...props}/>,
        );
        expect(container).toMatchSnapshot();
    });
});
