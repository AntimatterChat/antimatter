// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

import ExternalLink from 'components/external_link';
import BaseSettingItem from 'components/widgets/modals/components/base_setting_item';
import PublicPrivateSelector from 'components/widgets/public-private-selector/public-private-selector';

import {Constants} from 'utils/constants';

type Props = {
    isPublic: boolean;
    isGroupConstrained?: boolean;
    onChange: (isPublic: boolean) => void;
};

const OpenInvite = ({isPublic, isGroupConstrained, onChange}: Props) => {
    const {formatMessage} = useIntl();

    if (isGroupConstrained) {
        const groupConstrainedContent = (
            <p id='groupConstrainedContent'>{
                formatMessage({
                    id: 'fusion.communityAccess.groupConstrained',
                    defaultMessage: 'Members of this community are added and removed by linked groups. <link>Learn more</link>',
                }, {
                    link: (msg: React.ReactNode) => (
                        <ExternalLink
                            href='https://antimatter.chat/docs/admins/authentication.html#sso'
                            location='open_invite'
                        >
                            {msg}
                        </ExternalLink>
                    ),
                })}
            </p>
        );
        return (
            <BaseSettingItem
                className='access-invite-domains-section'
                title={formatMessage({
                    id: 'team_settings.discoverability.title',
                    defaultMessage: 'Discoverability',
                })}
                description={formatMessage({
                    id: 'fusion.communityAccess.publicDescription',
                    defaultMessage: 'Public communities are listed for everyone on the server, so anyone with an account can find and join them.',
                })}
                descriptionAboveContent={true}
                content={groupConstrainedContent}
            />
        );
    }

    const handleChange = (selected: string) => {
        onChange(selected === Constants.OPEN_CHANNEL);
    };

    const selectorContent = (
        <div className='TeamAccessTab__discoverabilitySelector'>
            <PublicPrivateSelector
                selected={isPublic ? Constants.OPEN_CHANNEL : Constants.PRIVATE_CHANNEL}
                publicButtonProps={{
                    title: formatMessage({
                        id: 'fusion.communityAccess.publicTitle',
                        defaultMessage: 'Public community',
                    }),
                    description: formatMessage({
                        id: 'team_settings.discoverability.public_description',
                        defaultMessage: 'Anyone on the server can find and join',
                    }),
                }}
                privateButtonProps={{
                    title: formatMessage({
                        id: 'fusion.communityAccess.privateTitle',
                        defaultMessage: 'Private community',
                    }),
                    description: formatMessage({
                        id: 'team_settings.discoverability.private_description',
                        defaultMessage: 'Only invited members can join',
                    }),
                }}
                onChange={handleChange}
            />
        </div>
    );

    return (
        <BaseSettingItem
            className='access-invite-domains-section'
            title={formatMessage({
                id: 'team_settings.discoverability.title',
                defaultMessage: 'Discoverability',
            })}
            description={formatMessage({
                id: 'fusion.communityAccess.discoverabilityDescription',
                defaultMessage: 'Public communities are listed for everyone on the server, so anyone with an account can find and join them. Making a public community private regenerates its invitation code: a new invitation link, and the old one stops working.',
            })}
            descriptionAboveContent={true}
            content={selectorContent}
        />
    );
};

export default OpenInvite;
