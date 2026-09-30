// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getCurrentUser} from 'mattermost-redux/selectors/entities/users';
import {isSystemAdmin} from 'mattermost-redux/utils/user_utils';

import Icon from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

// AboutPopover says what Antimatter is, with version details for system admins.
export default function AboutPopover({anchor, onClose}: {anchor: HTMLElement | null; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const config = useSelector(getConfig);
    const admin = useSelector((state: GlobalState) => isSystemAdmin(getCurrentUser(state)?.roles || ''));
    const year = new Date().getFullYear();
    const build: Array<[string, string]> = [
        [formatMessage({id: 'fusion.about.server', defaultMessage: 'Server'}), config.Version || ''],
        [formatMessage({id: 'fusion.about.build', defaultMessage: 'Build'}), [config.BuildNumber, config.BuildHash?.slice(0, 7)].filter(Boolean).join(' · ')],
        [formatMessage({id: 'fusion.about.date', defaultMessage: 'Build date'}), config.BuildDate || ''],
        [formatMessage({id: 'fusion.about.database', defaultMessage: 'Database'}), [config.SQLDriverName, config.SchemaVersion].filter(Boolean).join(' · ')],
    ];

    return (
        <Popover
            anchor={anchor}
            placement='beside'
            className='about-pop'
            label={formatMessage({id: 'fusion.about.label', defaultMessage: 'About Antimatter'})}
            onClose={onClose}
        >
            <div className={am('about-head')}>
                <svg
                    className={am('about-mark')}
                    viewBox='0 0 64 64'
                    aria-hidden='true'
                >
                    <use href='#am-i-mark'/>
                </svg>
                <div>
                    <h3>{'Antimatter'}</h3>
                    <p>{formatMessage({id: 'fusion.about.tagline', defaultMessage: 'Chat, voice and threads for your team. Free software.'})}</p>
                </div>
            </div>
            {admin && (
                <>
                    <dl className={am('about-build')}>
                        {build.filter(([, v]) => v).map(([k, v]) => (
                            <div key={k}>
                                <dt>{k}</dt>
                                <dd>{v}</dd>
                            </div>
                        ))}
                    </dl>
                    <p className={am('about-note')}>
                        <Icon
                            name='shield'
                            size='xs'
                        />
                        {formatMessage({id: 'fusion.about.adminOnly', defaultMessage: 'Version details are only shown to system admins.'})}
                    </p>
                </>
            )}
            <p className={am('about-legal')}>
                {formatMessage({id: 'fusion.about.license', defaultMessage: 'Antimatter is licensed under the GNU AGPL v3. Copyright {year} the Antimatter contributors.'}, {year})}
                <br/>
                {formatMessage({id: 'fusion.about.basedOn', defaultMessage: 'Based on Mattermost. Copyright 2015 – {year} Mattermost, Inc. All rights reserved.'}, {year})}
            </p>
        </Popover>
    );
}
