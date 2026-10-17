// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {defineMessages} from 'react-intl';
import type {MessageDescriptor} from 'react-intl';

// What an agent shows while it works without saying on what: a playful "-ing" word, picked at random.
export const PLAYFUL_WORDS = defineMessages({
    pondering: {id: 'fusion.agents.playful.pondering', defaultMessage: 'Pondering…'},
    tinkering: {id: 'fusion.agents.playful.tinkering', defaultMessage: 'Tinkering…'},
    untangling: {id: 'fusion.agents.playful.untangling', defaultMessage: 'Untangling…'},
    mulling: {id: 'fusion.agents.playful.mulling', defaultMessage: 'Mulling…'},
    sketching: {id: 'fusion.agents.playful.sketching', defaultMessage: 'Sketching…'},
    brewing: {id: 'fusion.agents.playful.brewing', defaultMessage: 'Brewing…'},
    weaving: {id: 'fusion.agents.playful.weaving', defaultMessage: 'Weaving…'},
    puzzling: {id: 'fusion.agents.playful.puzzling', defaultMessage: 'Puzzling…'},
    noodling: {id: 'fusion.agents.playful.noodling', defaultMessage: 'Noodling…'},
    simmering: {id: 'fusion.agents.playful.simmering', defaultMessage: 'Simmering…'},
    sorting: {id: 'fusion.agents.playful.sorting', defaultMessage: 'Sorting…'},
    piecing: {id: 'fusion.agents.playful.piecing', defaultMessage: 'Piecing together…'},
    charting: {id: 'fusion.agents.playful.charting', defaultMessage: 'Charting…'},
    polishing: {id: 'fusion.agents.playful.polishing', defaultMessage: 'Polishing…'},
    rummaging: {id: 'fusion.agents.playful.rummaging', defaultMessage: 'Rummaging…'},
    distilling: {id: 'fusion.agents.playful.distilling', defaultMessage: 'Distilling…'},
});

const WORDS: MessageDescriptor[] = Object.values(PLAYFUL_WORDS);

// pickPlayfulWord picks one of the words; random is Math.random unless a test fixes it.
export function pickPlayfulWord(random: () => number = Math.random): MessageDescriptor {
    const i = Math.min(WORDS.length - 1, Math.max(0, Math.floor(random() * WORDS.length)));
    return WORDS[i];
}
