// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import NewTopicForm from './new_topic_form';

type MockComposerProps = {
    placeholder: string;
    extraProps?: Record<string, string>;
    onReady?: (submit: () => void) => void;
    afterSubmit?: () => void;
};

let composerProps: MockComposerProps | undefined;
const mockSubmit = jest.fn();

// A thin stand-in for the real composer (heavy, and already covered by its own tests): just enough to exercise the
// New topic form's title field, its keyboard, and what it hands the composer.
jest.mock('fusion/composer/composer', () => ({
    __esModule: true,
    default: (props: MockComposerProps) => {
        composerProps = props;
        props.onReady?.(mockSubmit);
        return (
            <textarea aria-label={props.placeholder}/>
        );
    },
}));

describe('fusion/forum/NewTopicForm', () => {
    const channel = TestHelper.getChannelMock({id: 'c1', team_id: 'team'});

    beforeEach(() => {
        composerProps = undefined;
        mockSubmit.mockClear();
    });

    test('labels the title field and the composer\'s placeholder by mode', () => {
        renderWithContext(
            <NewTopicForm
                channel={channel}
                mode='qa'
                onClose={jest.fn()}
            />,
        );

        expect(screen.getByRole('textbox', {name: 'Title'})).toHaveAttribute('placeholder', 'What do you want to ask?');
        expect(composerProps?.placeholder).toBe('Write the first post…');
    });

    test('the title is capped at 100 characters', () => {
        renderWithContext(
            <NewTopicForm
                channel={channel}
                mode='discussion'
                onClose={jest.fn()}
            />,
        );

        expect(screen.getByRole('textbox', {name: 'Title'})).toHaveAttribute('maxLength', '100');
    });

    test('Enter in the title moves focus to the body, without sending', async () => {
        renderWithContext(
            <NewTopicForm
                channel={channel}
                mode='discussion'
                onClose={jest.fn()}
            />,
        );

        await userEvent.type(screen.getByRole('textbox', {name: 'Title'}), 'Beam alignment{Enter}');

        expect(screen.getByRole('textbox', {name: 'Write the first post…'})).toHaveFocus();
        expect(mockSubmit).not.toHaveBeenCalled();
    });

    test('Ctrl+Enter in the title with no title shows a validation message, and doesn\'t send', async () => {
        renderWithContext(
            <NewTopicForm
                channel={channel}
                mode='discussion'
                onClose={jest.fn()}
            />,
        );

        screen.getByRole('textbox', {name: 'Title'}).focus();
        await userEvent.keyboard('{Control>}{Enter}{/Control}');

        expect(screen.getByRole('alert')).toHaveTextContent('A topic needs a title.');
        expect(mockSubmit).not.toHaveBeenCalled();
    });

    test('Ctrl+Enter in the title with a title sends', async () => {
        renderWithContext(
            <NewTopicForm
                channel={channel}
                mode='discussion'
                onClose={jest.fn()}
            />,
        );

        await userEvent.type(screen.getByRole('textbox', {name: 'Title'}), 'Beam alignment');
        await userEvent.keyboard('{Control>}{Enter}{/Control}');

        expect(mockSubmit).toHaveBeenCalled();
    });

    test('passes the trimmed title to the composer as props.am_thread_title', async () => {
        renderWithContext(
            <NewTopicForm
                channel={channel}
                mode='discussion'
                onClose={jest.fn()}
            />,
        );

        await userEvent.type(screen.getByRole('textbox', {name: 'Title'}), '  Beam alignment  ');

        expect(composerProps?.extraProps).toEqual({am_thread_title: 'Beam alignment'});
    });

    test('Cancel closes the form', async () => {
        const onClose = jest.fn();
        renderWithContext(
            <NewTopicForm
                channel={channel}
                mode='discussion'
                onClose={onClose}
            />,
        );

        await userEvent.click(screen.getByRole('button', {name: 'Cancel'}));

        expect(onClose).toHaveBeenCalled();
    });

    test('posting the composer (afterSubmit) closes the form', () => {
        const onClose = jest.fn();
        renderWithContext(
            <NewTopicForm
                channel={channel}
                mode='discussion'
                onClose={onClose}
            />,
        );

        composerProps?.afterSubmit?.();

        expect(onClose).toHaveBeenCalled();
    });

    test('in the image board view, leads with the attachment area', () => {
        renderWithContext(
            <NewTopicForm
                channel={channel}
                mode='discussion'
                leadWithAttachment={true}
                onClose={jest.fn()}
            />,
        );

        expect(composerProps?.placeholder).toBe('Add images, then describe them…');
        const composerBox = screen.getByRole('textbox', {name: 'Add images, then describe them…'});
        const titleBox = screen.getByRole('textbox', {name: 'Title'});

        // eslint-disable-next-line no-bitwise
        expect(composerBox.compareDocumentPosition(titleBox) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    });

    test('outside the image board view, leads with the title, as before', () => {
        renderWithContext(
            <NewTopicForm
                channel={channel}
                mode='discussion'
                onClose={jest.fn()}
            />,
        );

        expect(composerProps?.placeholder).toBe('Write the first post…');
        const titleBox = screen.getByRole('textbox', {name: 'Title'});
        const composerBox = screen.getByRole('textbox', {name: 'Write the first post…'});

        // eslint-disable-next-line no-bitwise
        expect(titleBox.compareDocumentPosition(composerBox) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    });
});
