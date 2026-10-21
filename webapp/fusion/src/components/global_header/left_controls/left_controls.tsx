// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {type JSX} from 'react';

import {isDesktopApp} from '@mattermost/shared/utils/user_agent';

import HistoryButtons from './history_buttons';
import {ProductHeading} from './product_branding';

import './left_controls.scss';

// Antimatter: Fusion has no product menu (the app switcher). No app has a page of its own any more (Kanban, Playbooks
// and the AI chat are apps of the side panel), and the menu's other entries are in Fusion's menus: System Console,
// Integrations and App Marketplace in the status menu, user groups in the community menu, About and the apps'
// downloads in the About menu of the app rail. The classic web app keeps its product menu.

const LeftControls = (): JSX.Element => {
    return (
        <div className='globalHeader-leftControls'>
            <ProductHeading/>
            {isDesktopApp() && <HistoryButtons/>}
        </div>
    );
};

export default LeftControls;
