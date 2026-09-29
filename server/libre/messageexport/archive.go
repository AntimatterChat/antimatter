// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package messageexport

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/libre/messageexport/shared"
	"github.com/mattermost/mattermost/server/v8/platform/shared/filestore"
)

// exportRun holds what the formats need while exporting a batch.
type exportRun struct {
	ctx         context.Context
	logger      mlog.LoggerIFace
	fileBackend filestore.FileBackend
	settings    model.MessageExportSettings
	now         time.Time

	// Per batch state.
	warnings      []string
	filesExported int
}

// warn records a warning; for zip based formats it is also written to the
// batch's warning.txt.
func (r *exportRun) warn(msg string, fields ...mlog.Field) {
	r.logger.Warn(msg, fields...)
	var sb strings.Builder
	sb.WriteString(msg)
	for _, f := range fields {
		sb.WriteString(fmt.Sprintf(" %s=%v", f.Key, fieldValue(f)))
	}
	r.warnings = append(r.warnings, sb.String())
}

func fieldValue(f mlog.Field) any {
	switch {
	case f.String != "":
		return f.String
	case f.Interface != nil:
		return f.Interface
	default:
		return f.Integer
	}
}

// writeZip streams a zip archive built by fill to path in the backend.
func writeZip(backend filestore.FileBackend, filePath string, fill func(zw *zip.Writer) error) error {
	pr, pw := io.Pipe()
	fillErr := make(chan error, 1)
	go func() {
		zw := zip.NewWriter(pw)
		err := fill(zw)
		if err == nil {
			err = zw.Close()
		}
		pw.CloseWithError(err)
		fillErr <- err
	}()

	_, writeErr := backend.WriteFile(pr, filePath)
	pr.CloseWithError(writeErr)
	err := <-fillErr
	if err == nil {
		err = writeErr
	}
	if err != nil {
		if rmErr := backend.RemoveFile(filePath); rmErr != nil {
			mlog.Debug("Unable to remove partially written export file", mlog.String("path", filePath), mlog.Err(rmErr))
		}
		return err
	}
	return nil
}

func createZipEntry(zw *zip.Writer, name string, modified time.Time) (io.Writer, error) {
	return zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: modified})
}

// copyAttachment copies the content of a file attachment into the archive at
// name. A missing or unreadable file is recorded as a warning.
func (r *exportRun) copyAttachment(zw *zip.Writer, info *model.FileInfo, name string) error {
	if r.fileBackend == nil {
		r.warn("Unable to export attachment: no file backend", mlog.String("file_id", info.Id), mlog.String("post_id", info.PostId))
		return nil
	}
	reader, err := r.fileBackend.Reader(info.Path)
	if err != nil {
		r.warn("Unable to read attachment from the file store", mlog.String("file_id", info.Id), mlog.String("post_id", info.PostId), mlog.String("path", info.Path), mlog.Err(err))
		return nil
	}
	defer reader.Close()

	w, err := createZipEntry(zw, name, time.UnixMilli(info.CreateAt))
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, reader); err != nil {
		// The archive may now contain a truncated entry; the error is fatal
		// for this batch, which will be retried.
		return fmt.Errorf("failed to copy attachment %s: %w", info.Id, err)
	}
	r.filesExported++
	return nil
}

// writeWarnings writes warning.txt when warnings were recorded.
func (r *exportRun) writeWarnings(zw *zip.Writer) error {
	if len(r.warnings) == 0 {
		return nil
	}
	w, err := createZipEntry(zw, shared.WarningFileName, r.now)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, strings.Join(r.warnings, "\n")+"\n")
	return err
}

// channelMetadata and batchMetadata are the content of metadata.json.
type channelMetadata struct {
	TeamId             *string
	TeamName           *string
	TeamDisplayName    *string
	ChannelId          string
	ChannelName        string
	ChannelDisplayName string
	ChannelType        model.ChannelType
	RoomId             string
	StartTime          int64
	EndTime            int64
	MessagesCount      int
	AttachmentsCount   int
}

type batchMetadata struct {
	Channels         map[string]channelMetadata
	MessagesCount    int
	AttachmentsCount int
	StartTime        int64
	EndTime          int64
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (r *exportRun) writeMetadata(zw *zip.Writer, b *batch) error {
	meta := batchMetadata{
		Channels:  map[string]channelMetadata{},
		StartTime: b.start,
		EndTime:   b.end,
	}
	for _, c := range b.channels {
		cm := channelMetadata{
			TeamId:             optionalString(c.TeamID),
			TeamName:           optionalString(c.TeamName),
			TeamDisplayName:    optionalString(c.TeamDisplayName),
			ChannelId:          c.ID,
			ChannelName:        c.Name,
			ChannelDisplayName: c.DisplayName,
			ChannelType:        c.Type,
			RoomId:             c.roomID(),
			StartTime:          b.start,
			EndTime:            b.end,
			MessagesCount:      len(c.posts),
		}
		for _, p := range c.posts {
			cm.AttachmentsCount += len(p.files)
		}
		meta.Channels[c.ID] = cm
		meta.MessagesCount += cm.MessagesCount
		meta.AttachmentsCount += cm.AttachmentsCount
	}

	w, err := createZipEntry(zw, shared.MetadataFileName, r.now)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(meta)
}

// batchFileName returns the name of the archive of a batch.
func batchFileName(b *batch) string {
	return fmt.Sprintf("batch%03d-%d-%d.zip", b.number, b.start, b.end)
}

// batchFilePath returns the path of the archive of a batch.
func batchFilePath(exportDir string, b *batch) string {
	return path.Join(exportDir, batchFileName(b))
}
