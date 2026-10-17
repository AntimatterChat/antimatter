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
            TextFormatting.formatText('~town-square', {}, emojiMap).trim(),
        ).toBe(
            '<p>~town-square</p>',
        );
    });

    describe('Channel links', () => {
        afterEach(() => {
            delete (window as any).basename;
        });

        test('should link ~town-square', () => {
            expect(
                TextFormatting.formatText('~town-square', {
                    channelNamesMap: {'town-square': 'Town Square'},
                    team: TH.getTeamMock({name: 'myteam'}),
                }, emojiMap).trim(),
            ).toBe(
                '<p><a class="mention-link" href="/myteam/channels/town-square" data-channel-mention="town-square">~Town Square</a></p>',
            );
        });

        test('should link ~town-square followed by a period', () => {
            expect(
                TextFormatting.formatText('~town-square.', {
                    channelNamesMap: {'town-square': 'Town Square'},
                    team: TH.getTeamMock({name: 'myteam'}),
                }, emojiMap).trim(),
            ).toBe(
                '<p><a class="mention-link" href="/myteam/channels/town-square" data-channel-mention="town-square">~Town Square</a>.</p>',
            );
        });

        test('should link ~town-square, with display_name an HTML string', () => {
            expect(
                TextFormatting.formatText('~town-square', {
                    channelNamesMap: {'town-square': '<b>Reception</b>'},
                    team: TH.getTeamMock({name: 'myteam'}),
                }, emojiMap).trim(),
            ).toBe(
                '<p><a class="mention-link" href="/myteam/channels/town-square" data-channel-mention="town-square">~&lt;b&gt;Reception&lt;/b&gt;</a></p>',
            );
        });

        test('should link ~town-square, with a basename defined', () => {
            window.basename = '/subpath';
            expect(
                TextFormatting.formatText('~town-square', {
                    channelNamesMap: {'town-square': '<b>Reception</b>'},
                    team: TH.getTeamMock({name: 'myteam'}),
                }, emojiMap).trim(),
            ).toBe(
                '<p><a class="mention-link" href="/subpath/myteam/channels/town-square" data-channel-mention="town-square">~&lt;b&gt;Reception&lt;/b&gt;</a></p>',
            );
        });

        test('should link in brackets', () => {
            expect(
                TextFormatting.formatText('(~town-square)', {
                    channelNamesMap: {'town-square': 'Town Square'},
                    team: TH.getTeamMock({name: 'myteam'}),
                }, emojiMap).trim(),
            ).toBe(
                '<p>(<a class="mention-link" href="/myteam/channels/town-square" data-channel-mention="town-square">~Town Square</a>)</p>',
            );
        });
    });

    describe('invalid channel links', () => {
        test('should not link when a ~ is in the middle of a word', () => {
            expect(
                TextFormatting.formatText('aa~town-square', {
                    channelNamesMap: {'town-square': 'Town Square'},
                    team: TH.getTeamMock({name: 'myteam'}),
                }, emojiMap).trim(),
            ).toBe(
                '<p>aa~town-square</p>',
            );
        });
    });

    // Antimatter: channels are written #channel-name; a #word that names no known channel stays a hashtag.
    describe('#channel links', () => {
        const options = () => ({
            channelNamesMap: {'town-square': 'Town Square', 'off-topic': {display_name: 'Off-Topic', team_name: 'otherteam'}},
            team: TH.getTeamMock({name: 'myteam'}),
        });

        test('should link #town-square', () => {
            expect(
                TextFormatting.formatText('see #town-square', options(), emojiMap).trim(),
            ).toBe(
                '<p>see <a class="mention-link" href="/myteam/channels/town-square" data-channel-mention="town-square">#Town Square</a></p>',
            );
        });

        test('should link #town-square followed by a period, and keep ~ links working', () => {
            expect(
                TextFormatting.formatText('#town-square. ~town-square', options(), emojiMap).trim(),
            ).toBe(
                '<p><a class="mention-link" href="/myteam/channels/town-square" data-channel-mention="town-square">#Town Square</a>. ' +
                '<a class="mention-link" href="/myteam/channels/town-square" data-channel-mention="town-square">~Town Square</a></p>',
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
                TextFormatting.formatText('#town-square', {}, emojiMap).trim(),
            ).toBe(
                "<p><a class='mention-link' href='#' data-hashtag='#town-square'>#town-square</a></p>",
            );
        });

        test('should not link in the middle of a word, after & or ##, nor a heading', () => {
            expect(TextFormatting.formatText('aa#town-square', options(), emojiMap).trim()).toBe('<p>aa#town-square</p>');
            expect(TextFormatting.formatText('##town-square', options(), emojiMap).trim()).not.toContain('data-channel-mention');
            expect(TextFormatting.formatText('# town-square', options(), emojiMap).trim()).not.toContain('data-channel-mention');
            expect(TextFormatting.formatText('it\'s #town-square', options(), emojiMap).trim()).toContain('data-channel-mention="town-square"');
        });
    });
});
