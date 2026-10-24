// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package ban_purge

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

type fakeApp struct {
	calls   []string
	deleted int
	err     *model.AppError
}

func (f *fakeApp) PurgeMemberPosts(_ request.CTX, teamID, userID string, since, before int64, deletedBy string) (int, *model.AppError) {
	f.calls = append(f.calls, teamID+"/"+userID+"/"+deletedBy)
	if since != 100 || before != 200 {
		return 0, model.NewAppError("test", "wrong.range", nil, "", http.StatusBadRequest)
	}
	return f.deleted, f.err
}

func TestPurge(t *testing.T) {
	logger := mlog.CreateConsoleTestLogger(t)
	teamID, userID, adminID := model.NewId(), model.NewId(), model.NewId()
	job := func(data map[string]string) *model.Job {
		return &model.Job{Id: model.NewId(), Type: model.JobTypeBanPurge, Data: data}
	}
	valid := func() map[string]string {
		return map[string]string{"team_id": teamID, "user_id": userID, "since": "100", "before": "200", "deleted_by": adminID}
	}

	t.Run("deletes the banned person's messages of the range, and records how many", func(t *testing.T) {
		app := &fakeApp{deleted: 7}
		j := job(valid())
		require.NoError(t, purge(logger, j, app))
		assert.Equal(t, []string{teamID + "/" + userID + "/" + adminID}, app.calls)
		assert.Equal(t, "7", j.Data["deleted_posts"])
	})

	t.Run("fails when the purge stops early, keeping the count", func(t *testing.T) {
		app := &fakeApp{deleted: 3, err: model.NewAppError("test", "stopped", nil, "", http.StatusInternalServerError)}
		j := job(valid())
		require.Error(t, purge(logger, j, app))
		assert.Equal(t, "3", j.Data["deleted_posts"])
	})

	t.Run("refuses a job without a valid community, person or range", func(t *testing.T) {
		for name, change := range map[string]func(map[string]string){
			"no community": func(d map[string]string) { d["team_id"] = "" },
			"no person":    func(d map[string]string) { d["user_id"] = "nope" },
			"no start":     func(d map[string]string) { d["since"] = "soon" },
			"no end":       func(d map[string]string) { delete(d, "before") },
		} {
			data := valid()
			change(data)
			app := &fakeApp{}
			require.Error(t, purge(logger, job(data), app), name)
			assert.Empty(t, app.calls, name)
		}
	})
}
