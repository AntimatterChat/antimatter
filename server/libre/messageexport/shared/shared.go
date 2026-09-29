// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package shared holds the identifiers shared between the message (compliance)
// export job and its clients (e.g. mmctl), mainly the keys of the job's Data
// map.
package shared

// Keys of the message export job's Data map.
const (
	// JobDataInitiatedBy records who created the job when it was not created
	// by the scheduler or the System Console (e.g. "mmctl"). Jobs initiated
	// this way do not move the starting point of the next scheduled export.
	JobDataInitiatedBy = "initiated_by"

	// JobDataExportType is the export format (csv, actiance, globalrelay,
	// globalrelay-zip). Defaults to MessageExportSettings.ExportFormat.
	JobDataExportType = "export_type"

	// JobDataExportDir is the directory, relative to the export file store,
	// where the job writes one zip file per batch.
	JobDataExportDir = "export_dir"

	// JobDataJobStartTime and JobDataJobStartId define the (exclusive)
	// starting point of the export: posts with UpdateAt > start, or equal to
	// start with Id > start id, are exported.
	JobDataJobStartTime = "job_start_time"
	JobDataJobStartId   = "job_start_id"

	// JobDataJobEndTime is the (inclusive) UpdateAt up to which posts are
	// exported.
	JobDataJobEndTime = "job_end_time"

	// JobDataBatchStartTime and JobDataBatchStartId are the position of the
	// export: the start of the next batch while the job is running, and the
	// point from which the next job continues once the job is done.
	JobDataBatchStartTime = "batch_start_time"
	JobDataBatchStartId   = "batch_start_id"

	// JobDataBatchNumber is the number of batches written so far.
	JobDataBatchNumber = "batch_number"

	// JobDataMessagesExported is the number of posts exported so far.
	JobDataMessagesExported = "messages_exported"

	// JobDataFilesExported is the number of attachments exported so far.
	JobDataFilesExported = "files_exported"

	// JobDataWarningCount is the number of warnings encountered.
	JobDataWarningCount = "warning_count"

	// JobDataIsDownloadable is "true" when the job produced files which can
	// be downloaded through the API.
	JobDataIsDownloadable = "is_downloadable"

	// JobDataProgressMessage is a human readable progress message.
	JobDataProgressMessage = "progress_message"
)

// Values of JobDataInitiatedBy.
const (
	InitiatedByMmctl = "mmctl"
	InitiatedByCLI   = "cli"
)

// Files written in each batch archive.
const (
	ActianceExportFileName = "actiance_export.xml"
	CSVExportFileName      = "posts.csv"
	MetadataFileName       = "metadata.json"
	WarningFileName        = "warning.txt"
	CSVFilesDirectory      = "files"
)
