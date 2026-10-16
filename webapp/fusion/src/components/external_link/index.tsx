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
};

const linkOnlyProps = [
    'href', 'target', 'rel', 'onClick', 'queryParams', 'location', 'download', 'hrefLang', 'ping', 'referrerPolicy',
    'type', 'media',
] as const;

const ExternalLink = forwardRef<HTMLAnchorElement, Props>((props, ref) => {
    const [href] = useExternalLink(props.href, props.location, props.queryParams);

    const handleClick = (e: React.MouseEvent<HTMLElement>) => {
        if (typeof props.onClick === 'function') {
            props.onClick(e);
        }
    };

    // Antimatter: no links to mattermost.com; their text stays, with the look the link had (class, style, title,
    // test id), so that a link preview card keeps its layout. What only makes sense on a link is left out.
    if (isMattermostURL(href)) {
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
            {...props}
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
