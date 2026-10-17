// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';
import {useLocation} from 'react-router-dom';

import type {Team} from '@mattermost/types/teams';

import {getCustomEmojisEnabled} from 'mattermost-redux/selectors/entities/emojis';
import {getConfig} from 'mattermost-redux/selectors/entities/general';

import {AddCustomEmojiView, ManageCustomEmojiView} from 'fusion/popovers/custom_emoji_views';
import {PickerEscapeProvider, PickerNotice, useEscapeStack} from 'fusion/popovers/picker_views';
import {canCreateCustomEmoji} from 'fusion/selectors/custom_emoji';
import {am} from 'fusion/utils/class_names';
import {getHistory} from 'utils/browser_history';

type Props = {
    team: Team;

    // /:team/emoji/add opens on adding one.
    view?: 'manage' | 'add';
};

// CustomEmojiPage is the /:team/emoji page (Integrations › Custom emoji, and its old links): the emoji picker's own
// add and manage views, on a page. Adding one comes back to the list, searched for it.
export default function CustomEmojiPage({team, view = 'manage'}: Props) {
    const {formatMessage} = useIntl();
    const enabled = useSelector(getCustomEmojisEnabled);
    const canCreate = useSelector(canCreateCustomEmoji);
    const siteName = useSelector(getConfig).SiteName || '';
    const location = useLocation<{added?: string} | undefined>();
    const added = location.state?.added;
    const escapes = useEscapeStack();
    const base = `/${team.name}/emoji`;
    const title = formatMessage({id: 'fusion.customEmoji.title', defaultMessage: 'Custom emoji'});

    useEffect(() => {
        document.title = `${title} - ${team.display_name} ${siteName}`.trim();
    }, [title, team.display_name, siteName]);

    // Escape goes back from adding one, or cancels a row's confirmation, as in the picker.
    useEffect(() => {
        const onKeyDown = (e: KeyboardEvent) => {
            if (e.key === 'Escape' && escapes.escape()) {
                e.preventDefault();
            }
        };
        document.addEventListener('keydown', onKeyDown);
        return () => document.removeEventListener('keydown', onKeyDown);
    }, [escapes]);

    let body: React.ReactNode;
    if (!enabled) {
        body = <PickerNotice kind='info'>{formatMessage({id: 'fusion.customEmoji.off', defaultMessage: 'Custom emoji are turned off on this server.'})}</PickerNotice>;
    } else if (view === 'add' && canCreate) {
        body = (
            <AddCustomEmojiView
                variant='page'
                onBack={() => getHistory().push(base)}
                onAdded={(emoji) => getHistory().push(base, {added: emoji.name})}
            />
        );
    } else {
        body = (
            <>
                {added && <PickerNotice kind='ok'>{formatMessage({id: 'fusion.customEmoji.added', defaultMessage: ':{name}: is ready to use.'}, {name: added})}</PickerNotice>}
                <ManageCustomEmojiView
                    key={added || ''}
                    variant='page'
                    noBack={true}
                    initialQuery={added}
                    onBack={() => getHistory().push(`/${team.name}`)}
                    onAdd={() => getHistory().push(`${base}/add`)}
                />
            </>
        );
    }

    return (
        <PickerEscapeProvider value={escapes.value}>
            <div className={am('ce-page')}>{body}</div>
        </PickerEscapeProvider>
    );
}
