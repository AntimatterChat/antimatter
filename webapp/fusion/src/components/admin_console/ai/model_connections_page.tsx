// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import React from 'react';
import {defineMessages, FormattedMessage} from 'react-intl';

import type {SearchableStrings} from 'components/admin_console/types';
import AdminHeader from 'components/widgets/admin_console/admin_header';

import ModelConnections from 'fusion/ai/model_connections';
import {IconSprite} from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

// The Fusion styles, should the console be opened before the app.
import 'fusion/styles/_module.scss';

import './ai.scss';

const messages = defineMessages({
    title: {id: 'admin.ai.connections.title', defaultMessage: 'Model connections'},
    search: {id: 'admin.ai.connections.search', defaultMessage: 'AI providers, API keys, models, limits and usage'},
});

export const searchableStrings: SearchableStrings = Object.values(messages);

// ModelConnectionsPage is the System Console's AI › Model connections: the server-wide connections and every
// community's, drawn with the Fusion controls the community settings use for theirs.
export default function ModelConnectionsPage() {
    return (
        <div className='wrapper--fixed ai-connections-page'>
            <AdminHeader>
                <FormattedMessage {...messages.title}/>
            </AdminHeader>
            <div className='admin-console__wrapper'>
                <div className='admin-console__content'>
                    <div className={am('layer', 'ai-console')}>
                        <IconSprite/>
                        <ModelConnections
                            scope='server'
                            listScope=''
                        />
                    </div>
                </div>
            </div>
        </div>
    );
}
