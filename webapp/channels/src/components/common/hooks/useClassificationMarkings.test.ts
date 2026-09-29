// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import * as ReactRedux from 'react-redux';

import type {PropertyField} from '@mattermost/types/properties';

import {ACCESS_CONTROL_PROPERTY_GROUP} from 'mattermost-redux/constants/properties';

import {
    CLASSIFICATIONS_CHANNEL_FIELD_NAME,
    CLASSIFICATIONS_CHANNEL_OBJECT_TYPE,
    CLASSIFICATIONS_FIELD_TARGET_TYPE,
} from 'components/admin_console/classification_markings/utils';

import {renderHookWithContext} from 'tests/react_testing_utils';

import useClassificationMarkings from './useClassificationMarkings';

type PartialState = Parameters<typeof renderHookWithContext>[1];

jest.mock('react-redux', () => ({
    __esModule: true,
    ...jest.requireActual('react-redux'),
}));

function makeChannelField(overrides: Partial<PropertyField> = {}): PropertyField {
    return {
        id: 'channel1',
        group_id: GROUP_ID,
        name: CLASSIFICATIONS_CHANNEL_FIELD_NAME,
        type: 'select',
        attrs: {options: [{id: 'lvl1', name: 'UNCLASSIFIED', color: '#007A33', rank: 1}]},
        target_id: '',
        target_type: CLASSIFICATIONS_FIELD_TARGET_TYPE,
        object_type: CLASSIFICATIONS_CHANNEL_OBJECT_TYPE,
        linked_field_id: 'template1',
        create_at: 2000,
        update_at: 2000,
        delete_at: 0,
        created_by: 'user1',
        updated_by: 'user1',
        ...overrides,
    };
}

const GROUP_ID = 'group_access_control';

function stateWith({featureFlag, fields = {}}: {
    featureFlag?: string;
    fields?: Record<string, PropertyField>;
}): PartialState {
    // Fields are looked up through the group-scoped map, which the fetch action
    // populates together with the name -> id mapping, so both are set here.
    return {
        entities: {
            general: {
                config: featureFlag === undefined ? {} : {FeatureFlagClassificationMarkings: featureFlag},
            },
            properties: {
                groups: {
                    byId: {[GROUP_ID]: {id: GROUP_ID, name: ACCESS_CONTROL_PROPERTY_GROUP}},
                    byName: {[ACCESS_CONTROL_PROPERTY_GROUP]: {id: GROUP_ID, name: ACCESS_CONTROL_PROPERTY_GROUP}},
                },
                fields: {
                    byId: fields,
                    byObjectType: {[CLASSIFICATIONS_CHANNEL_OBJECT_TYPE]: {[GROUP_ID]: fields}},
                },
                values: {byTargetId: {}, byFieldId: {}},
            },
        },
    } as PartialState;
}

describe('useClassificationMarkings', () => {
    const dispatchMock = jest.fn();

    beforeAll(() => {
        jest.spyOn(ReactRedux, 'useDispatch').mockImplementation(() => dispatchMock);
    });

    afterAll(() => {
        jest.restoreAllMocks();
    });

    beforeEach(() => {
        dispatchMock.mockClear();
    });

    test('returns available=false when feature flag is disabled', () => {
        const {result} = renderHookWithContext(
            () => useClassificationMarkings(),
            stateWith({featureFlag: 'false'}),
        );

        expect(result.current.available).toBe(false);
        expect(result.current.loading).toBe(false);
        expect(result.current.levels).toEqual([]);
        expect(dispatchMock).not.toHaveBeenCalled();
    });

    test('returns available=false when feature flag is missing from config', () => {
        const {result} = renderHookWithContext(
            () => useClassificationMarkings(),
            stateWith({}),
        );

        expect(result.current.available).toBe(false);
        expect(result.current.loading).toBe(false);
        expect(dispatchMock).not.toHaveBeenCalled();
    });

    test('returns loading=true and dispatches fetch when flag is on but no channel field', () => {
        const {result} = renderHookWithContext(
            () => useClassificationMarkings(),
            stateWith({featureFlag: 'true'}),
        );

        expect(result.current.loading).toBe(true);
        expect(result.current.available).toBe(false);
        expect(result.current.channelField).toBeNull();
        expect(result.current.levels).toEqual([]);

        // The hook dispatches one fetch for the channel field.
        expect(dispatchMock).toHaveBeenCalledTimes(1);
    });

    test('returns available=true and derives levels from channel field options', () => {
        const channel = makeChannelField({
            attrs: {
                options: [
                    {id: 'lvl2', name: 'SECRET', color: '#C8102E', rank: 2},
                    {id: 'lvl1', name: 'UNCLASSIFIED', color: '#007A33', rank: 1},
                ],
            },
        });

        const {result} = renderHookWithContext(
            () => useClassificationMarkings(),
            stateWith({featureFlag: 'true', fields: {channel1: channel}}),
        );

        expect(result.current.available).toBe(true);
        expect(result.current.loading).toBe(false);
        expect(result.current.channelField).toBe(channel);
        expect(result.current.levels).toHaveLength(2);

        // Levels are sorted by rank ascending.
        expect(result.current.levels[0].name).toBe('UNCLASSIFIED');
        expect(result.current.levels[1].name).toBe('SECRET');
    });

    test('returns available=false when channel field exists but has no options', () => {
        const channel = makeChannelField({attrs: {options: []}});

        const {result} = renderHookWithContext(
            () => useClassificationMarkings(),
            stateWith({featureFlag: 'true', fields: {channel1: channel}}),
        );

        expect(result.current.available).toBe(false);
        expect(result.current.loading).toBe(false);
        expect(result.current.channelField).toBe(channel);
        expect(result.current.levels).toEqual([]);
    });

    test('exposes channelField when it exists in the store', () => {
        const channel = makeChannelField();

        const {result} = renderHookWithContext(
            () => useClassificationMarkings(),
            stateWith({
                featureFlag: 'true',
                fields: {channel1: channel},
            }),
        );

        expect(result.current.channelField).toBe(channel);
    });

    test('returns channelField=null when channel-linked field is missing linked_field_id', () => {
        const orphan = makeChannelField({id: 'orphan', linked_field_id: ''});

        const {result} = renderHookWithContext(
            () => useClassificationMarkings(),
            stateWith({
                featureFlag: 'true',
                fields: {orphan},
            }),
        );

        expect(result.current.channelField).toBeNull();
    });

    test('returns channelField=null when channel-linked field is soft-deleted', () => {
        const deleted = makeChannelField({delete_at: 9999});

        const {result} = renderHookWithContext(
            () => useClassificationMarkings(),
            stateWith({
                featureFlag: 'true',
                fields: {channel1: deleted},
            }),
        );

        expect(result.current.channelField).toBeNull();
    });

    test('does not dispatch fetch when channel field is already in the store', () => {
        const channel = makeChannelField();

        renderHookWithContext(
            () => useClassificationMarkings(),
            stateWith({
                featureFlag: 'true',
                fields: {channel1: channel},
            }),
        );

        expect(dispatchMock).not.toHaveBeenCalled();
    });
});

