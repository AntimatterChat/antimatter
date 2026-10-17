// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package jobs

import "github.com/mattermost/mattermost/server/public/model"

// JobType returns the worker and the scheduler registered for the job type,
// nil when there is none.
func (srv *JobServer) JobType(name string) (model.Worker, Scheduler) {
	srv.mut.Lock()
	defer srv.mut.Unlock()

	var worker model.Worker
	var scheduler Scheduler
	if srv.workers != nil {
		worker = srv.workers.Get(name)
	}
	if srv.schedulers != nil {
		scheduler = srv.schedulers.schedulers[name]
	}
	return worker, scheduler
}
