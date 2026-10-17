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
                '<p><a class="mention-link" href="/myteam/channels/general" data-channel-mention="general">#General</a></p>',
            );
        });

        test('should link ~general followed by a period', () => {
            expect(
                TextFormatting.formatText('~general.', {
                    channelNamesMap: {general: 'General'},
                    team: TH.getTeamMock({name: 'myteam'}),
                }, emojiMap).trim(),
            ).toBe(
                '<p><a class="mention-link" href="/myteam/channels/general" data-channel-mention="general">#General</a>.</p>',
            );
        });

        test('should link ~general, with display_name an HTML string', () => {
            expect(
                TextFormatting.formatText('~general', {
                    channelNamesMap: {general: '<b>Reception</b>'},
                    team: TH.getTeamMock({name: 'myteam'}),
                }, emojiMap).trim(),
            ).toBe(
                '<p><a class="mention-link" href="/myteam/channels/general" data-channel-mention="general">#&lt;b&gt;Reception&lt;/b&gt;</a></p>',
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
                '<p><a class="mention-link" href="/subpath/myteam/channels/general" data-channel-mention="general">#&lt;b&gt;Reception&lt;/b&gt;</a></p>',
            );
        });

        test('should link in brackets', () => {
            expect(
                TextFormatting.formatText('(~general)', {
                    channelNamesMap: {general: 'General'},
                    team: TH.getTeamMock({name: 'myteam'}),
                }, emojiMap).trim(),
            ).toBe(
                '<p>(<a class="mention-link" href="/myteam/channels/general" data-channel-mention="general">#General</a>)</p>',
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

    // Antimatter: channels are written #channel-name; a #word that names no known channel stays a hashtag.
    describe('#channel links', () => {
        const options = () => ({
            channelNamesMap: {general: 'General', 'off-topic': {display_name: 'Off-Topic', team_name: 'otherteam'}},
            team: TH.getTeamMock({name: 'myteam'}),
        });

        test('should link #general', () => {
            expect(
                TextFormatting.formatText('see #general', options(), emojiMap).trim(),
            ).toBe(
                '<p>see <a class="mention-link" href="/myteam/channels/general" data-channel-mention="general">#General</a></p>',
            );
        });

        test('should link #general followed by a period, and show old ~ links as #', () => {
            expect(
                TextFormatting.formatText('#general. ~general', options(), emojiMap).trim(),
            ).toBe(
                '<p><a class="mention-link" href="/myteam/channels/general" data-channel-mention="general">#General</a>. ' +
                '<a class="mention-link" href="/myteam/channels/general" data-channel-mention="general">#General</a></p>',
            );
        });

        test('should link a channel the server resolved in another community', () => {
            expect(
                TextFormatting.formatText('#off-topic', options(), emojiMap).trim(),
            ).toBe(
                '<p><a class="mention-link" href="/otherteam/channels/off-topic" data-channel-mention-team="otherteam" data-channel-mention="off-topic">#Off-Topic</a></p>',
            );
        });

        test('should keep a hashtag when no channel has that name', () => {
            expect(
                TextFormatting.formatText('#release', options(), emojiMap).trim(),
            ).toBe(
                "<p><a class='mention-link' href='#' data-hashtag='#release'>#release</a></p>",
            );
        });

        test('should keep a hashtag without channels', () => {
            expect(
                TextFormatting.formatText('#general', {}, emojiMap).trim(),
            ).toBe(
                "<p><a class='mention-link' href='#' data-hashtag='#general'>#general</a></p>",
            );
        });

        test('should not link in the middle of a word, after & or ##, nor a heading', () => {
            expect(TextFormatting.formatText('aa#general', options(), emojiMap).trim()).toBe('<p>aa#general</p>');
            expect(TextFormatting.formatText('##general', options(), emojiMap).trim()).not.toContain('data-channel-mention');
            expect(TextFormatting.formatText('# general', options(), emojiMap).trim()).not.toContain('data-channel-mention');
            expect(TextFormatting.formatText('it\'s #general', options(), emojiMap).trim()).toContain('data-channel-mention="general"');
        });
    });
});
