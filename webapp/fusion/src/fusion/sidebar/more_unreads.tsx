// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

const UNREAD = '.am-ch.am-unread';

// MoreUnreads shows "More unreads" at the top or bottom of the sidebar while unread channels are scrolled out of view
// there, as in the classic sidebar; clicking scrolls to the nearest one.
export default function MoreUnreads({scrollRef}: {scrollRef: React.RefObject<HTMLDivElement | null>}) {
    const {formatMessage} = useIntl();
    const [above, setAbove] = useState(false);
    const [below, setBelow] = useState(false);

    useEffect(() => {
        const view = scrollRef.current;
        if (!view) {
            return undefined;
        }
        const check = () => {
            const box = view.getBoundingClientRect();
            let up = false;
            let down = false;
            view.querySelectorAll(UNREAD).forEach((el) => {
                const r = el.getBoundingClientRect();
                up = up || r.bottom < box.top + 4;
                down = down || r.top > box.bottom - 4;
            });
            setAbove(up);
            setBelow(down);
        };
        check();
        view.addEventListener('scroll', check, {passive: true});
        const mutations = new MutationObserver(check);
        mutations.observe(view, {subtree: true, childList: true, attributes: true, attributeFilter: ['class']});
        const resize = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(check);
        resize?.observe(view);
        return () => {
            view.removeEventListener('scroll', check);
            mutations.disconnect();
            resize?.disconnect();
        };
    }, [scrollRef]);

    const jump = (up: boolean) => {
        const view = scrollRef.current;
        if (!view) {
            return;
        }
        const box = view.getBoundingClientRect();
        const unread = Array.from(view.querySelectorAll(UNREAD));
        const target = up ? unread.reverse().find((el) => el.getBoundingClientRect().bottom < box.top + 4) : unread.find((el) => el.getBoundingClientRect().top > box.bottom - 4);
        target?.scrollIntoView({block: 'center', behavior: 'smooth'});
    };

    const pill = (up: boolean) => (
        <div className={am('more-unreads', up ? 'mu-top' : 'mu-bottom')}>
            <button
                type='button'
                onClick={() => jump(up)}
            >
                <Icon
                    name='chev'
                    size='xs'
                />
                {formatMessage({id: 'fusion.sidebar.moreUnreads', defaultMessage: 'More unreads'})}
            </button>
        </div>
    );

    return (
        <>
            {above && pill(true)}
            {below && pill(false)}
        </>
    );
}
