// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {ChannelType} from '@mattermost/types/channels';
import type {OutgoingWebhook} from '@mattermost/types/integrations';
import type {Team} from '@mattermost/types/teams';
import type {DeepPartial} from '@mattermost/types/utilities';

import AbstractOutgoingWebhook from 'components/integrations/abstract_outgoing_webhook';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';

const initialState: DeepPartial<GlobalState> = {
    entities: {
        channels: {
            currentChannelId: 'channel_id',
            channels: {
                channel_id: TestHelper.getChannelMock({id: 'channel_id', team_id: 'team_id', type: 'O' as ChannelType, name: 'general'}),
            },
            myMembers: {
                channel_id: TestHelper.getChannelMembershipMock({channel_id: 'channel_id'}),
            },
            channelsInTeam: {
                team_id: new Set(['channel_id']),
            },
        },
        teams: {
            currentTeamId: 'team_id',
            teams: {
                team_id: TestHelper.getTeamMock({id: 'team_id'}),
            },
            myMembers: {
                team_id: TestHelper.getTeamMembershipMock({roles: 'team_roles'}),
            },
        },
    },
};

// Message actions (Antimatter): outgoing webhooks triggered from the message menu.
describe('components/integrations/AbstractOutgoingWebhook, message actions', () => {
    const team = TestHelper.getTeamMock({id: 'team_id', name: 'team_name'}) as Team;
    const hook = (extra: Partial<OutgoingWebhook> = {}): OutgoingWebhook => ({
        id: 'hook_id',
        token: 'token',
        create_at: 1,
        update_at: 1,
        delete_at: 0,
        creator_id: 'creator',
        channel_id: '',
        team_id: 'team_id',
        trigger_words: [],
        trigger_when: 2,
        callback_urls: ['https://example.com/hook'],
        display_name: 'Archive',
        description: '',
        content_type: 'application/json',
        username: '',
        icon_url: '',
        ...extra,
    });

    const render = (props: Partial<React.ComponentProps<typeof AbstractOutgoingWebhook>> = {}) => {
        const action = jest.fn(() => Promise.resolve());
        const result = renderWithContext(
            <AbstractOutgoingWebhook
                team={team}
                header={{id: 'header', defaultMessage: 'Header'}}
                footer={{id: 'footer', defaultMessage: 'Save'}}
                loading={{id: 'loading', defaultMessage: 'Saving'}}
                renderExtra=''
                serverError=''
                enablePostUsernameOverride={false}
                enablePostIconOverride={false}
                action={action}
                {...props}
            />,
            initialState as GlobalState,
        );
        return {...result, action};
    };

    test('offers the message menu as a trigger, which hides the trigger words', async () => {
        const {container} = render({initialHook: hook({trigger_when: 0, trigger_words: ['archive']})});
        expect(container.querySelector('#triggerWords')).toBeInTheDocument();

        await userEvent.selectOptions(container.querySelector('#triggerWhen') as HTMLSelectElement, '2');
        expect(container.querySelector('#triggerWords')).not.toBeInTheDocument();
        expect(screen.getByText(/every public channel of the community/)).toBeInTheDocument();
    });

    test('a message action is saved without trigger words nor a channel', async () => {
        const {container, action} = render({initialHook: hook({trigger_when: 0, trigger_words: ['archive']})});
        await userEvent.selectOptions(container.querySelector('#triggerWhen') as HTMLSelectElement, '2');
        await userEvent.click(container.querySelector('#saveWebhook') as HTMLButtonElement);

        expect(action).toHaveBeenCalledTimes(1);
        const [saved, iconFile] = action.mock.calls[0] as unknown as [OutgoingWebhook, File | null];
        expect(saved.trigger_when).toBe(2);
        expect(saved.trigger_words).toEqual([]);
        expect(saved.channel_id).toBe('');
        expect(iconFile).toBeNull();
    });

    test('a message action needs a title, its name in the menu', async () => {
        const {container, action} = render({initialHook: hook({display_name: ''})});
        await userEvent.click(container.querySelector('#saveWebhook') as HTMLButtonElement);
        expect(action).not.toHaveBeenCalled();
        expect(screen.getByText(/A title is required/)).toBeInTheDocument();
    });

    test('its icon is a URL, shown even without post icon overrides', async () => {
        const {container, action} = render({initialHook: hook()});
        expect(container.querySelector('#iconURL')).not.toBeInTheDocument();
        await userEvent.type(container.querySelector('#messageActionIconURL') as HTMLInputElement, 'https://example.com/icon.png');
        await userEvent.click(container.querySelector('#saveWebhook') as HTMLButtonElement);
        expect((action.mock.calls[0] as unknown as [OutgoingWebhook])[0].icon_url).toBe('https://example.com/icon.png');
    });

    test('or an uploaded image, handed over to be uploaded once saved', async () => {
        const {container, action} = render({initialHook: hook(), canUploadIcon: true});
        const file = new File(['png'], 'icon.png', {type: 'image/png'});
        await userEvent.upload(container.querySelector('#messageActionIconFile') as HTMLInputElement, file);
        await userEvent.click(container.querySelector('#saveWebhook') as HTMLButtonElement);
        const [saved, iconFile] = action.mock.calls[0] as unknown as [OutgoingWebhook, File | null];
        expect(iconFile).toBe(file);
        expect(saved.icon_url).toBe('');
    });

    test('an uploaded icon is shown, and can be removed', async () => {
        const {container, action} = render({initialHook: hook({icon_url: '/api/v4/hooks/outgoing/hook_id/icon?_=1'})});
        expect(container.querySelector('#messageActionIconURL')).not.toBeInTheDocument();
        expect(screen.getByRole('img')).toHaveAttribute('src', expect.stringMatching(/\/api\/v4\/hooks\/outgoing\/hook_id\/icon\?_=1$/));

        await userEvent.click(screen.getByRole('button', {name: 'Remove the icon'}));
        expect(container.querySelector('#messageActionIconURL')).toBeInTheDocument();
        await userEvent.click(container.querySelector('#saveWebhook') as HTMLButtonElement);
        expect((action.mock.calls[0] as unknown as [OutgoingWebhook])[0].icon_url).toBe('');
    });

    test('no upload where it isn\'t offered (the classic pages)', () => {
        const {container} = render({initialHook: hook()});
        expect(container.querySelector('#messageActionIconFile')).not.toBeInTheDocument();
    });
});
