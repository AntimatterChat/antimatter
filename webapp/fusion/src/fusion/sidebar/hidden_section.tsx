// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

type Props<T> = {
    items: T[];
    render: (item: T) => React.ReactNode;
};

// HiddenSection lists the conversations hidden until something new is said in them, folded at first, at the end of
// the sidebar: their menu puts them back.
export default function HiddenSection<T>({items, render}: Props<T>) {
    const {formatMessage} = useIntl();
    const [open, setOpen] = useState(false);
    if (!items.length) {
        return null;
    }
    return (
        <>
            <button
                type='button'
                className={am('cat', 'hidden-cat', {collapsed: !open})}
                aria-expanded={open}
                title={formatMessage({id: 'fusion.sidebar.hiddenTitle', defaultMessage: 'Conversations hidden until they get new messages'})}
                onClick={() => setOpen(!open)}
            >
                <Icon
                    name='chev'
                    size='xs'
                />
                <span className={am('grow')}>{formatMessage({id: 'fusion.sidebar.hidden', defaultMessage: 'Hidden'})}</span>
                <span className={am('hidden-count')}>{items.length}</span>
            </button>
            {open && items.map(render)}
        </>
    );
}
