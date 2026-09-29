// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import StatisticCount from 'components/analytics/statistic_count';

import Title from './title';

type ActivatedUserCardProps = {
    activatedUsers: number | undefined;
};

const ActivatedUserCard = ({activatedUsers}: ActivatedUserCardProps) => {
    return (
        <StatisticCount
            title={<Title/>}
            icon='fa-users'
            count={activatedUsers}
            id='totalActiveUsers'
        />
    );
};

export default ActivatedUserCard;
