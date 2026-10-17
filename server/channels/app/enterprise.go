// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"github.com/mattermost/mattermost/server/v8/channels/app/autotranslation"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

var cloudInterface func(*Server) einterfaces.CloudInterface

func RegisterCloudInterface(f func(*Server) einterfaces.CloudInterface) {
	cloudInterface = f
}

func (s *Server) initEnterprise() {
	if cloudInterface != nil {
		s.Cloud = cloudInterface(s)
	}
	s.AutoTranslation = autotranslation.New(newAutoTranslationHost(s))
}
