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

    /** Where the tile leads, or, for plugin entries, what clicking it does. */
    link?: string;
    onClick?: () => void;
};

const IntegrationOption = ({icon, title, description, link, onClick}: Props) => {
    const className = `integration-option integration-option--${icon}`;
    const content = (
        <>
            <span className='integration-option__image'>
                <Icon name={icon}/>
            </span>
            <span className='integration-option__text'>
                <span className='integration-option__title'>
                    {title}
                </span>
                <span className='integration-option__description'>
                    {description}
                </span>
            </span>
        </>
    );
    if (!link) {
        return (
            <button
                type='button'
                className={className}
                onClick={onClick}
            >
                {content}
            </button>
        );
    }
    return (
        <Link
            to={link}
            className={className}
        >
            {content}
        </Link>
    );
};

export default React.memo(IntegrationOption);
