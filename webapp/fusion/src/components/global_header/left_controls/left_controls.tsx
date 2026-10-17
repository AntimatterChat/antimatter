// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {type JSX} from 'react';
import {useSelector} from 'react-redux';

import {isDesktopApp} from '@mattermost/shared/utils/user_agent';
import type {ProductIdentifier} from '@mattermost/types/products';

import type {GlobalState} from 'types/store';

import HistoryButtons from './history_buttons';
import {ProductHeading} from './product_branding';
import ProductSwitcherMenu from './switch_product_menu';

import './left_controls.scss';

// Antimatter: the product menu only shows when there is something to switch to: an app with a page of its own
// (registerProduct) or an entry an app added to the menu (registerProductSwitcherMenuItem). No Antimatter app needs
// it any more (Kanban and Playbooks are apps of the side panel); its other entries are in Fusion's menus (System
// Console, Integrations and App Marketplace in the status menu, user groups in the community menu, About and the
// apps' downloads in the About menu of the app rail).
export const hasProductSwitcherEntries = (state: GlobalState): boolean => (
    (state.plugins.components.Product?.length ?? 0) > 0 ||
    (state.plugins.components.ProductSwitcherMenuItem?.length ?? 0) > 0
);

type Props = {
    productId: ProductIdentifier;
};

const LeftControls = (props: Props): JSX.Element => {
    const showSwitcher = useSelector(hasProductSwitcherEntries);

    return (
        <div className='globalHeader-leftControls'>
            <ProductHeading/>
            {showSwitcher && <ProductSwitcherMenu productId={props.productId}/>}
            {isDesktopApp() && <HistoryButtons/>}
        </div>
    );
};

export default LeftControls;
