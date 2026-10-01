// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {FormattedMessage} from 'react-intl';

import {Button} from '@mattermost/shared/components/button';

import IPNotInRangeErrorPanel from './edit_section_ip_not_in_range_panel';

type EditSectionHeaderProps = {
    setShowAddModal: (show: boolean) => void;
    currentIPIsInRange: boolean;
    currentUsersIP: string | null;
};

const EditSectionHeader = ({
    setShowAddModal,
    currentIPIsInRange,
    currentUsersIP,
}: EditSectionHeaderProps) => (
    <div className='AllowedIPAddressesSection'>
        <div className='SectionHeaderContent'>
            <div className='HeaderContent'>
                <div className='TitleSubtitle'>
                    <div className='Title'>
                        <FormattedMessage
                            id='admin.ip_filtering.rules_title'
                            defaultMessage='IP Filter Rules'
                        />
                    </div>
                    <div className='Subtitle'>
                        <FormattedMessage
                            id='admin.ip_filtering.edit_section_description_line_1'
                            defaultMessage='Deny rules always block the addresses they match. If any allow rule exists, only addresses matching an allow rule can reach the server.'
                        />
                    </div>
                    <div className='Subtitle'>
                        <FormattedMessage
                            id='admin.ip_filtering.edit_section_description_line_2'
                            defaultMessage='<strong>NOTE:</strong> If no rules are enabled, all IP addresses are allowed.'
                            values={{
                                strong: (msg) => <strong>{msg}</strong>,
                            }}
                        />
                    </div>
                </div>
                <div className='AddIPFilterButton'>
                    <Button
                        emphasis='primary'
                        onClick={() => {
                            setShowAddModal(true);
                        }}
                        type='button'
                    >
                        <FormattedMessage
                            id='admin.ip_filtering.add_filter'
                            defaultMessage='Add Filter'
                        />
                    </Button>
                </div>
            </div>
            {
                !currentIPIsInRange &&
                    <IPNotInRangeErrorPanel
                        setShowAddModal={setShowAddModal}
                        currentUsersIP={currentUsersIP}
                    />
            }
        </div>
    </div>
);

export default EditSectionHeader;
