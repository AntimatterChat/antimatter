// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

// stopComplianceDailyJob stops the daily compliance report task started with
// the server (the einterfaces interface has no such method).
func (s *Server) stopComplianceDailyJob() {
	ch := s.Channels()
	if ch == nil {
		return
	}
	if c, ok := ch.Compliance.(interface{ StopComplianceDailyJob() }); ok {
		c.StopComplianceDailyJob()
	}
}
