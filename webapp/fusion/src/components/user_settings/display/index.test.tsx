// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {UserProfile} from '@mattermost/types/users';

import {getLanguageInfo} from 'i18n/i18n';
import mergeObjects from 'packages/mattermost-redux/test/merge_objects';
import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';

import {makeMapStateToProps} from './index';

describe('components/user_settings/display/index', () => {
    const user = TestHelper.getUserMock({id: 'user_id', locale: 'de'});

    const baseState = {
        entities: {
            general: {
                config: {
                    DefaultClientLocale: 'en',
                },
                license: {},
            },
            users: {
                currentUserId: 'user_id',
                profiles: {user_id: user},
            },
            preferences: {myPreferences: {}},
            teams: {teams: {}, myMembers: {}},
            channels: {channels: {}, myMembers: {}},
        },
    } as unknown as GlobalState;

    const ownProps = {adminMode: false, user} as {adminMode: boolean; user: UserProfile};

    function userLocaleFor(state: GlobalState, locale: string) {
        const mapStateToProps = makeMapStateToProps();
        return mapStateToProps(state, {...ownProps, user: {...user, locale}}).userLocale;
    }

    test('keeps a supported user locale', () => {
        expect(userLocaleFor(baseState, 'de')).toBe('de');
    });

    // Lithuanian is one of upstream's languages that the Fusion UI doesn't ship (it has Czech, which upstream's test used).
    test('falls back to DefaultClientLocale when the user locale is not supported', () => {
        expect(userLocaleFor(baseState, 'lt')).toBe('en');
    });

    test('falls back to English when DefaultClientLocale is not supported either', () => {
        // fixInvalidLocales normalizes this server side, but the settings modal
        // reads .name off the result without a guard, so the fallback has to
        // resolve on its own.
        const state = mergeObjects(baseState, {
            entities: {general: {config: {DefaultClientLocale: 'lt'}}},
        });

        const userLocale = userLocaleFor(state, 'lt');
        expect(userLocale).toBe('en');
        expect(getLanguageInfo(userLocale)).toBeDefined();
    });

    test('falls back to English when AvailableLocales excludes DefaultClientLocale', () => {
        const state = mergeObjects(baseState, {
            entities: {general: {config: {AvailableLocales: 'fr', DefaultClientLocale: 'lt'}}},
        });

        const userLocale = userLocaleFor(state, 'de');
        expect(userLocale).toBe('en');
        expect(getLanguageInfo(userLocale)).toBeDefined();
    });
});
