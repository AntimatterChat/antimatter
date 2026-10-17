// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

import type {IconGlyphTypes} from '@mattermost/compass-icons/IconGlyphs';
import type {Channel} from '@mattermost/types/channels';

import {useChannelIconOverrideName} from 'components/channel_type_icon/useChannelIconOverrideName';

import Icon from 'fusion/components/icon';
import type {IconName} from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

// The mockup's glyphs for the icons plugins give channels (registerChannelIconOverride).
const overrideGlyphs: Partial<Record<IconGlyphTypes, IconName>> = {
    'volume-high': 'speaker',
    'shield-outline': 'shield',
    'forum-outline': 'forum',
};

type Props = {
    channel: Channel;
    size?: 'sm' | 'xs';

    // A message waits in the channel's draft: a pencil instead of the channel's glyph.
    draft?: boolean;
};

// ChannelIcon is the mockup's channel glyph: # for channels, with a lock for private ones (a globe for shared ones), or the
// icon a plugin gives the channel (e.g. a speaker for voice channels); a pencil while a draft waits in it. The marks name
// themselves in the Fusion tooltip (data-am-tip), not the browser's.
export default function ChannelIcon({channel, size, draft = false}: Props) {
    const {formatMessage} = useIntl();
    const override = useChannelIconOverrideName(channel);
    const glyph = override ? overrideGlyphs[override] : undefined;

    let icon;
    if (draft) {
        const label = formatMessage({id: 'fusion.channel.draft', defaultMessage: 'Draft waiting'});
        icon = (
            <span
                className={am('draft-ic')}
                role='img'
                aria-label={label}
                data-am-tip={label}
            >
                <Icon
                    name='pen'
                    size={size}
                />
            </span>
        );
    } else if (override && !glyph) {
        icon = <i className={`icon icon-${override}`}/>;
    } else {
        icon = (
            <Icon
                name={glyph || 'hash'}
                size={size}
            />
        );
    }

    return (
        <span className={am('ch-ic')}>
            {icon}
            {channel.type === 'P' && (
                <span
                    className={am('lock')}
                    role='img'
                    aria-label={formatMessage({id: 'fusion.channel.private', defaultMessage: 'Private'})}
                    data-am-tip={formatMessage({id: 'fusion.channel.private', defaultMessage: 'Private'})}
                >
                    <Icon name='lock'/>
                </span>
            )}
            {channel.type !== 'P' && channel.shared && (
                <span
                    className={am('lock', 'shared')}
                    role='img'
                    aria-label={formatMessage({id: 'fusion.channel.shared', defaultMessage: 'Shared with other servers'})}
                    data-am-tip={formatMessage({id: 'fusion.channel.shared', defaultMessage: 'Shared with other servers'})}
                >
                    <Icon name='globe'/>
                </span>
            )}
        </span>
    );
}
