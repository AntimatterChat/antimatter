// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/* eslint-disable @mattermost/use-external-link */

import omit from 'lodash/omit';
import React, {forwardRef} from 'react';

import type {ExternalLinkQueryParams} from 'components/common/hooks/use_external_link';
import {useExternalLink} from 'components/common/hooks/use_external_link';

import {isMattermostURL} from 'utils/mattermost_links';

type Props = React.AnchorHTMLAttributes<HTMLAnchorElement> & {
    href: string;
    target?: string;
    rel?: string;
    onClick?: (event: React.MouseEvent<HTMLElement>) => void;
    queryParams?: ExternalLinkQueryParams;
    location: string;
    children: React.ReactNode;

    // Antimatter: the link is part of what someone wrote (a message's card or link preview), not of Antimatter's
    // UI: it's kept as given, also to mattermost.com, without the tracking parameters added to those.
    userContent?: boolean;
};

const linkOnlyProps = [
    'href', 'target', 'rel', 'onClick', 'queryParams', 'location', 'download', 'hrefLang', 'ping', 'referrerPolicy',
    'type', 'media', 'userContent',
] as const;

const ExternalLink = forwardRef<HTMLAnchorElement, Props>((props, ref) => {
    const {userContent, ...anchorProps} = props;
    const [rewritten] = useExternalLink(props.href, props.location, props.queryParams);
    const href = userContent ? props.href : rewritten;

    const handleClick = (e: React.MouseEvent<HTMLElement>) => {
        if (typeof props.onClick === 'function') {
            props.onClick(e);
        }
    };

    // Antimatter: no links to mattermost.com; their text stays, with the look the link had (class, style, title,
    // test id), so that a link preview card keeps its layout. What only makes sense on a link is left out.
    if (!userContent && isMattermostURL(href)) {
        const textProps = omit(props, linkOnlyProps);
        return (
            <span
                {...textProps}
                role={textProps.role === 'link' ? undefined : textProps.role}
            >
                {props.children}
            </span>
        );
    }

    return (
        <a
            {...anchorProps}
            ref={ref}
            target={props.target || '_blank'}
            rel={props.rel || 'noopener noreferrer'}
            onClick={handleClick}
            href={href}
        >
            {props.children}
        </a>
    );
});

export default ExternalLink;
