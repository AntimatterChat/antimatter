// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package compliance implements einterfaces.ComplianceInterface: it produces
// the "compliance monitoring" reports (ad-hoc and daily) as zip archives
// containing a CSV of the matching posts and a JSON description of the report.
//
// The report for a job is written to
//
//	<ComplianceSettings.Directory>compliance/<job.JobName()>.zip
//
// which is the location the download endpoint (App.GetComplianceFile) reads.
package compliance

import (
	"archive/zip"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

const (
	// ReportPostsFileName is the name of the CSV file inside a report archive.
	ReportPostsFileName = "posts.csv"
	// ReportMetadataFileName is the name of the JSON file describing the report.
	ReportMetadataFileName = "metadata.json"

	defaultBatchSize = 30000

	// dailyCheckInterval is how often we check whether the daily report for
	// the previous day still has to be generated.
	dailyCheckInterval = time.Hour

	// dailyLookupPageSize is the page size used when looking for an existing
	// daily report.
	dailyLookupPageSize = 100
)

func init() {
	app.RegisterComplianceInterface(func(a *app.App) einterfaces.ComplianceInterface {
		return New(Deps{
			Store:    func() store.Store { return a.Srv().Store() },
			Config:   a.Config,
			IsLeader: a.IsLeader,
			Logger:   a.Log(),
		})
	})
}

// Deps holds the dependencies of the compliance implementation. Functions are
// used (rather than values) so that the latest store/config are always used.
type Deps struct {
	Store    func() store.Store
	Config   func() *model.Config
	IsLeader func() bool
	Logger   mlog.LoggerIFace
	// Now returns the current time. Defaults to time.Now.
	Now func() time.Time
}

// Compliance implements einterfaces.ComplianceInterface.
type Compliance struct {
	deps Deps

	startOnce sync.Once
	dailyMut  sync.Mutex
	dailyTask *model.ScheduledTask
}

var _ einterfaces.ComplianceInterface = (*Compliance)(nil)

// New creates a new compliance implementation.
func New(deps Deps) *Compliance {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.IsLeader == nil {
		deps.IsLeader = func() bool { return true }
	}
	if deps.Logger == nil {
		deps.Logger = mlog.CreateConsoleLogger()
	}
	return &Compliance{deps: deps}
}

// ReportPath returns the path of the zip archive of a compliance report.
// It intentionally mirrors the path built by App.GetComplianceFile.
func ReportPath(cfg *model.Config, job *model.Compliance) string {
	return *cfg.ComplianceSettings.Directory + "compliance/" + job.JobName() + ".zip"
}

// StartComplianceDailyJob starts the background task generating the daily
// compliance report. The report for a given day is produced once, shortly
// after that day has ended (server local time), provided compliance and the
// daily report are enabled at that moment.
func (c *Compliance) StartComplianceDailyJob() {
	c.startOnce.Do(func() {
		c.RunDailyReportIfNeeded()
		c.dailyTask = model.CreateRecurringTask("Compliance Daily Report", func() {
			c.RunDailyReportIfNeeded()
		}, dailyCheckInterval)
	})
}

// RunDailyReportIfNeeded generates the daily report covering the previous day
// if it is enabled and has not been generated yet.
func (c *Compliance) RunDailyReportIfNeeded() {
	c.dailyMut.Lock()
	defer c.dailyMut.Unlock()

	cfg := c.deps.Config()
	if !model.SafeDereference(cfg.ComplianceSettings.Enable) || !model.SafeDereference(cfg.ComplianceSettings.EnableDaily) {
		return
	}
	if !c.deps.IsLeader() {
		return
	}

	now := c.deps.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterdayStart := todayStart.AddDate(0, 0, -1)
	desc := yesterdayStart.Format("2006-01-02")

	exists, err := c.dailyReportExists(desc, todayStart.UnixMilli())
	if err != nil {
		c.deps.Logger.Error("Unable to check for an existing daily compliance report", mlog.Err(err))
		return
	}
	if exists {
		return
	}

	job := &model.Compliance{
		Desc:    desc,
		Type:    model.ComplianceTypeDaily,
		StartAt: yesterdayStart.UnixMilli(),
		EndAt:   todayStart.UnixMilli(),
	}
	saved, err := c.deps.Store().Compliance().Save(job)
	if err != nil {
		c.deps.Logger.Error("Unable to save the daily compliance report", mlog.Err(err))
		return
	}

	rctx := request.EmptyContext(c.deps.Logger).WithLogFields(saved.LoggerFields()...)
	if appErr := c.RunComplianceJob(rctx, saved); appErr != nil {
		rctx.Logger().Error("Daily compliance report failed", mlog.Err(appErr))
	}
}

// dailyReportExists looks for a daily report with the given description that
// was created at or after createdSince.
func (c *Compliance) dailyReportExists(desc string, createdSince int64) (bool, error) {
	for offset := 0; ; offset += dailyLookupPageSize {
		reports, err := c.deps.Store().Compliance().GetAll(offset, dailyLookupPageSize)
		if err != nil {
			return false, err
		}
		for _, r := range reports {
			if r.Type == model.ComplianceTypeDaily && r.Desc == desc && r.Status != model.ComplianceStatusFailed {
				return true, nil
			}
			if r.CreateAt < createdSince {
				// Reports are sorted by CreateAt descending: nothing older
				// can be the report we are looking for.
				return false, nil
			}
		}
		if len(reports) < dailyLookupPageSize {
			return false, nil
		}
	}
}

// RunComplianceJob generates the report for the given compliance job.
func (c *Compliance) RunComplianceJob(rctx request.CTX, job *model.Compliance) *model.AppError {
	cfg := c.deps.Config()
	filePath := ReportPath(cfg, job)
	logger := rctx.Logger().With(mlog.String("file_path", filePath))

	logger.Info("Compliance job started")

	job.Status = model.ComplianceStatusRunning
	c.updateJob(logger, job)

	batchSize := model.SafeDereference(cfg.ComplianceSettings.BatchSize)
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}

	count, err := c.writeReport(job, filePath, batchSize)
	if err != nil {
		job.Status = model.ComplianceStatusFailed
		c.updateJob(logger, job)
		logger.Error("Compliance job failed", mlog.Err(err))
		return model.NewAppError("RunComplianceJob", "ent.compliance.run_failed.error",
			map[string]any{"JobName": job.JobName(), "FilePath": filePath}, "", http.StatusInternalServerError).Wrap(err)
	}

	job.Count = count
	job.Status = model.ComplianceStatusFinished
	c.updateJob(logger, job)

	logger.Info("Compliance job finished", mlog.Int("count", count))
	return nil
}

func (c *Compliance) updateJob(logger mlog.LoggerIFace, job *model.Compliance) {
	if _, err := c.deps.Store().Compliance().Update(job); err != nil {
		logger.Warn("Unable to update the compliance job", mlog.Err(err))
	}
}

// reportMetadata is the content of metadata.json in a report archive.
type reportMetadata struct {
	Id         string `json:"id"`
	Type       string `json:"type"`
	Desc       string `json:"desc"`
	UserId     string `json:"user_id,omitempty"`
	CreateAt   int64  `json:"create_at"`
	StartAt    int64  `json:"start_at"`
	EndAt      int64  `json:"end_at"`
	Keywords   string `json:"keywords,omitempty"`
	Emails     string `json:"emails,omitempty"`
	Count      int    `json:"count"`
	ExportedAt int64  `json:"exported_at"`
}

// writeReport writes the report archive for job at filePath and returns the
// number of exported posts. The archive is written to a temporary file first
// and atomically renamed, so a partially written report is never served.
func (c *Compliance) writeReport(job *model.Compliance, filePath string, batchSize int) (count int, err error) {
	if err = os.MkdirAll(filepath.Dir(filePath), 0o750); err != nil {
		return 0, fmt.Errorf("unable to create the compliance directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(filePath), ".compliance-*.zip.tmp")
	if err != nil {
		return 0, fmt.Errorf("unable to create the compliance report file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	zw := zip.NewWriter(tmp)
	count, err = c.writeArchive(zw, job, batchSize)
	if err != nil {
		return 0, err
	}
	if err = zw.Close(); err != nil {
		return 0, fmt.Errorf("unable to finalize the compliance report archive: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return 0, fmt.Errorf("unable to close the compliance report file: %w", err)
	}
	if err = os.Rename(tmpName, filePath); err != nil {
		return 0, fmt.Errorf("unable to move the compliance report in place: %w", err)
	}
	return count, nil
}

func (c *Compliance) writeArchive(zw *zip.Writer, job *model.Compliance, batchSize int) (int, error) {
	now := c.deps.Now()
	postsWriter, err := zw.CreateHeader(&zip.FileHeader{Name: ReportPostsFileName, Method: zip.Deflate, Modified: now})
	if err != nil {
		return 0, err
	}

	count, err := c.writePosts(postsWriter, job, batchSize)
	if err != nil {
		return 0, err
	}

	metaWriter, err := zw.CreateHeader(&zip.FileHeader{Name: ReportMetadataFileName, Method: zip.Deflate, Modified: now})
	if err != nil {
		return 0, err
	}
	meta := reportMetadata{
		Id:         job.Id,
		Type:       job.Type,
		Desc:       job.Desc,
		UserId:     job.UserId,
		CreateAt:   job.CreateAt,
		StartAt:    job.StartAt,
		EndAt:      job.EndAt,
		Keywords:   job.Keywords,
		Emails:     job.Emails,
		Count:      count,
		ExportedAt: now.UnixMilli(),
	}
	enc := json.NewEncoder(metaWriter)
	enc.SetIndent("", "  ")
	if err := enc.Encode(meta); err != nil {
		return 0, err
	}
	return count, nil
}

// writePosts streams the posts matching the compliance job, batch by batch,
// as CSV to w.
func (c *Compliance) writePosts(w io.Writer, job *model.Compliance, batchSize int) (int, error) {
	csvWriter := csv.NewWriter(w)
	if err := csvWriter.Write(model.CompliancePostHeader()); err != nil {
		return 0, err
	}

	count := 0
	cursor := model.ComplianceExportCursor{}
	for {
		var (
			posts []*model.CompliancePost
			err   error
		)
		posts, cursor, err = c.deps.Store().Compliance().ComplianceExport(job, cursor, batchSize)
		if err != nil {
			return 0, fmt.Errorf("unable to fetch the compliance posts: %w", err)
		}

		for _, post := range posts {
			if err := csvWriter.Write(post.Row()); err != nil {
				return 0, err
			}
		}
		count += len(posts)

		csvWriter.Flush()
		if err := csvWriter.Error(); err != nil {
			return 0, err
		}

		if cursor.ChannelsQueryCompleted && cursor.DirectMessagesQueryCompleted {
			break
		}
		if len(posts) == 0 {
			// Defensive: the store did not make progress, avoid looping forever.
			return count, errors.New("compliance export made no progress")
		}
	}

	return count, nil
}
