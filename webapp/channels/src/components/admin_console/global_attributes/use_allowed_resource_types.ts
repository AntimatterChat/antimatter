// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useMemo} from 'react';
import {useSelector} from 'react-redux';

import {isChannelAttributesEnabled, isPostAttributesEnabled} from 'mattermost-redux/selectors/entities/general';

import {ALL_RESOURCE_TYPES} from './attribute_details/attribute_applies_to_constants';
import type {ResourceObjectType} from './attribute_details/attribute_applies_to_constants';

// The Applies-to resources this server offers, in the fixed Users -> Channels ->
// Posts order. Shared by the listing table and the details page so both pages
// offer, fetch, and display exactly the same set.
//
// Channels is gated on its ChannelAttributes flag. Posts is gated on
// PostAttributes: the resource is not finished yet, so it must not be offered
// until the flag is on.
export default function useAllowedResourceTypes(): ResourceObjectType[] {
    const channelAttributesEnabled = useSelector(isChannelAttributesEnabled);
    const postAttributesEnabled = useSelector(isPostAttributesEnabled);

    return useMemo(() => ALL_RESOURCE_TYPES.filter((type) => {
        switch (type) {
        case 'channel':
            return channelAttributesEnabled;
        case 'post':
            return postAttributesEnabled;
        default:
            return true;
        }
    }), [channelAttributesEnabled, postAttributesEnabled]);
}
