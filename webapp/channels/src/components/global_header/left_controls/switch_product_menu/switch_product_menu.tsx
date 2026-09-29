// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import type {ProductIdentifier} from '@mattermost/types/products';

import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import * as Menu from 'components/menu';

import {isChannels} from 'utils/products';

import ProductSwitcherAboutMenuItem from './switch_product_about_menuitem';
import ProductSwitcherChannelsMenuItem from './switch_product_channels_menuitem';
import ProductSwitcherDownloadMenuItem from './switch_product_download_menuitem';
import ProductSwitcherIntegrationsMenuItem from './switch_product_integrations_menuitem';
import ProductSwitcherMarketplaceMenuItem from './switch_product_marketplace_menuitem';
import ProductSwitcherPluginMenuItems from './switch_product_plugin_menuitems';
import ProductSwitcherProductsMenuItems from './switch_product_products_menuitems';
import ProductSwitcherSystemConsoleMenuItem from './switch_product_system_console_menuitem';
import ProductSwitcherUserGroupsMenuItem from './switch_product_user_groups_menuitem';

import ProductBranding from '../product_branding';

export const ELEMENT_ID_FOR_SWITCH_PRODUCT_MENU = 'switchProductMenu';
export const ELEMENT_ID_FOR_SWITCH_PRODUCT_MENU_BUTTON = 'switchProductMenuButton';

type Props = {
    productId: ProductIdentifier;
};

export function SwitchProductMenu(props: Props) {
    const {formatMessage} = useIntl();

    const config = useSelector(getConfig);

    const appDownloadLink = config.AppDownloadLink;

    const haveEnabledIncomingWebhooks = config.EnableIncomingWebhooks === 'true';
    const haveEnabledOutgoingWebhooks = config.EnableOutgoingWebhooks === 'true';
    const haveEnabledSlashCommands = config.EnableCommands === 'true';
    const haveEnabledOAuthServiceProvider = config.EnableOAuthServiceProvider === 'true';

    const siteName = config.SiteName;

    const currentTeam = useSelector(getCurrentTeam);
    const currentTeamId = currentTeam?.id;
    const currentTeamName = currentTeam?.name;

    const isChannelsProductActive = isChannels(props.productId);

    return (
        <Menu.Container
            menuButton={{
                id: ELEMENT_ID_FOR_SWITCH_PRODUCT_MENU_BUTTON,

                // HeaderIconButton is the same classname as the HeaderIconButton component
                class: 'HeaderIconButton globalHeader-leftControls-productMenuButton',

                // The branding sits inside the button so that clicking the product
                // name opens the menu too, not just the icon.
                children: (
                    <>
                        <i className='icon-products'/>
                        <ProductBranding/>
                    </>
                ),
                'aria-label': formatMessage({id: 'globalHeader.productSwitcherMenu.menuButtonLabel', defaultMessage: 'Open product menu'}),
            }}
            menuButtonTooltip={{
                text: formatMessage({id: 'globalHeader.productSwitcherMenu.menuButtonLabel', defaultMessage: 'Open product menu'}),
            }}
            menu={{
                id: ELEMENT_ID_FOR_SWITCH_PRODUCT_MENU,
                width: '240px',
                className: 'globalHeader-leftControls-productSwitcherMenu',
                'aria-label': formatMessage({id: 'globalHeader.productSwitcherMenu.ariaLabel', defaultMessage: 'Product menu'}),
            }}
        >
            <ProductSwitcherChannelsMenuItem
                isChannelsProductActive={isChannelsProductActive}
            />
            <ProductSwitcherProductsMenuItems
                currentProductID={props.productId}
            />
            <ProductSwitcherPluginMenuItems/>
            <Menu.Separator/>
            <ProductSwitcherSystemConsoleMenuItem/>
            <ProductSwitcherIntegrationsMenuItem
                isChannelsProductActive={isChannelsProductActive}
                haveEnabledIncomingWebhooks={haveEnabledIncomingWebhooks}
                haveEnabledOutgoingWebhooks={haveEnabledOutgoingWebhooks}
                haveEnabledSlashCommands={haveEnabledSlashCommands}
                haveEnabledOAuthServiceProvider={haveEnabledOAuthServiceProvider}
                currentTeamName={currentTeamName}
            />
            <ProductSwitcherUserGroupsMenuItem/>
            <ProductSwitcherMarketplaceMenuItem
                isChannelsProductActive={isChannelsProductActive}
                currentTeamId={currentTeamId}
            />
            <ProductSwitcherDownloadMenuItem appDownloadLink={appDownloadLink}/>
            <ProductSwitcherAboutMenuItem siteName={siteName}/>
        </Menu.Container>
    );
}
