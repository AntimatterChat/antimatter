// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package platform

import (
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

var accessControlServiceInterface func(*PlatformService) einterfaces.AccessControlServiceInterface

func RegisterAccessControlServiceInterface(f func(*PlatformService) einterfaces.AccessControlServiceInterface) {
	accessControlServiceInterface = f
}
