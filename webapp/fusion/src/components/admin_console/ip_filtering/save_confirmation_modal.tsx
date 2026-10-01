// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {type JSX} from 'react';
import {Modal} from 'react-bootstrap';
import {FormattedMessage, useIntl} from 'react-intl';

import {InformationOutlineIcon} from '@mattermost/compass-icons/components';
import {Button} from '@mattermost/shared/components/button';

import './save_confirmation_modal.scss';

type Props = {
    onExited: () => void;
    onConfirm?: () => void;
    title?: string;
    subtitle: JSX.Element | string;
    buttonText?: string;
    includeDisclaimer?: boolean;
};

export default function SaveConfirmationModal({onExited, onConfirm, title, subtitle, includeDisclaimer, buttonText}: Props) {
    const {formatMessage} = useIntl();
    return (
        <Modal
            className={'SaveConfirmationModal'}
            dialogClassName={'SaveConfirmationModal__dialog'}
            show={true}
            onExited={onExited}
            onHide={onExited}
        >
            <Modal.Header closeButton={true}>
                <div className='title'>
                    {title}
                </div>
            </Modal.Header>
            <Modal.Body>
                {subtitle}
                {includeDisclaimer &&
                    <div className='disclaimer'>
                        <div className='Icon'>
                            <InformationOutlineIcon/>
                        </div>
                        <div className='Body'>
                            <div className='Title'>{formatMessage({id: 'admin.ip_filtering.save_disclaimer_title', defaultMessage: 'Restoring access'})}</div>
                            <div className='Subtitle'>
                                <FormattedMessage
                                    id={'admin.ip_filtering.save_disclaimer_subtitle'}
                                    defaultMessage={'If these rules block you, an administrator with access to the server can clear them in local mode, which is not subject to IP filtering: <code>mmctl --local config edit</code>, then empty <code>IPFilteringSettings.Rules</code>.'}
                                    values={{
                                        code: (msg) => <code>{msg}</code>,
                                    }}
                                />
                            </div>
                        </div>
                    </div>
                }
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
                    data-testid='save-confirmation-button'
                    type='button'
                    emphasis='primary'
                    variant='destructive'
                    onClick={() => onConfirm?.()}
                >
                    {buttonText}
                </Button>
            </Modal.Footer>
        </Modal>
    );
}
