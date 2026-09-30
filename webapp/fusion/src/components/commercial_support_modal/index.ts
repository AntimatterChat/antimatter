// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {connect} from 'react-redux';

import {getConfig} from 'mattermost-redux/selectors/entities/general';

import type {GlobalState} from 'types/store';

import CommercialSupportModal from './commercial_support_modal';

function mapStateToProps(state: GlobalState) {
    const config = getConfig(state);
    const showBannerWarning = (config.EnableFile !== 'true' || config.FileLevel !== 'DEBUG');
    const packetContents = [
        {id: 'basic.contents', label: 'Basic contents', selected: true, mandatory: true},
        {id: 'basic.server.logs', label: 'Server logs', selected: true, mandatory: false},
    ];

    for (const [key, value] of Object.entries(state.entities.admin.plugins!)) {
        if (value.active && value.props !== undefined && value.props.support_packet !== undefined) {
            packetContents.push({
                id: key,
                label: value.props.support_packet,
                selected: false,
                mandatory: false,
            });
        }
    }

    return {
        showBannerWarning,
        packetContents,
    };
}

export default connect(mapStateToProps)(CommercialSupportModal);
