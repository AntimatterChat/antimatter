// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useState} from 'react';
import {FormattedMessage} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {getConfig} from 'mattermost-redux/selectors/entities/general';

import {switchWebUI} from 'actions/web_ui';

import SettingItem from 'components/setting_item';
import SettingItemMax from 'components/setting_item_max';

import {CURRENT_WEB_UI, WebUIs} from 'utils/web_ui';
import type {WebUI} from 'utils/web_ui';

type Props = {
    active: boolean;
    areAllSectionsInactive: boolean;
    updateSection: (section: string) => void;
};

const SECTION = 'webUI';

const webUINames: Record<WebUI, React.ReactNode> = {
    [WebUIs.CLASSIC]: (
        <FormattedMessage
            id='user.settings.display.webUI.classic'
            defaultMessage='Classic (legacy)'
        />
    ),
    [WebUIs.FUSION]: (
        <FormattedMessage
            id='user.settings.display.webUI.fusion'
            defaultMessage='Fusion'
        />
    ),
};

const title = (
    <FormattedMessage
        id='user.settings.display.webUI.title'
        defaultMessage='Web interface'
    />
);

// WebUISection lets users pick the web UI they're served, when the server allows it.
export default function WebUISection({active, areAllSectionsInactive, updateSection}: Props) {
    const dispatch = useDispatch();
    const allowed = useSelector(getConfig).AllowUserWebUISelection === 'true';
    const [value, setValue] = useState<WebUI>(CURRENT_WEB_UI);

    const handleChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
        setValue(e.currentTarget.value as WebUI);
    }, []);

    const submit = useCallback(() => {
        if (value === CURRENT_WEB_UI) {
            updateSection('');
            return;
        }
        dispatch(switchWebUI(value));
    }, [value, updateSection, dispatch]);

    const handleUpdateSection = useCallback((section: string) => {
        setValue(CURRENT_WEB_UI);
        updateSection(section);
    }, [updateSection]);

    if (!allowed) {
        return null;
    }

    const input = (
        <fieldset key='webUISetting'>
            <legend className='form-legend hidden-label'>
                {title}
            </legend>
            {[WebUIs.FUSION, WebUIs.CLASSIC].map((webUI) => (
                <div
                    className='radio'
                    key={webUI}
                >
                    <label>
                        <input
                            id={`webUI-${webUI}`}
                            type='radio'
                            name='webUI'
                            value={webUI}
                            checked={value === webUI}
                            onChange={handleChange}
                        />
                        {webUINames[webUI]}
                    </label>
                    <br/>
                </div>
            ))}
            <div className='mt-5'>
                <FormattedMessage
                    id='user.settings.display.webUI.description'
                    defaultMessage="Fusion is Antimatter's interface; the classic one is kept for compatibility. Your choice applies to every browser you sign in from, and the page reloads to apply it."
                />
            </div>
        </fieldset>
    );

    return (
        <div>
            <SettingItem
                active={active}
                areAllSectionsInactive={areAllSectionsInactive}
                title={title}
                describe={webUINames[CURRENT_WEB_UI]}
                section={SECTION}
                updateSection={handleUpdateSection}
                max={(
                    <SettingItemMax
                        title={title}
                        inputs={[input]}
                        submit={submit}
                        updateSection={handleUpdateSection}
                        disableEnterSubmit={true}
                    />
                )}
            />
            <div className='divider-dark'/>
        </div>
    );
}

WebUISection.section = SECTION;
