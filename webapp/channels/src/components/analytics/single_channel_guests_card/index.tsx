// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {defineMessages, FormattedMessage} from 'react-intl';

import StatisticCount from 'components/analytics/statistic_count';

export const messages = defineMessages({
    singleChannelGuests: {id: 'analytics.system.singleChannelGuests', defaultMessage: 'Single-channel Guests'},
});

type SingleChannelGuestsCardProps = {
    singleChannelGuestsCount: number | undefined;
};

const SingleChannelGuestsCard = ({singleChannelGuestsCount}: SingleChannelGuestsCardProps) => {
    return (
        <StatisticCount
            title={<FormattedMessage {...messages.singleChannelGuests}/>}
            icon='fa-users'
            count={singleChannelGuestsCount}
            id='singleChannelGuests'
        />
    );
};

export default SingleChannelGuestsCard;
