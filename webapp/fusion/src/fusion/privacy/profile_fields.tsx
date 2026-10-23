// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import {getConfig} from 'mattermost-redux/selectors/entities/general';

import {Toggle} from 'fusion/modals/settings_controls';

import type {ProfileField} from './profile_privacy';
import {PROFILE_FIELDS} from './profile_privacy';

// ProfileFieldToggles are the profile details someone shows, one switch each (on: shown). The full name and the
// email are offered only when the server shows them at all.
export default function ProfileFieldToggles({hidden, onChange}: {hidden: ProfileField[]; onChange: (hidden: ProfileField[]) => void}) {
    const {formatMessage} = useIntl();
    const config = useSelector(getConfig);

    const labels: Record<ProfileField, [string, string]> = {
        full_name: [
            formatMessage({id: 'fusion.profilePrivacy.fullName', defaultMessage: 'Full name'}),
            formatMessage({id: 'fusion.profilePrivacy.fullNameDesc', defaultMessage: 'Your first and last name. Hidden, you show as your username or nickname.'}),
        ],
        nickname: [
            formatMessage({id: 'fusion.profilePrivacy.nickname', defaultMessage: 'Nickname'}),
            formatMessage({id: 'fusion.profilePrivacy.nicknameDesc', defaultMessage: 'The name you chose to be called.'}),
        ],
        position: [
            formatMessage({id: 'fusion.profilePrivacy.position', defaultMessage: 'Position'}),
            formatMessage({id: 'fusion.profilePrivacy.positionDesc', defaultMessage: 'Your role or job title.'}),
        ],
        email: [
            formatMessage({id: 'fusion.profilePrivacy.email', defaultMessage: 'Email address'}),
            formatMessage({id: 'fusion.profilePrivacy.emailDesc', defaultMessage: 'Hidden, nobody finds you by it either.'}),
        ],
        local_time: [
            formatMessage({id: 'fusion.profilePrivacy.localTime', defaultMessage: 'Local time'}),
            formatMessage({id: 'fusion.profilePrivacy.localTimeDesc', defaultMessage: 'Your time zone, and the time it is for you.'}),
        ],
        attributes: [
            formatMessage({id: 'fusion.profilePrivacy.attributes', defaultMessage: 'Attributes'}),
            formatMessage({id: 'fusion.profilePrivacy.attributesDesc', defaultMessage: 'The attributes on your profile, such as who you work for or your site. The community\'s admins still see them.'}),
        ],
    };

    const offered = PROFILE_FIELDS.filter((field) => {
        if (field === 'full_name') {
            // The server sends it, though the client config's type doesn't name it.
            return (config as unknown as Record<string, string | undefined>).ShowFullName !== 'false';
        }
        if (field === 'email') {
            return config.ShowEmailAddress === 'true';
        }
        return true;
    });

    return (
        <>
            {offered.map((field) => (
                <Toggle
                    key={field}
                    title={labels[field][0]}
                    desc={labels[field][1]}
                    on={!hidden.includes(field)}
                    onChange={(on) => onChange(on ? hidden.filter((f) => f !== field) : [...hidden, field])}
                />
            ))}
        </>
    );
}
