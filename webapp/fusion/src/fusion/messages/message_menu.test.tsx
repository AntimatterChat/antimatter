// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Post} from '@mattermost/types/posts';

import {Posts} from 'mattermost-redux/constants';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import Files from './content/files';
import {clearImagesShown} from './content/image_fold';
import MessageMenu from './message_menu';

jest.mock('components/file_preview_modal', () => () => null);
jest.mock('components/emoji/render_emoji', () => () => null);
const mockNoEmoji: string[] = [];
jest.mock('selectors/emojis', () => ({...jest.requireActual('selectors/emojis'), getRecentEmojisNames: () => mockNoEmoji}));
jest.mock('fusion/shell/toast_context', () => ({useToast: () => jest.fn()}));
jest.mock('fusion/shell/dialogs_context', () => ({useDialogs: () => ({deleteMessage: jest.fn()})}));
jest.mock('fusion/sidebar/group_dm', () => ({useNotesToSelf: () => jest.fn()}));
jest.mock('fusion/rhs/thread_to', () => ({canMoveThread: () => false}));
jest.mock('./plugin_menu_actions', () => () => null);

const imageFile = {id: 'f1', name: 'cat.png', extension: 'png', mime_type: 'image/png', has_preview_image: true, width: 100, height: 80, size: 1000, post_id: 'p1', user_id: 'u1', create_at: 0, update_at: 0, delete_at: 0};

// Hiding a message's images is in its ⋯ menu (the hover button over the images is gone).
describe('fusion/messages/MessageMenu: hiding images', () => {
    beforeEach(() => {
        clearImagesShown();
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const post = (id: string) => ({id, channel_id: 'c1', user_id: 'u1', root_id: '', type: '', message: 'look', create_at: 0, props: {}, metadata: {}} as unknown as Post);
    const state = (collapsed = false) => ({
        entities: {
            users: {currentUserId: 'me', profiles: {me: {id: 'me', roles: 'system_user'}}},
            teams: {currentTeamId: 't1', teams: {t1: {id: 't1', name: 'one', display_name: 'One', delete_at: 0}}},
            channels: {channels: {c1: {id: 'c1', team_id: 't1', type: 'O', name: 'town', display_name: 'Town', delete_at: 0}}},
            posts: {posts: {p1: post('p1'), p2: post('p2')}},
            files: {files: {f1: imageFile}, fileIdsByPostId: {p1: ['f1']}},
            preferences: {myPreferences: collapsed ? {'display_settings--collapse_previews': {category: 'display_settings', name: 'collapse_previews', value: 'true', user_id: 'me'}} : {}},
        },
    } as never);
    const renderMenu = (id: string, collapsed = false) => renderWithContext(
        <>
            <Files post={post(id)}/>
            <MessageMenu
                post={post(id)}
                anchor={null}
                inThread={false}
                onClose={jest.fn()}
                onMoreReactions={jest.fn()}
            />
        </>,
        state(collapsed),
    );

    test('a message with an image offers Hide image, which hides it', async () => {
        renderMenu('p1');
        expect(screen.getByRole('button', {name: 'Open image cat.png'})).toBeInTheDocument();
        const hide = screen.getByRole('menuitem', {name: 'Hide image'});
        expect(hide.querySelector('use')).toHaveAttribute('href', '#am-i-eye-off');

        await userEvent.click(hide);
        expect(screen.queryByRole('button', {name: 'Open image cat.png'})).not.toBeInTheDocument();
        expect(screen.getByRole('menuitem', {name: 'Show image'})).toBeInTheDocument();
    });

    test('with previews collapsed, it offers Show image', async () => {
        renderMenu('p1', true);
        expect(screen.queryByRole('button', {name: 'Open image cat.png'})).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('menuitem', {name: 'Show image'}));
        expect(screen.getByRole('button', {name: 'Open image cat.png'})).toBeInTheDocument();
    });

    test('a message without images has no such entry', () => {
        renderMenu('p2');
        expect(screen.getByRole('menu', {name: 'Message actions'})).toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: /image/})).not.toBeInTheDocument();
    });
});

// As in the classic menu, Pin is only offered where the server allows it.
describe('fusion/messages/MessageMenu: pinning', () => {
    beforeEach(() => {
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const post = (type = '') => ({id: 'p1', channel_id: 'c1', user_id: 'u1', root_id: '', type, message: 'look', create_at: 0, props: {}, metadata: {}} as unknown as Post);
    const renderMenu = (p: Post, archived = false) => renderWithContext(
        <MessageMenu
            post={p}
            anchor={null}
            inThread={false}
            onClose={jest.fn()}
            onMoreReactions={jest.fn()}
        />,
        {
            entities: {
                users: {currentUserId: 'me', profiles: {me: {id: 'me', roles: 'system_user'}}},
                teams: {currentTeamId: 't1', teams: {t1: {id: 't1', name: 'one', display_name: 'One', delete_at: 0}}},
                channels: {channels: {c1: {id: 'c1', team_id: 't1', type: 'O', name: 'town', display_name: 'Town', delete_at: archived ? 1 : 0}}},
                posts: {posts: {p1: p}},
            },
        } as never,
    );

    test('a message can be pinned', () => {
        renderMenu(post());
        expect(screen.getByRole('menuitem', {name: 'Pin to channel'})).toBeInTheDocument();
    });

    test('a burn-on-read message is not offered to pin', () => {
        renderMenu(post(Posts.POST_TYPES.BURN_ON_READ));
        expect(screen.getByRole('menu', {name: 'Message actions'})).toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: 'Pin to channel'})).not.toBeInTheDocument();
    });

    test('nothing is offered to pin in an archived channel', () => {
        renderMenu(post(), true);
        expect(screen.queryByRole('menuitem', {name: 'Pin to channel'})).not.toBeInTheDocument();
    });
});

// Antimatter forums (design §3.3, §4.4): Accept answer on a Q&A reply (the asker and moderators only), Set status
// on an Ideas topic's root (moderators only). The UI only mirrors the server's rules.
describe('fusion/messages/MessageMenu: forums', () => {
    beforeEach(() => {
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const reply = (override: Partial<Post> = {}) => ({id: 'r1', channel_id: 'c1', user_id: 'maya', root_id: 'r0', type: '', message: 'Try the reset switch.', create_at: 100, props: {}, metadata: {}, ...override} as unknown as Post);
    const root = (override: Partial<Post> = {}) => ({id: 'r0', channel_id: 'c1', user_id: 'ada', root_id: '', type: '', message: 'How do I reset the beam?', create_at: 0, props: {am_thread_title: 'How do I reset the beam?'}, metadata: {}, ...override} as unknown as Post);

    const stateFor = (mode: 'qa' | 'ideas', systemRoles: string, rootPost: Post, timedOut = false) => ({
        entities: {
            general: {serverVersion: ''},
            users: {currentUserId: 'me', profiles: {me: {id: 'me', roles: systemRoles}}},
            teams: {
                currentTeamId: 't1',
                teams: {t1: {id: 't1', name: 'one', display_name: 'One', delete_at: 0}},
                myMembers: {t1: {team_id: 't1', user_id: 'me', roles: 'team_user'}},
                timeouts: timedOut ? {t1: [{team_id: 't1', user_id: 'me', until: Date.now() + 3600000, reason: '', created_by: 'mod', create_at: 0}]} : {},
            },
            channels: {
                channels: {c1: {id: 'c1', team_id: 't1', type: 'O', name: 'town', display_name: 'Town', delete_at: 0}},
                channelTypes: {c1: {type: 'forum', mode, root_posts: 'members', view: 'topics', preset: ''}},
                myMembers: {c1: {channel_id: 'c1', user_id: 'me', roles: 'channel_user'}},
                roles: {c1: new Set(['channel_user'])},
            },
            posts: {posts: {r0: rootPost, r1: reply()}},
            files: {files: {}, fileIdsByPostId: {}},
            preferences: {myPreferences: {}},
            roles: {
                roles: {
                    system_user: {name: 'system_user', permissions: []},
                    system_admin: {name: 'system_admin', permissions: ['manage_system']},
                    team_user: {name: 'team_user', permissions: []},
                    channel_user: {name: 'channel_user', permissions: ['create_post']},
                },
            },
        },
    } as never);

    const renderMenuFor = (post: Post, state: ReturnType<typeof stateFor>) => renderWithContext(
        <MessageMenu
            post={post}
            anchor={null}
            inThread={true}
            onClose={jest.fn()}
            onMoreReactions={jest.fn()}
        />,
        state,
    );

    test('Q&A: a member who isn\'t the asker gets no Accept answer', () => {
        renderMenuFor(reply(), stateFor('qa', 'system_user', root()));
        expect(screen.queryByRole('menuitem', {name: /accept answer/i})).not.toBeInTheDocument();
    });

    test('Q&A: the asker gets Accept answer on a reply', () => {
        renderMenuFor(reply(), stateFor('qa', 'system_user', root({user_id: 'me'})));
        expect(screen.getByRole('menuitem', {name: 'Accept answer'})).toBeInTheDocument();
    });

    // Mirrors the server's sessionMayAcceptAnswer: the asker only accepts while they may still post (a timeout
    // takes that away, as it takes create_post away everywhere in the team); a moderator still can.
    test('Q&A: a timed-out asker doesn\'t get Accept answer', () => {
        renderMenuFor(reply(), stateFor('qa', 'system_user', root({user_id: 'me'}), true));
        expect(screen.queryByRole('menuitem', {name: /accept answer/i})).not.toBeInTheDocument();
    });

    test('Q&A: a moderator gets Accept answer even when they didn\'t ask', () => {
        renderMenuFor(reply(), stateFor('qa', 'system_user system_admin', root()));
        expect(screen.getByRole('menuitem', {name: 'Accept answer'})).toBeInTheDocument();
    });

    test('Q&A: an already-accepted reply offers Unaccept instead', () => {
        renderMenuFor(reply(), stateFor('qa', 'system_user', root({user_id: 'me', props: {am_accepted_answer: 'r1'}})));
        expect(screen.getByRole('menuitem', {name: 'Unaccept answer'})).toBeInTheDocument();
    });

    test('Q&A: Accept answer isn\'t offered on the question itself', () => {
        renderMenuFor(root({user_id: 'me'}), stateFor('qa', 'system_user', root({user_id: 'me'})));
        expect(screen.queryByRole('menuitem', {name: /accept answer/i})).not.toBeInTheDocument();
    });

    test('Ideas: a member gets no Set status', () => {
        renderMenuFor(root(), stateFor('ideas', 'system_user', root()));
        expect(screen.queryByRole('menuitem', {name: 'Set status'})).not.toBeInTheDocument();
    });

    test('Ideas: a moderator gets Set status on the topic\'s root, listing every status', () => {
        renderMenuFor(root(), stateFor('ideas', 'system_user system_admin', root()));
        expect(screen.getByRole('menuitem', {name: 'Set status'})).toBeInTheDocument();
    });

    test('Ideas: Set status isn\'t offered on a reply', () => {
        renderMenuFor(reply(), stateFor('ideas', 'system_user system_admin', root()));
        expect(screen.queryByRole('menuitem', {name: 'Set status'})).not.toBeInTheDocument();
    });

    test('a discussion forum\'s moderator gets Set status from neither (no mode for it)', () => {
        const discussion = {
            entities: {
                general: {serverVersion: ''},
                users: {currentUserId: 'me', profiles: {me: {id: 'me', roles: 'system_user system_admin'}}},
                teams: {currentTeamId: 't1', teams: {t1: {id: 't1', name: 'one', display_name: 'One', delete_at: 0}}},
                channels: {
                    channels: {c1: {id: 'c1', team_id: 't1', type: 'O', name: 'town', display_name: 'Town', delete_at: 0}},
                    channelTypes: {c1: {type: 'forum', mode: 'discussion', root_posts: 'members', view: 'topics', preset: ''}},
                },
                posts: {posts: {r0: root(), r1: reply()}},
                files: {files: {}, fileIdsByPostId: {}},
                preferences: {myPreferences: {}},
            },
        } as never;

        renderMenuFor(root(), discussion);
        expect(screen.queryByRole('menuitem', {name: 'Set status'})).not.toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: /accept answer/i})).not.toBeInTheDocument();
    });
});
