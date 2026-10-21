// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {Client4} from 'mattermost-redux/client';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getReportAProblemLink} from 'mattermost-redux/selectors/entities/report_a_problem';
import {getCurrentUser} from 'mattermost-redux/selectors/entities/users';
import {isSystemAdmin} from 'mattermost-redux/utils/user_utils';

import {getUserGuideDropdownPluginMenuItems} from 'selectors/plugins';

import ExternalLink from 'components/external_link';
import Markdown from 'components/markdown';

import Icon from 'fusion/components/icon';
import {Dialog, Popover} from 'fusion/components/layer';
import {MenuHeading, MenuItem} from 'fusion/components/menu';
import {useDialogs} from 'fusion/shell/dialogs_context';
import {am} from 'fusion/utils/class_names';
import {openKeyboardShortcuts} from 'fusion/utils/modals';

import type {GlobalState} from 'types/store';

import {appDownloadLink} from './admin_links';

// Where Antimatter's source code is (AGPL §13). The server has no setting for it.
const SOURCE_CODE_URL = 'https://github.com/AntimatterChat/antimatter';

function ExternalAnchor({href, className, children}: {href: string; className?: string; children: React.ReactNode}) {
    return (
        <ExternalLink
            className={className}
            href={href}
            location='fusion_about'
        >
            {children}
        </ExternalLink>
    );
}

// TermsDialog shows the server's own terms of service, which everyone accepted on signing in.
function TermsDialog({onClose}: {onClose: () => void}) {
    const {formatMessage} = useIntl();
    const [text, setText] = useState<string | null>(null);
    useEffect(() => {
        Client4.getTermsOfService().then((terms) => setText(terms.text)).catch(() => setText(''));
    }, []);
    const title = formatMessage({id: 'fusion.about.customTerms', defaultMessage: 'Terms of service'});
    return (
        <Dialog
            label={title}
            small={false}
            onClose={onClose}
        >
            <div className={am('pane', 'terms-pane')}>
                <h2>{title}</h2>
                {text === null ? <p className={am('lead')}>{formatMessage({id: 'fusion.about.loading', defaultMessage: 'Loading…'})}</p> : <Markdown message={text}/>}
            </div>
        </Dialog>
    );
}

// AboutPopover says what Antimatter is, with version details for system admins.
export default function AboutPopover({anchor, onClose}: {anchor: HTMLElement | null; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dialogs = useDialogs();
    const config = useSelector(getConfig);
    const admin = useSelector((state: GlobalState) => isSystemAdmin(getCurrentUser(state)?.roles || ''));
    const year = new Date().getFullYear();
    const dispatch = useDispatch();
    const reportLink = useSelector(getReportAProblemLink);
    const helpItems = useSelector(getUserGuideDropdownPluginMenuItems);
    const downloadLink = useSelector(appDownloadLink);
    const [termsOpen, setTermsOpen] = useState(false);
    const run = (action: () => void) => () => {
        action();
        onClose();
    };
    const build: Array<[string, string]> = [
        [formatMessage({id: 'fusion.about.server', defaultMessage: 'Server'}), config.Version || ''],
        [formatMessage({id: 'fusion.about.build', defaultMessage: 'Build'}), [config.BuildNumber, config.BuildHash?.slice(0, 7)].filter(Boolean).join(' · ')],
        [formatMessage({id: 'fusion.about.date', defaultMessage: 'Build date'}), config.BuildDate || ''],
        [formatMessage({id: 'fusion.about.database', defaultMessage: 'Database'}), [config.SQLDriverName, config.SchemaVersion].filter(Boolean).join(' · ')],
    ];

    const links = ([
        [formatMessage({id: 'fusion.about.terms', defaultMessage: 'Terms of use'}), config.TermsOfServiceLink],
        [formatMessage({id: 'fusion.about.privacy', defaultMessage: 'Privacy policy'}), config.PrivacyPolicyLink],
        [formatMessage({id: 'fusion.about.source', defaultMessage: 'Source code'}), SOURCE_CODE_URL],
    ] as Array<[string, string | undefined]>).filter((l): l is [string, string] => Boolean(l[1]));

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
                    <p>{formatMessage({id: 'fusion.about.tagline', defaultMessage: 'Chat, voice and threads for your team or community. Free software.'})}</p>
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
            {config.AboutLink && (
                <ExternalAnchor
                    className={am('about-more')}
                    href={config.AboutLink}
                >
                    {formatMessage({id: 'fusion.about.learnMore', defaultMessage: 'Learn more about Antimatter'})}
                    <Icon
                        name='popout'
                        size='xs'
                    />
                </ExternalAnchor>
            )}
            <div className={am('about-menu')}>
                <MenuHeading>{formatMessage({id: 'fusion.about.help', defaultMessage: 'Help'})}</MenuHeading>
                <MenuItem
                    icon='compass'
                    label={formatMessage({id: 'fusion.about.guide', defaultMessage: 'User guide'})}
                    onClick={run(() => dialogs.help('messaging'))}
                />
                <MenuItem
                    icon='type'
                    label={formatMessage({id: 'fusion.about.shortcuts', defaultMessage: 'Keyboard shortcuts'})}
                    onClick={run(() => dispatch(openKeyboardShortcuts()))}
                />
                <MenuItem
                    icon='text'
                    label={formatMessage({id: 'fusion.about.formatting', defaultMessage: 'Formatting help'})}
                    onClick={run(() => dialogs.help('formatting'))}
                />
                {reportLink && (
                    <MenuItem
                        icon='flag'
                        label={formatMessage({id: 'fusion.about.report', defaultMessage: 'Report a problem'})}
                        onClick={run(() => window.open(reportLink, '_blank', 'noopener'))}
                    />
                )}
                {downloadLink && (
                    <MenuItem
                        icon='phone'
                        label={formatMessage({id: 'fusion.about.download', defaultMessage: 'Download the apps', description: 'About menu of the app rail: opens the page with the desktop and mobile apps to download (the server\'s app download link).'})}
                        onClick={run(() => window.open(downloadLink, '_blank', 'noopener,noreferrer'))}
                    />
                )}
                {config.EnableCustomTermsOfService === 'true' && (
                    <MenuItem
                        icon='shield'
                        label={formatMessage({id: 'fusion.about.customTerms', defaultMessage: 'Terms of service'})}
                        onClick={() => setTermsOpen(true)}
                    />
                )}
                {helpItems.map((item) => (
                    <button
                        key={item.id}
                        type='button'
                        role='menuitem'
                        onClick={run(() => item.action?.(undefined as never))}
                    >
                        <Icon name='plug'/>
                        {item.text}
                    </button>
                ))}
            </div>
            {termsOpen && <TermsDialog onClose={() => setTermsOpen(false)}/>}
            <p className={am('about-legal')}>
                {formatMessage({id: 'fusion.about.license', defaultMessage: 'Antimatter is licensed under the GNU AGPL v3. Copyright {year} the Antimatter contributors.'}, {year})}
                <br/>
                {formatMessage({id: 'fusion.about.basedOn', defaultMessage: 'Based on Mattermost. Copyright 2015 – {year} Mattermost, Inc. All rights reserved.'}, {year})}
            </p>
            <div className={am('about-links')}>
                {links.map(([label, href], i) => (
                    <React.Fragment key={href}>
                        {i > 0 && <span aria-hidden='true'>{'·'}</span>}
                        <ExternalAnchor href={href}>{label}</ExternalAnchor>
                    </React.Fragment>
                ))}
            </div>
        </Popover>
    );
}
