// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {showToast} from 'fusion/components/toast';

type ShowToast = (text: string) => void;

// useToast returns the function that shows a toast: one short confirmation at a time, under the main column's header,
// announced to screen readers. The frame's Toasts shows it.
export function useToast(): ShowToast {
    return showToast;
}
