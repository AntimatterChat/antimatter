// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {connect} from 'react-redux';

import {getSiteURL} from 'utils/url';

import ConfigurationBar from './configuration_bar';

function mapStateToProps() {
    return {
        siteURL: getSiteURL(),
    };
}

export default connect(mapStateToProps)(ConfigurationBar);
