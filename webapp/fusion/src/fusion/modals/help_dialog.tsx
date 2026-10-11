// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useLayoutEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';

import HelpAttaching from 'components/help/attaching';
import HelpCommands from 'components/help/commands';
import HelpFormatting from 'components/help/formatting';
import type {HelpPage} from 'components/help/help_links';
import HelpMentioning from 'components/help/mentioning';
import HelpMessaging from 'components/help/messaging';
import HelpSending from 'components/help/sending';

import Icon from 'fusion/components/icon';
import {Dialog} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';

export type {HelpPage};

const PAGES: Array<{page: HelpPage; label: {id: string; defaultMessage: string}; Component: React.ComponentType}> = [
    {page: 'messaging', label: {id: 'fusion.help.messaging', defaultMessage: 'Messaging'}, Component: HelpMessaging},
    {page: 'sending', label: {id: 'fusion.help.sending', defaultMessage: 'Sending'}, Component: HelpSending},
    {page: 'formatting', label: {id: 'fusion.help.formatting', defaultMessage: 'Formatting'}, Component: HelpFormatting},
    {page: 'mentioning', label: {id: 'fusion.help.mentioning', defaultMessage: 'Mentions'}, Component: HelpMentioning},
    {page: 'attaching', label: {id: 'fusion.help.attaching', defaultMessage: 'Files'}, Component: HelpAttaching},
    {page: 'commands', label: {id: 'fusion.help.commands', defaultMessage: 'Commands'}, Component: HelpCommands},
];

// HelpDialog shows the help pages (messaging, formatting, mentions, files, commands) in a dialog, instead of a new
// tab: a tab per page, and the pages' links to one another switch tabs.
export default function HelpDialog({page = 'messaging', onClose}: {page?: HelpPage; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const [current, setCurrent] = useState<HelpPage>(page);
    const body = useRef<HTMLDivElement>(null);
    const {Component} = PAGES.find((p) => p.page === current) || PAGES[0];

    // The pages name the tab they're in: the window's title comes back when the dialog closes.
    useLayoutEffect(() => {
        const title = document.title;
        return () => {
            document.title = title;
        };
    }, []);
    useLayoutEffect(() => {
        body.current?.scrollTo?.(0, 0);
    }, [current]);

    const onClick = (e: React.MouseEvent) => {
        const href = (e.target as Element).closest?.('a[href]')?.getAttribute('href') || '';
        const match = (/^(?:\/_popout)?\/help(?:\/([a-z]+))?\/?$/).exec(href);
        if (match) {
            e.preventDefault();
            setCurrent((match[1] || 'messaging') as HelpPage);
        }
    };

    return (
        <Dialog
            label={formatMessage({id: 'fusion.help.title', defaultMessage: 'Help'})}
            onClose={onClose}
            small={false}
            className={am('wide', 'help-dialog')}
        >
            <div className={am('help-head')}>
                <div
                    className={am('seg')}
                    role='tablist'
                >
                    {PAGES.map((p) => (
                        <button
                            key={p.page}
                            type='button'
                            role='tab'
                            aria-selected={p.page === current}
                            className={p.page === current ? am('on') : ''}
                            onClick={() => setCurrent(p.page)}
                        >
                            {formatMessage(p.label)}
                        </button>
                    ))}
                </div>
                <button
                    type='button'
                    className={am('icon-btn')}
                    aria-label={formatMessage({id: 'fusion.help.close', defaultMessage: 'Close'})}
                    onClick={onClose}
                >
                    <Icon name='x'/>
                </button>
            </div>
            <div
                ref={body}
                className={am('help-body')}
                role='tabpanel'
                onClick={onClick}
            >
                <Component/>
            </div>
        </Dialog>
    );
}
