// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {Modal} from 'react-bootstrap';
import {useIntl} from 'react-intl';

import {InformationOutlineIcon} from '@mattermost/compass-icons/components';
import {Button} from '@mattermost/shared/components/button';
import type {AllowedIPRange, IPFilterAction} from '@mattermost/types/config';

import RadioGroup from 'components/common/radio_group';
import type {CustomMessageInputType} from 'components/widgets/inputs/input/input';
import Input from 'components/widgets/inputs/input/input';

import './add_edit_ip_filter_modal.scss';
import {validateCIDR} from './ip_filtering_utils';

type Props = {
    onExited: () => void;
    onSave: (allowedIPRange: AllowedIPRange, oldIPRange?: AllowedIPRange) => void;
    existingRange?: AllowedIPRange;
    currentIP?: string;
};

export default function IPFilteringAddOrEditModal({onExited, onSave, existingRange, currentIP}: Props) {
    const {formatMessage} = useIntl();
    const [name, setName] = useState(existingRange?.description || '');
    const [CIDR, setCIDR] = useState(existingRange?.cidr_block || '');
    const [action, setAction] = useState<IPFilterAction>(existingRange?.action ?? 'allow');

    const [CIDRError, setCIDRError] = useState<CustomMessageInputType>(null);

    const handleSave = () => {
        const allowedIPRange: AllowedIPRange = {
            cidr_block: CIDR.trim(),
            description: name,
            enabled: true,
            owner_id: existingRange?.owner_id ?? '',
            action,
        };

        if (existingRange) {
            onSave(allowedIPRange, existingRange);
        } else {
            onSave(allowedIPRange);
        }

        onExited();
    };

    const handleCIDRChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const cidr = e.target.value;
        setCIDR(cidr);
        setCIDRError(null);
    };

    const validateCIDRInput = () => {
        if (!validateCIDR(CIDR)) {
            setCIDRError({type: 'error', value: formatMessage({id: 'admin.ip_filtering.invalid_range', defaultMessage: 'Enter an IP address or a range in CIDR format'})});
        }
    };

    return (
        <Modal
            className={'IPFilteringAddOrEditModal'}
            dialogClassName={'IPFilteringAddOrEditModal__dialog'}
            show={true}
            onExited={onExited}
            onHide={onExited}
        >
            <Modal.Header closeButton={true}>
                <div className='title'>
                    {existingRange?.cidr_block ? formatMessage({id: 'admin.ip_filtering.edit_ip_filter', defaultMessage: 'Edit IP Filter'}) : formatMessage({id: 'admin.ip_filtering.add_ip_filter', defaultMessage: 'Add IP Filter'})}
                </div>
            </Modal.Header>
            <Modal.Body>
                <div className='body'>
                    <div className='current_ip_notice'>
                        <div className='Content'>
                            <span><InformationOutlineIcon/>{formatMessage({id: 'admin.ip_filtering.your_current_ip_is', defaultMessage: 'Your current IP address is {ip}'}, {ip: currentIP})}</span>
                        </div>
                    </div>
                    <div className='inputs'>
                        <div>
                            {formatMessage({id: 'admin.ip_filtering.name', defaultMessage: 'Name'})}
                            <Input
                                type='text'
                                name='name'
                                onChange={(e) => setName(e.target.value)}
                                value={name}
                                placeholder={formatMessage({id: 'admin.ip_filtering.rule_name_placeholder', defaultMessage: 'Enter a name for this rule'})}
                                required={true}
                                useLegend={false}
                            />
                        </div>
                        <div>
                            {formatMessage({id: 'admin.ip_filtering.rule_action', defaultMessage: 'Action'})}
                            <RadioGroup
                                id='ip_filter_action'
                                value={action}
                                onChange={(e) => setAction(e.target.value as IPFilterAction)}
                                values={[
                                    {
                                        key: formatMessage({id: 'admin.ip_filtering.action_allow', defaultMessage: 'Allow these addresses'}),
                                        value: 'allow',
                                        testId: 'ip-filter-action-allow',
                                    },
                                    {
                                        key: formatMessage({id: 'admin.ip_filtering.action_deny', defaultMessage: 'Deny these addresses'}),
                                        value: 'deny',
                                        testId: 'ip-filter-action-deny',
                                    },
                                ]}
                            />
                        </div>
                        <div>{formatMessage({id: 'admin.ip_filtering.ip_address_range', defaultMessage: 'IP Address Range'})}
                            <Input
                                type='text'
                                name='ip_address_range'
                                onChange={handleCIDRChange}
                                onBlur={validateCIDRInput}
                                value={CIDR}
                                placeholder={formatMessage({id: 'admin.ip_filtering.ip_range_placeholder', defaultMessage: 'Enter an IP address or range'})}
                                required={true}
                                useLegend={false}
                                customMessage={CIDRError}
                            />
                        </div>
                        <p>
                            {formatMessage({id: 'admin.ip_filtering.range_format', defaultMessage: 'Enter a single address (e.g. 192.0.2.7) or a range in CIDR format (e.g. 192.168.0.0/16 or 2001:db8::/32).'})}
                        </p>
                    </div>
                </div>
            </Modal.Body>
            <Modal.Footer>
                <Button
                    type='button'
                    emphasis='tertiary'
                    onClick={onExited}
                >
                    {formatMessage({id: 'admin.ip_filtering.cancel', defaultMessage: 'Cancel'})}
                </Button>
                <Button
                    data-testid='save-add-edit-button'
                    type='button'
                    emphasis='primary'
                    onClick={handleSave}
                    disabled={Boolean(CIDRError) || !CIDR.length || !name.length}
                >
                    {existingRange ? formatMessage({id: 'admin.ip_filtering.update_filter', defaultMessage: 'Update filter'}) : formatMessage({id: 'admin.ip_filtering.save', defaultMessage: 'Save'})}
                </Button>
            </Modal.Footer>
        </Modal>
    );
}
