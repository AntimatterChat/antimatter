// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {uploadFile} from 'actions/file_actions';
import {onSubmit} from 'actions/views/create_comment';

import {LAYER_ID} from 'fusion/components/layer';
import {act, fireEvent, renderWithContext, screen, userEvent, waitFor, within} from 'tests/react_testing_utils';
import {execCommandInsertText} from 'utils/exec_commands';
import {TestHelper} from 'utils/test_helper';

import type {ComposerModeProps} from 'types/store/plugins';

import Composer from './composer';
import {COMPOSER_MODE_EVENT} from './composer_modes';

const mockTomorrow = Date.UTC(2030, 0, 1, 9);

jest.mock('actions/views/create_comment', () => ({
    ...jest.requireActual('actions/views/create_comment'),
    onSubmit: jest.fn(() => () => Promise.resolve({data: true})),
}));
jest.mock('actions/global_actions', () => ({emitLocalUserTypingEvent: jest.fn()}));
jest.mock('actions/file_actions', () => ({
    ...jest.requireActual('actions/file_actions'),
    uploadFile: jest.fn(() => () => ({abort: jest.fn()})),
}));
jest.mock('./confirm_mentions', () => ({confirmChannelWideMentions: () => () => Promise.resolve({data: true})}));
jest.mock('fusion/shell/dialogs_context', () => ({useDialogs: () => ({})}));
jest.mock('fusion/shell/toast_context', () => ({useToast: () => jest.fn()}));
let mockBurnAllowed = false;
jest.mock('components/common/hooks/useCreateBurnOnReadAccess', () => ({useCreateBurnOnReadAccess: () => mockBurnAllowed}));
jest.mock('./ai_rewrite', () => ({__esModule: true, default: () => null, useAIRewriteAvailable: () => false}));
jest.mock('utils/exec_commands', () => ({execCommandInsertText: jest.fn()}));
jest.mock('./autocomplete', () => ({__esModule: true, default: jest.requireActual('react').forwardRef(() => null)}));
jest.mock('./away_note', () => ({__esModule: true, default: () => null}));
jest.mock('./sleep_note', () => ({__esModule: true, default: () => null}));
jest.mock('./typing', () => ({__esModule: true, default: () => null}));

// The schedule picker, reduced to one of its choices.
jest.mock('./schedule', () => ({
    ScheduledNote: () => null,
    SchedulePopover: ({onSchedule}: {onSchedule: (at: number) => void}) => (
        <button
            type='button'
            onClick={() => onSchedule(mockTomorrow)}
        >
            {'Tomorrow at 9:00'}
        </button>
    ),
}));

describe('fusion/composer/Composer', () => {
    const channel = TestHelper.getChannelMock({id: 'c1', name: 'general', display_name: 'General', type: 'O', team_id: 'team'});
    const render = (config: Record<string, string> = {}, components: Record<string, unknown> = {}) => renderWithContext(
        <Composer
            channelId='c1'
            placeholder='Write to General'
        />,
        {
            entities: {
                general: {config: {ScheduledPosts: 'true', ...config}},
                users: {currentUserId: 'me'},
                channels: {currentChannelId: 'c1', channels: {c1: channel}},
            },
            plugins: {components},
        } as never,
    );

    beforeEach(() => {
        jest.clearAllMocks();
        mockBurnAllowed = false;
        const layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.appendChild(layer);

        // The formatting bar shown, as chosen in this browser.
        localStorage.setItem('am-formatting', 'true');
    });
    afterEach(() => {
        localStorage.removeItem('am-formatting');
        document.getElementById(LAYER_ID)?.remove();
    });

    test('has no schedule button in the formatting bar: the send button\'s chevron schedules', () => {
        render();

        expect(screen.getByRole('toolbar', {name: 'Formatting'})).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Schedule message'})).not.toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Send later'})).toBeInTheDocument();
    });

    test('schedules the message from "Send later"', async () => {
        render();

        await userEvent.type(screen.getByRole('textbox', {name: 'Write to General'}), 'See you tomorrow');
        await userEvent.click(screen.getByRole('button', {name: 'Send later'}));
        await userEvent.click(screen.getByRole('button', {name: 'Tomorrow at 9:00'}));

        await waitFor(() => expect(onSubmit).toHaveBeenCalledWith('c1', '', expect.objectContaining({message: 'See you tomorrow'}), {}, {scheduled_at: mockTomorrow}));
    });

    // Antimatter forums: the New topic form's title rides along on whatever is sent, now or scheduled.
    describe('extraProps (Antimatter forums: the New topic form\'s title)', () => {
        const extraProps = {am_thread_title: 'Beam alignment'};

        const renderWithExtraProps = (props: Record<string, unknown> = {}) => renderWithContext(
            <Composer
                channelId='c1'
                placeholder='Write to General'
                extraProps={extraProps}
                {...props}
            />,
            {
                entities: {
                    general: {config: {ScheduledPosts: 'true'}},
                    users: {currentUserId: 'me'},
                    channels: {currentChannelId: 'c1', channels: {c1: channel}},
                },
                plugins: {components: {}},
            } as never,
        );

        test('merges extraProps onto a message sent now', async () => {
            renderWithExtraProps();

            await userEvent.type(screen.getByRole('textbox', {name: 'Write to General'}), 'Body text');
            await userEvent.click(screen.getByRole('button', {name: 'Send'}));

            await waitFor(() => expect(onSubmit).toHaveBeenCalledWith('c1', '', expect.objectContaining({props: expect.objectContaining(extraProps)}), {}, undefined));
        });

        test('merges extraProps onto a message scheduled for later', async () => {
            renderWithExtraProps();

            await userEvent.type(screen.getByRole('textbox', {name: 'Write to General'}), 'Body text');
            await userEvent.click(screen.getByRole('button', {name: 'Send later'}));
            await userEvent.click(screen.getByRole('button', {name: 'Tomorrow at 9:00'}));

            await waitFor(() => expect(onSubmit).toHaveBeenCalledWith('c1', '', expect.objectContaining({props: expect.objectContaining(extraProps)}), {}, {scheduled_at: mockTomorrow}));
        });

        test('onReady gets a function that sends the current draft', async () => {
            let send: (() => void) | undefined;
            renderWithExtraProps({onReady: (fn: () => void) => {
                send = fn;
            }});

            await userEvent.type(screen.getByRole('textbox', {name: 'Write to General'}), 'Body text');
            expect(send).toBeInstanceOf(Function);
            await act(async () => send!());

            await waitFor(() => expect(onSubmit).toHaveBeenCalledWith('c1', '', expect.objectContaining({message: 'Body text', props: expect.objectContaining(extraProps)}), {}, undefined));
        });
    });

    describe('message options', () => {
        const options = {PostPriority: 'true', EnableBurnOnRead: 'true', BurnOnReadDurationSeconds: '600', BurnOnReadMaximumDurationSeconds: '14400', BurnOnReadMaximumTimeToLiveSeconds: '604800'};
        const open = () => userEvent.click(screen.getByRole('button', {name: 'Message options'}));
        const popover = () => screen.getByRole('dialog', {name: 'Message options'});
        const sent = () => jest.mocked(onSubmit).mock.calls[0][2];

        beforeEach(() => {
            mockBurnAllowed = true;
        });

        test('one button opens them all; the formatting bar no longer has them', async () => {
            render(options);
            const bar = screen.getByRole('toolbar', {name: 'Formatting'});
            expect(within(bar).queryByRole('button', {name: /priority|burn|spoiler/i})).not.toBeInTheDocument();

            await open();
            expect(within(popover()).getByRole('group', {name: 'Message priority'})).toBeInTheDocument();
            expect(within(popover()).getByRole('switch', {name: 'Request acknowledgement'})).toBeInTheDocument();
            expect(within(popover()).getByRole('switch', {name: 'Burn on read'})).toBeInTheDocument();
            expect(within(popover()).getByRole('switch', {name: 'Send as a spoiler'})).toBeInTheDocument();
        });

        test('what is set shows as chips with its state; a chip\'s cross removes it', async () => {
            render(options);
            await open();
            await userEvent.click(within(popover()).getByRole('button', {name: 'Urgent'}));
            await userEvent.click(within(popover()).getByRole('switch', {name: 'Request acknowledgement'}));
            await userEvent.click(within(popover()).getByRole('switch', {name: 'Send as a spoiler'}));
            await userEvent.click(within(popover()).getByRole('button', {name: 'Done'}));

            expect(screen.getByRole('button', {name: 'Urgent · acknowledgement'})).toBeInTheDocument();
            expect(screen.getByRole('button', {name: 'Spoiler'})).toBeInTheDocument();
            expect(screen.getByRole('button', {name: 'Message options'})).toHaveClass('am-on');

            await userEvent.click(screen.getByRole('button', {name: 'Remove priority'}));
            expect(screen.queryByRole('button', {name: /Urgent/})).not.toBeInTheDocument();
            await userEvent.click(screen.getByRole('button', {name: 'Don\'t send as a spoiler'}));
            expect(screen.queryByRole('button', {name: 'Spoiler'})).not.toBeInTheDocument();
            expect(screen.getByRole('button', {name: 'Message options'})).not.toHaveClass('am-on');
        });

        test('a chip opens the options on its own', async () => {
            render(options);
            await open();
            await userEvent.click(within(popover()).getByRole('switch', {name: 'Send as a spoiler'}));
            await userEvent.click(within(popover()).getByRole('button', {name: 'Done'}));

            await userEvent.click(screen.getByRole('button', {name: 'Spoiler'}));
            expect(within(popover()).getByRole('switch', {name: 'Send as a spoiler'})).toHaveFocus();
        });

        test('burning on read: the default duration until one is picked, then the one picked, sent with the message', async () => {
            render(options);
            await open();
            await userEvent.click(within(popover()).getByRole('switch', {name: 'Burn on read'}));

            // The default (10 min) isn't a step: the slider shows it as a custom duration.
            expect(screen.getByRole('button', {name: 'Burns 10 min after reading'})).toBeInTheDocument();
            const slider = within(popover()).getByRole('slider', {name: 'Deleted after reading'});
            expect(slider).toHaveAttribute('aria-valuetext', 'Custom: 10 minutes');

            // Over the administrator's maximum (4 h), no step: 1 day isn't offered.
            expect(slider).toHaveAttribute('max', '5');
            fireEvent.change(slider, {target: {value: '1'}});
            expect(screen.getByRole('button', {name: 'Burns 15 min after reading'})).toBeInTheDocument();
            fireEvent.change(slider, {target: {value: '4'}});
            expect(screen.getByRole('button', {name: 'Burns 4 h after reading'})).toBeInTheDocument();
            await userEvent.click(within(popover()).getByRole('button', {name: 'Done'}));

            await userEvent.type(screen.getByRole('textbox', {name: 'Write to General'}), 'Gone soon{Enter}');
            await waitFor(() => expect(onSubmit).toHaveBeenCalled());
            expect(sent()).toEqual(expect.objectContaining({message: 'Gone soon', type: 'burn_on_read', props: expect.objectContaining({read_duration: 4 * 60 * 60 * 1000})}));
        });

        test('without a duration picked, the message carries none: the server applies its default', async () => {
            render(options);
            await open();
            await userEvent.click(within(popover()).getByRole('switch', {name: 'Burn on read'}));
            await userEvent.click(within(popover()).getByRole('button', {name: 'Done'}));
            await userEvent.type(screen.getByRole('textbox', {name: 'Write to General'}), 'Default{Enter}');

            await waitFor(() => expect(onSubmit).toHaveBeenCalled());
            expect(sent().type).toBe('burn_on_read');
            expect(sent().props?.read_duration).toBeUndefined();
        });

        test('turning burning off drops the duration picked', async () => {
            render(options);
            await open();
            await userEvent.click(within(popover()).getByRole('switch', {name: 'Burn on read'}));
            fireEvent.change(within(popover()).getByRole('slider', {name: 'Deleted after reading'}), {target: {value: '2'}});
            await userEvent.click(within(popover()).getByRole('button', {name: 'Done'}));

            await userEvent.click(screen.getByRole('button', {name: 'Turn off burn on read'}));
            expect(screen.queryByRole('button', {name: /Burns/})).not.toBeInTheDocument();
            await userEvent.type(screen.getByRole('textbox', {name: 'Write to General'}), 'Plain{Enter}');

            await waitFor(() => expect(onSubmit).toHaveBeenCalled());
            expect(sent().type).toBeUndefined();
            expect(sent().props?.read_duration).toBeUndefined();
        });

        test('in a narrow composer (a phone), they open from the "+" menu, leaving the text box its room', async () => {
            const original = window.ResizeObserver;
            window.ResizeObserver = class {
                constructor(private callback: () => void) {}
                observe() {
                    this.callback();
                }
                disconnect() {}
                unobserve() {}
            } as unknown as typeof ResizeObserver;
            const width = jest.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(320);
            try {
                render(options);
                expect(screen.queryByRole('button', {name: 'Message options'})).not.toBeInTheDocument();
                await userEvent.click(screen.getByRole('button', {name: 'Add a file and more'}));
                await userEvent.click(screen.getByRole('menuitem', {name: 'Message options'}));
                expect(popover()).toBeInTheDocument();
                await userEvent.click(within(popover()).getByRole('switch', {name: 'Send as a spoiler'}));
                expect(screen.getByRole('button', {name: 'Spoiler'})).toBeInTheDocument();
            } finally {
                width.mockRestore();
                window.ResizeObserver = original;
            }
        });

        test('no burning where the channel doesn\'t allow it', async () => {
            mockBurnAllowed = false;
            render(options);
            await open();
            expect(within(popover()).queryByRole('switch', {name: 'Burn on read'})).not.toBeInTheDocument();
            expect(within(popover()).getByRole('switch', {name: 'Send as a spoiler'})).toBeInTheDocument();
        });
    });

    // An app's way of writing a message, e.g. a poll: its question the composer's text, its options under it.
    describe('composer modes', () => {
        let submitted: string[] = [];
        let answer: string | void;
        function PollFields({message, setSubmit, onCancel, channelId}: ComposerModeProps) {
            setSubmit(async (text) => {
                submitted.push(text);
                return answer;
            });
            return (
                <div data-testid='poll-fields'>
                    {`${channelId}: ${message}`}
                    <button
                        type='button'
                        onClick={onCancel}
                    >
                        {'Plain message'}
                    </button>
                </div>
            );
        }
        const poll = {
            id: 'polls.poll',
            pluginId: 'polls',
            label: 'Create a poll',
            fusionIcon: 'poll',
            placeholder: () => 'Ask a question',
            sendLabel: 'Post poll',
            component: PollFields,
            shouldRender: () => true,
        };
        const renderWithPoll = () => render({}, {ComposerMode: [poll]});
        const enterPoll = async () => {
            await userEvent.click(screen.getByRole('button', {name: 'Add a file and more'}));
            await userEvent.click(screen.getByRole('menuitem', {name: 'Create a poll'}));
        };

        beforeEach(() => {
            submitted = [];
            answer = undefined;
        });

        test('the "+" menu offers it; in it, the text is the question and the mode\'s fields are under it', async () => {
            renderWithPoll();
            await enterPoll();

            expect(screen.getByText('Create a poll')).toBeInTheDocument();
            const question = screen.getByRole('textbox', {name: 'Ask a question'});
            await waitFor(() => expect(question).toHaveFocus());
            await userEvent.type(question, 'Lunch?');
            expect(screen.getByTestId('poll-fields')).toHaveTextContent('c1: Lunch?');

            // Send posts the poll; what doesn't apply to it goes meanwhile.
            expect(screen.getByRole('button', {name: 'Post poll'})).toBeInTheDocument();
            expect(screen.queryByRole('button', {name: 'Send later'})).not.toBeInTheDocument();
            expect(screen.queryByRole('button', {name: 'Message options'})).not.toBeInTheDocument();
            expect(screen.queryByRole('button', {name: 'Add a file and more'})).not.toBeInTheDocument();
        });

        test('Send runs the mode\'s submit with the text, then the composer is a plain, empty one again', async () => {
            renderWithPoll();
            await enterPoll();
            await userEvent.type(screen.getByRole('textbox', {name: 'Ask a question'}), 'Lunch?');
            await userEvent.click(screen.getByRole('button', {name: 'Post poll'}));

            await waitFor(() => expect(submitted).toEqual(['Lunch?']));
            expect(onSubmit).not.toHaveBeenCalled();
            await waitFor(() => expect(screen.queryByTestId('poll-fields')).not.toBeInTheDocument());
            expect(screen.getByRole('textbox', {name: 'Write to General'})).toHaveValue('');
        });

        test('what the mode says went wrong shows, and the composer stays in it', async () => {
            answer = 'A poll needs at least two options.';
            renderWithPoll();
            await enterPoll();
            await userEvent.type(screen.getByRole('textbox', {name: 'Ask a question'}), 'Lunch?{Enter}');

            expect(await screen.findByRole('alert')).toHaveTextContent('A poll needs at least two options.');
            expect(screen.getByTestId('poll-fields')).toBeInTheDocument();
            expect(screen.getByRole('textbox', {name: 'Ask a question'})).toHaveValue('Lunch?');
        });

        test('Cancel, Escape or the mode itself go back to a plain message, keeping the text', async () => {
            renderWithPoll();
            await enterPoll();
            await userEvent.type(screen.getByRole('textbox', {name: 'Ask a question'}), 'Lunch?');
            await userEvent.click(screen.getByRole('button', {name: 'Cancel: back to a plain message'}));
            expect(screen.queryByTestId('poll-fields')).not.toBeInTheDocument();
            expect(screen.getByRole('textbox', {name: 'Write to General'})).toHaveValue('Lunch?');

            await enterPoll();
            await userEvent.type(screen.getByRole('textbox', {name: 'Ask a question'}), '{Escape}');
            expect(screen.queryByTestId('poll-fields')).not.toBeInTheDocument();

            await enterPoll();
            await userEvent.click(screen.getByRole('button', {name: 'Plain message'}));
            expect(screen.queryByTestId('poll-fields')).not.toBeInTheDocument();
            expect(submitted).toEqual([]);
        });

        test('an app opens its mode in the composer of a conversation (e.g. from a slash command)', () => {
            renderWithPoll();
            act(() => {
                window.dispatchEvent(new CustomEvent(COMPOSER_MODE_EVENT, {detail: {id: 'polls.poll', channelId: 'other'}}));
            });
            expect(screen.queryByTestId('poll-fields')).not.toBeInTheDocument();
            act(() => {
                window.dispatchEvent(new CustomEvent(COMPOSER_MODE_EVENT, {detail: {id: 'polls.poll', channelId: 'c1'}}));
            });
            expect(screen.getByTestId('poll-fields')).toBeInTheDocument();
        });

        test('a mode its app no longer offers isn\'t there', async () => {
            render({}, {ComposerMode: [{...poll, shouldRender: () => false}]});
            await userEvent.click(screen.getByRole('button', {name: 'Add a file and more'}));
            expect(screen.queryByRole('menuitem', {name: 'Create a poll'})).not.toBeInTheDocument();
        });
    });

    // The plain-text textarea (the default composer, with no rich text editor flag/preference): pasted HTML
    // converts to Markdown, the same way the classic textbox does, with the same helpers (utils/paste).
    describe('plain-text paste', () => {
        const pasteHtml = (html: string, plainText = '') => {
            const textarea = screen.getByRole('textbox', {name: 'Write to General'});
            fireEvent.paste(textarea, {
                clipboardData: {
                    files: [],
                    types: ['text/html', 'text/plain'],
                    getData: (type: string) => (type === 'text/plain' ? plainText : html),
                },
            });
        };

        test.each([
            ['a table', '<table><tr><td>test</td><td>test</td></tr><tr><td>test</td><td>test</td></tr></table>', '| test | test |\n| --- | --- |\n| test | test |\n'],
            ['a link', '<a href="https://test.domain">link text</a>', '[link text](https://test.domain)'],
            ['strikethrough formatting', '<p>a <del>struck</del> word</p>', 'a ~~struck~~ word'],
            ['a fenced code block', '<pre><code class="language-js">const a = 1;\nconsole.log(a);</code></pre>', '```js\nconst a = 1;\nconsole.log(a);\n```'],
        ])('converts pasted %s to Markdown', (_name, html, expectedMarkdown) => {
            render();
            pasteHtml(html);

            expect(execCommandInsertText).toHaveBeenCalledWith(expectedMarkdown);
        });

        test('plain text with no markdown-worthy HTML on the clipboard pastes unchanged', () => {
            render();
            pasteHtml('<span style="color: red">a * b</span>', 'a * b');

            expect(execCommandInsertText).not.toHaveBeenCalled();
        });

        test('pasted files win over pasted HTML: they attach, and no markdown is inserted', () => {
            render();
            const textarea = screen.getByRole('textbox', {name: 'Write to General'});
            const file = new File(['png'], 'screenshot.png', {type: 'image/png'});

            fireEvent.paste(textarea, {
                clipboardData: {
                    files: [file],
                    types: ['Files', 'text/html', 'text/plain'],
                    getData: (type: string) => (type === 'text/plain' ? 'screenshot.png' : '<strong>screenshot.png</strong>'),
                },
            });

            expect(uploadFile).toHaveBeenCalledWith(expect.objectContaining({file, name: 'screenshot.png', channelId: 'c1'}));
            expect(execCommandInsertText).not.toHaveBeenCalled();
        });
    });

    // The rich text editor (the WysiwygEditor flag and the person's choice in the settings).
    describe('rich text editor', () => {
        const renderRich = () => renderWithContext(
            <Composer
                channelId='c1'
                placeholder='Write to General'
            />,
            {
                entities: {
                    general: {config: {FeatureFlagWysiwygEditor: 'true'}},
                    users: {currentUserId: 'me'},
                    channels: {currentChannelId: 'c1', channels: {c1: channel}},
                    preferences: {myPreferences: {'display_settings--wysiwyg_editor': {category: 'display_settings', name: 'wysiwyg_editor', user_id: 'me', value: 'true'}}},
                },
            } as never,
        );

        beforeAll(() => {
            document.elementFromPoint = () => null;
        });

        test('a pasted file is attached, and the text that comes with it (its name) is not written', () => {
            renderRich();
            const editor = document.querySelector('.am-rich-input .ProseMirror') as HTMLElement;
            expect(editor).not.toBeNull();
            const file = new File(['png'], 'screenshot.png', {type: 'image/png'});

            fireEvent.paste(editor, {
                clipboardData: {
                    files: [file],
                    types: ['Files', 'text/plain'],
                    getData: (type: string) => (type === 'text/plain' ? 'screenshot.png' : ''),
                },
            });

            expect(uploadFile).toHaveBeenCalledWith(expect.objectContaining({file, name: 'screenshot.png', channelId: 'c1'}));
            expect(editor.textContent).not.toContain('screenshot.png');
        });
    });
});
