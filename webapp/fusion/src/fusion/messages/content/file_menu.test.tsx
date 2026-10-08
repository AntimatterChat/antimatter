// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {getHistory} from 'utils/browser_history';
import {TestHelper} from 'utils/test_helper';
import {copyToClipboard} from 'utils/utils';

import FileMenu from './file_menu';

jest.mock('utils/utils', () => ({...jest.requireActual('utils/utils'), copyToClipboard: jest.fn()}));
jest.mock('fusion/shell/toast_context', () => ({useToast: () => jest.fn()}));
jest.mock('mattermost-redux/actions/files', () => ({getFilePublicLink: jest.fn(() => ({type: 'LINK', data: {link: 'https://chat.example/files/public'}}))}));

describe('fusion/messages/content/FileMenu', () => {
    const file = TestHelper.getFileInfoMock({id: 'file1', name: 'notes.pdf'});
    const post = TestHelper.getPostMock({id: 'post1'});
    const team = TestHelper.getTeamMock({id: 'team', name: 'antimatter'});

    beforeEach(() => {
        jest.clearAllMocks();
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const render = (publicLinks: boolean) => renderWithContext(
        <FileMenu
            file={file}
            post={post}
            point={{x: 10, y: 10}}
            onClose={jest.fn()}
        />,
        {entities: {general: {config: {EnablePublicLink: String(publicLinks)}}, teams: {currentTeamId: team.id, teams: {[team.id]: team}}}} as never,
    );

    test('copies the public link when the server allows them', async () => {
        render(true);
        await userEvent.click(screen.getByRole('menuitem', {name: 'Copy public link'}));
        await Promise.resolve();
        expect(copyToClipboard).toHaveBeenCalledWith('https://chat.example/files/public');
    });

    test('copies the file\'s own link otherwise', async () => {
        render(false);
        await userEvent.click(screen.getByRole('menuitem', {name: 'Copy link'}));
        expect(copyToClipboard).toHaveBeenCalledWith(expect.stringContaining('/files/file1'));
    });

    test('opens the message in its channel', async () => {
        render(false);
        await userEvent.click(screen.getByRole('menuitem', {name: 'Open in channel'}));
        expect(getHistory().push).toHaveBeenCalledWith('/antimatter/pl/post1');
    });
});
