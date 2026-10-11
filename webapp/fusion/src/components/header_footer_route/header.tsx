// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import classNames from 'classnames';
import React from 'react';
import {useSelector} from 'react-redux';
import {Link} from 'react-router-dom';

import {getConfig} from 'mattermost-redux/selectors/entities/general';

import BackButton from 'components/common/back_button';

import './header.scss';

export type HeaderProps = {
    alternateLink?: React.ReactElement<any>;
    backButtonURL?: string;
    onBackButtonClick?: React.EventHandler<React.MouseEvent>;
};

// Brand is the Antimatter mark and name, in the theme's colours (styles/auth.scss), so it reads on light and dark.
export const Brand = () => (
    <span className='am-auth-brand'>
        <svg
            viewBox='0 0 64 64'
            aria-hidden='true'
        >
            <ellipse
                className='am-auth-orbit-1'
                cx='32'
                cy='32'
                rx='25'
                ry='9.5'
                transform='rotate(45 32 32)'
            />
            <ellipse
                className='am-auth-orbit-2'
                cx='32'
                cy='32'
                rx='25'
                ry='9.5'
                transform='rotate(-45 32 32)'
            />
            <path
                className='am-auth-spark'
                d='M32 20.5Q33.9 30.1 43.5 32Q33.9 33.9 32 43.5Q30.1 33.9 20.5 32Q30.1 30.1 32 20.5Z'
            />
            <circle
                className='am-auth-matter'
                cx='14.32'
                cy='14.32'
                r='5.5'
            />
            <circle
                className='am-auth-anti'
                cx='49.68'
                cy='14.32'
                r='4.25'
            />
        </svg>
        {'Antimatter'}
    </span>
);

const Header = ({alternateLink, backButtonURL, onBackButtonClick}: HeaderProps) => {
    const {SiteName} = useSelector(getConfig);

    const ariaLabel = SiteName || 'Antimatter';

    let title: React.ReactNode = SiteName;
    if (title === 'Antimatter') {
        title = <Brand/>;
    }

    return (
        <div
            data-testid='hfroute-header'
            className={classNames('hfroute-header', {'has-custom-site-name': title})}
        >
            <div className='header-main'>
                <div>
                    {title &&
                        <Link
                            data-testid='header-logo-link'
                            className='header-logo-link'
                            to='/'
                            aria-label={ariaLabel}
                        >
                            {title}
                        </Link>
                    }
                </div>
                {alternateLink}
            </div>
            {onBackButtonClick && (
                <BackButton
                    className='header-back-button'
                    url={backButtonURL}
                    onClick={onBackButtonClick}
                />
            )}
        </div>
    );
};

export default Header;
