// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {FormattedMessage} from 'react-intl';

import {AlertOutlineIcon} from '@mattermost/compass-icons/components';
import {Button} from '@mattermost/shared/components/button';

type IPNotInRangeErrorPanelProps = {
    currentUsersIP: string | null;
    setShowAddModal: (show: boolean) => void;
};

const IPNotInRangeErrorPanel = ({
    currentUsersIP,
    setShowAddModal,
}: IPNotInRangeErrorPanelProps) => (
    <div className='NotInRangeErrorPanel'>
        <div className='Icon'>
            <AlertOutlineIcon size={20}/>
        </div>
        <div className='Content'>
            <div className='Title'>
                <FormattedMessage
                    id='admin.ip_filtering.your_current_ip_would_be_blocked'
                    defaultMessage='These rules would block your own IP address {ip}.'
                    values={{ip: currentUsersIP}}
                />
            </div>
            <div className='Body'>
                <FormattedMessage
                    id='admin.ip_filtering.include_your_ip'
                    defaultMessage='Allow your IP address, or remove the deny rule matching it, to continue.'
                />
                <Button
                    emphasis='primary'
                    onClick={() => setShowAddModal(true)}
                >
                    <FormattedMessage
                        id='admin.ip_filtering.add_your_ip'
                        defaultMessage='Add your IP address'
                    />
                </Button>
            </div>
        </div>
    </div>
);

export default IPNotInRangeErrorPanel;
