// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {render, screen} from '@testing-library/react';
import React from 'react';
import {IntlProvider} from 'react-intl';

import {IntegrationTag} from 'fusion/agents/agent_status';

import UserTag from './index';

// The real useIntl, which reads the IntlProvider below (the shared setup pins it to English).
jest.mock('react-intl', () => jest.requireActual('react-intl'));

describe('fusion/components/user_tag', () => {
    const translated = (node: React.ReactNode) => render(
        <IntlProvider
            locale='fr'
            messages={{'fusion.message.bot': 'ROBOT', 'fusion.message.guest': 'INVITÉ'}}
        >
            {node}
        </IntlProvider>,
    );

    test('the BOT and GUEST tags are translated', () => {
        translated(
            <>
                <UserTag kind='bot'/>
                <UserTag kind='guest'/>
            </>,
        );
        expect(screen.getByText('ROBOT')).toHaveClass('am-bot-tag');
        expect(screen.getByText('INVITÉ')).toHaveClass('am-bot-tag', 'am-guest-tag');
    });

    test("a bot's integration tag is the translated BOT tag", () => {
        translated(<IntegrationTag agent={false}/>);
        expect(screen.getByText('ROBOT')).toHaveClass('am-bot-tag');
    });
});
