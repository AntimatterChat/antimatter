// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {type JSX} from 'react';
import {Link} from 'react-router-dom';

import Icon from 'fusion/components/icon';
import type {IconName} from 'fusion/components/icon';

type Props = {

    /** The Fusion UI draws each kind of integration with one of its icons, in a tile of its colour (see
     * _97_integrations.scss), rather than the classic illustrations. */
    icon: IconName;
    title: JSX.Element;
    description: JSX.Element;
    link: string;
};

const IntegrationOption = ({icon, title, description, link}: Props) => {
    return (
        <Link
            to={link}
            className={`integration-option integration-option--${icon}`}
        >
            <span className='integration-option__image'>
                <Icon name={icon}/>
            </span>
            <div className='integration-option__title'>
                {title}
            </div>
            <div className='integration-option__description'>
                {description}
            </div>
        </Link>
    );
};

export default React.memo(IntegrationOption);
