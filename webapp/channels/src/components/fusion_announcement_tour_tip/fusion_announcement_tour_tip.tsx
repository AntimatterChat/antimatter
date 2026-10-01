// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useState} from 'react';
import {FormattedMessage} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {TourTip} from '@mattermost/components';

import {savePreferences} from 'mattermost-redux/actions/preferences';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {get as getPreference} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {switchWebUI} from 'actions/web_ui';

import {CURRENT_WEB_UI, FUSION_ANNOUNCEMENT_PREFERENCE, WEB_UI_PREFERENCE, WebUIs} from 'utils/web_ui';

import type {GlobalState} from 'types/store';

import './fusion_announcement_tour_tip.scss';

// Users are told about the Fusion UI when they're on the classic one because they were kept on it,
// i.e. their web UI preference is classic, and they may switch.
function shouldAnnounceFusion(state: GlobalState) {
    return CURRENT_WEB_UI === WebUIs.CLASSIC &&
        getConfig(state).AllowUserWebUISelection === 'true' &&
        getPreference(state, WEB_UI_PREFERENCE.CATEGORY, WEB_UI_PREFERENCE.NAME, '') === WebUIs.CLASSIC &&
        getPreference(state, FUSION_ANNOUNCEMENT_PREFERENCE.CATEGORY, FUSION_ANNOUNCEMENT_PREFERENCE.NAME, '') !== FUSION_ANNOUNCEMENT_PREFERENCE.DISMISSED;
}

// FusionAnnouncementTourTip tells users of the classic web UI that the Fusion UI is available, until
// they try it or choose not to. Closing it leaves its pulsating dot, which opens it again.
const FusionAnnouncementTourTip = () => {
    const dispatch = useDispatch();
    const currentUserId = useSelector(getCurrentUserId);
    const announce = useSelector(shouldAnnounceFusion);
    const [open, setOpen] = useState(true);

    const handleOpen = useCallback((e: React.MouseEvent) => {
        e.preventDefault();
        e.stopPropagation();
        setOpen(true);
    }, []);

    const handleClose = useCallback((e: React.MouseEvent) => {
        e.preventDefault();
        e.stopPropagation();
        setOpen(false);
    }, []);

    const handleNotNow = useCallback((e: React.MouseEvent) => {
        e.preventDefault();
        e.stopPropagation();
        dispatch(savePreferences(currentUserId, [{
            user_id: currentUserId,
            category: FUSION_ANNOUNCEMENT_PREFERENCE.CATEGORY,
            name: FUSION_ANNOUNCEMENT_PREFERENCE.NAME,
            value: FUSION_ANNOUNCEMENT_PREFERENCE.DISMISSED,
        }]));
    }, [currentUserId, dispatch]);

    const handleTryFusion = useCallback((e: React.MouseEvent) => {
        e.preventDefault();
        e.stopPropagation();
        dispatch(switchWebUI(WebUIs.FUSION));
    }, [dispatch]);

    if (!announce) {
        return null;
    }

    const title = (
        <span className='FusionAnnouncementTourTip__title'>
            <FormattedMessage
                id='fusion_announcement.title'
                defaultMessage='A new interface is available'
            />
            <span className='FusionAnnouncementTourTip__badge'>
                <FormattedMessage
                    id='fusion_announcement.badge'
                    defaultMessage='New'
                />
            </span>
        </span>
    );

    const screen = (
        <>
            <p>
                <FormattedMessage
                    id='fusion_announcement.description'
                    defaultMessage='Fusion is the new interface of Antimatter: a refreshed layout with voice channels, apps and more. Your conversations and settings stay the same.'
                />
            </p>
            <p>
                <FormattedMessage
                    id='fusion_announcement.switchBack'
                    defaultMessage='You can switch back at any time in <b>Settings > Display > Web interface</b>.'
                    values={{b: (chunks: React.ReactNode) => <b>{chunks}</b>}}
                />
            </p>
        </>
    );

    return (
        <TourTip
            show={open}
            screen={screen}
            title={title}
            overlayPunchOut={null}
            placement='bottom-end'
            pulsatingDotPlacement='right-end'
            pulsatingDotTranslate={{x: -56, y: 4}}
            step={1}
            singleTip={true}
            showOptOut={false}
            interactivePunchOut={false}
            handleOpen={handleOpen}
            handleDismiss={handleClose}
            handlePrevious={handleNotNow}
            handleNext={handleTryFusion}
            prevBtn={(
                <FormattedMessage
                    id='fusion_announcement.notNow'
                    defaultMessage='Not now'
                />
            )}
            nextBtn={(
                <FormattedMessage
                    id='fusion_announcement.tryFusion'
                    defaultMessage='Try Fusion'
                />
            )}
            width={352}
            tippyBlueStyle={true}
            hideBackdrop={true}
            className='FusionAnnouncementTourTip'
        />
    );
};

export default FusionAnnouncementTourTip;
