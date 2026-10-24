// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect} from 'react';
import {useDispatch, useStore} from 'react-redux';

import {getCurrentChannelId, getUnreadChannelIds} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeam, getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';

import {switchTeam} from 'actions/team_actions';
import {closeRightHandSide, showChannelInfo, showMentions, toggleRhsExpanded} from 'actions/views/rhs';
import {getIsRhsOpen} from 'selectors/rhs';
import {getDisplayedChannels} from 'selectors/views/channel_sidebar';

import {getLastTeamChannelName, getSortedMyTeams} from 'fusion/selectors';
import {markTeamAsRead} from 'fusion/sidebar/team_actions';
import {openNewDirectMessage} from 'fusion/utils/modals';
import {channelPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

import {useLayout} from './layout_context';

// next picks the item after (or before) the current one, going round.
export function next<T>(items: T[], current: T, step: number): T | undefined {
    if (!items.length) {
        return undefined;
    }
    let at = items.indexOf(current);
    if (at < 0) {
        at = step > 0 ? -1 : 0;
    }
    const n = items.length;
    return items[(((at + step) % n) + n) % n];
}

// useNavShortcuts wires the classic web app's navigation shortcuts, which its sidebar and team sidebar handled:
// Alt+Up/Down (channels), Alt+Shift+Up/Down (unread channels), Ctrl/⌘+Alt+Up/Down and Ctrl/⌘+Alt+1–9 (teams),
// Ctrl/⌘+Shift+M (mentions), Ctrl/⌘+Shift+L (the message box), Ctrl/⌘+. (the right panel) and Ctrl/⌘+Shift+. (its
// width), Ctrl/⌘+Alt+I (channel info), Ctrl/⌘+Alt+E (emoji), Ctrl/⌘+Shift+K (new direct message), Shift+Esc (mark
// the team read).
export function useNavShortcuts() {
    const dispatch = useDispatch();
    const store = useStore<GlobalState>();
    const layout = useLayout();

    useEffect(() => {
        const goToChannel = (channelId: string) => {
            const state = store.getState();
            const team = getCurrentTeam(state);
            const channel = state.entities.channels.channels[channelId];
            if (team && channel) {
                layout.setHome(false);
                getHistory().push(channelPath(team.name, channel));
            }
        };
        const goToTeam = (index: number | 'prev' | 'next') => {
            const state = store.getState();
            const teams = getSortedMyTeams(state);
            const nextId = typeof index === 'number' ? teams[index]?.id : next(teams.map((t) => t.id), getCurrentTeamId(state), index === 'next' ? 1 : -1);
            const team = teams.find((t) => t.id === nextId);
            if (!team) {
                return;
            }
            layout.setHome(false);
            const channelName = getLastTeamChannelName(state, team.id);
            if (channelName) {
                getHistory().push(channelPath(team.name, {name: channelName}));
            } else {
                dispatch(switchTeam(`/${team.name}`));
            }
        };

        const onKeyDown = (e: KeyboardEvent) => {
            if (e.defaultPrevented) {
                return;
            }
            const mod = e.ctrlKey || e.metaKey;
            const up = e.key === 'ArrowUp';
            const down = e.key === 'ArrowDown';
            const state = store.getState();
            let handled = true;

            if (e.altKey && !mod && (up || down)) {
                const channels = getDisplayedChannels(state).map((c) => c.id);
                const ids = e.shiftKey ? channels.filter((id) => getUnreadChannelIds(state).includes(id) || id === getCurrentChannelId(state)) : channels;
                const target = next(ids, getCurrentChannelId(state), down ? 1 : -1);
                if (target && target !== getCurrentChannelId(state)) {
                    goToChannel(target);
                }
            } else if (mod && e.altKey && !e.shiftKey && (up || down)) {
                goToTeam(down ? 'next' : 'prev');
            } else if (mod && e.altKey && !e.shiftKey && (/^Digit[1-9]$/).test(e.code)) {
                goToTeam(Number(e.code.slice(5)) - 1);
            } else if (mod && e.shiftKey && !e.altKey && e.code === 'KeyM') {
                dispatch(showMentions());
            } else if (mod && e.shiftKey && !e.altKey && e.code === 'KeyL') {
                // The rich text editor renders a ProseMirror contenteditable div instead of the plain textarea
                // (composer.tsx), so look for whichever one is actually there.
                const rich = document.querySelector<HTMLElement>('.am-main .am-composer .am-rich-input .ProseMirror');
                (rich ?? document.querySelector<HTMLTextAreaElement>('.am-main .am-composer textarea'))?.focus();
            } else if (mod && !e.altKey && e.code === 'Period') {
                if (e.shiftKey) {
                    if (getIsRhsOpen(state)) {
                        dispatch(toggleRhsExpanded());
                    }
                } else if (getIsRhsOpen(state)) {
                    dispatch(closeRightHandSide());
                } else {
                    layout.toggleMembers();
                }
            } else if (mod && e.altKey && !e.shiftKey && e.code === 'KeyI') {
                dispatch(showChannelInfo(getCurrentChannelId(state)));
            } else if (mod && e.altKey && !e.shiftKey && e.code === 'KeyE') {
                document.querySelector<HTMLButtonElement>('.am-main .am-composer button[aria-haspopup="dialog"]')?.click();
            } else if (mod && e.shiftKey && !e.altKey && e.code === 'KeyK') {
                dispatch(openNewDirectMessage());
            } else if (e.shiftKey && !mod && !e.altKey && e.key === 'Escape') {
                dispatch(markTeamAsRead(getCurrentTeamId(state)));
            } else {
                handled = false;
            }
            if (handled) {
                e.preventDefault();
            }
        };
        document.addEventListener('keydown', onKeyDown);
        return () => document.removeEventListener('keydown', onKeyDown);
    }, [dispatch, store, layout]);
}
