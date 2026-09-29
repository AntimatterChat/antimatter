// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

// ClientLicense returns the feature map served to clients that still read
// /api/v4/license/client.
func (s *Server) ClientLicense() map[string]string {
	return s.platform.ClientLicense()
}
