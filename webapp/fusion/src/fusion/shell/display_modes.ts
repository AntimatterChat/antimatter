// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect} from 'react';
import {useSelector} from 'react-redux';

import {getDensity, isSimplifiedInterface} from 'fusion/hooks/display';

// useDisplayModes marks <html> with the message density (data-am-density: compact, standard or spacious) and the
// simplified interface (data-am-simplified), for the styles of the app and of its layer (styles/_99_display_modes.scss),
// while the Fusion UI is shown.
export function useDisplayModes() {
    const density = useSelector(getDensity);
    const simplified = useSelector(isSimplifiedInterface);
    useEffect(() => {
        const root = document.documentElement;
        root.dataset.amDensity = density;
        if (simplified) {
            root.dataset.amSimplified = 'true';
        } else {
            delete root.dataset.amSimplified;
        }
        return () => {
            delete root.dataset.amDensity;
            delete root.dataset.amSimplified;
        };
    }, [density, simplified]);
}
