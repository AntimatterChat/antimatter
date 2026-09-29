// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import AdminDefinition from './admin_definition';

describe('AdminDefinition - system_attributes/user_attributes always has a registered route', () => {
    // If this subsection is ever hidden, the admin console has no route for
    // that URL and silently redirects elsewhere (admin_console.tsx's catch-all
    // <Redirect>) instead of showing anything.
    test('user_attributes_redirect is always visible', () => {
        const subsection = AdminDefinition.system_attributes.subsections.user_attributes_redirect;
        expect(subsection.url).toBe('system_attributes/user_attributes');
        expect(subsection.isHidden).toBeUndefined();
    });
});
