// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import EmojiMap from 'utils/emoji_map';
import {TestHelper as TH} from 'utils/test_helper';
import * as TextFormatting from 'utils/text_formatting';
const emojiMap = new EmojiMap(new Map());

describe('TextFormatting.ChannelLinks', () => {
    test('Not channel links', () => {
        expect(
            TextFormatting.formatText('~123', {}, emojiMap).trim(),
        ).toBe(
            '<p>~123</p>',
        );

        expect(
            TextFormatting.formatText('~general', {}, emojiMap).trim(),
        ).toBe(
            '<p>~general</p>',
        );
    });

    describe('Channel links', () => {
        afterEach(() => {
            delete (window as any).basename;
        });

        test('should link ~general', () => {
            expect(
                TextFormatting.formatText('~general', {
                    channelNamesMap: {general: 'General'},
                    team: TH.getTeamMock({name: 'myteam'}),
                }, emojiMap).trim(),
            ).toBe(
                '<p><a class="mention-link" href="/myteam/channels/general" data-channel-mention="general">~General</a></p>',
            );
        });

        test('should link ~general followed by a period', () => {
            expect(
                TextFormatting.formatText('~general.', {
                    channelNamesMap: {general: 'General'},
                    team: TH.getTeamMock({name: 'myteam'}),
                }, emojiMap).trim(),
            ).toBe(
                '<p><a class="mention-link" href="/myteam/channels/general" data-channel-mention="general">~General</a>.</p>',
            );
        });

        test('should link ~general, with display_name an HTML string', () => {
            expect(
                TextFormatting.formatText('~general', {
                    channelNamesMap: {general: '<b>Reception</b>'},
                    team: TH.getTeamMock({name: 'myteam'}),
                }, emojiMap).trim(),
            ).toBe(
                '<p><a class="mention-link" href="/myteam/channels/general" data-channel-mention="general">~&lt;b&gt;Reception&lt;/b&gt;</a></p>',
            );
        });

        test('should link ~general, with a basename defined', () => {
            window.basename = '/subpath';
            expect(
                TextFormatting.formatText('~general', {
                    channelNamesMap: {general: '<b>Reception</b>'},
                    team: TH.getTeamMock({name: 'myteam'}),
                }, emojiMap).trim(),
            ).toBe(
                '<p><a class="mention-link" href="/subpath/myteam/channels/general" data-channel-mention="general">~&lt;b&gt;Reception&lt;/b&gt;</a></p>',
            );
        });

        test('should link in brackets', () => {
            expect(
                TextFormatting.formatText('(~general)', {
                    channelNamesMap: {general: 'General'},
                    team: TH.getTeamMock({name: 'myteam'}),
                }, emojiMap).trim(),
            ).toBe(
                '<p>(<a class="mention-link" href="/myteam/channels/general" data-channel-mention="general">~General</a>)</p>',
            );
        });
    });

    describe('invalid channel links', () => {
        test('should not link when a ~ is in the middle of a word', () => {
            expect(
                TextFormatting.formatText('aa~general', {
                    channelNamesMap: {general: 'General'},
                    team: TH.getTeamMock({name: 'myteam'}),
                }, emojiMap).trim(),
            ).toBe(
                '<p>aa~general</p>',
            );
        });
    });
});
