// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package messageexport

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"sort"

	"github.com/mattermost/mattermost/server/v8/libre/messageexport/shared"
)

// The Actiance (Smarsh Vantage) XML format: a FileDump root containing one
// Conversation per channel. A conversation lists, in order, the room
// identifier and start time, the participants who entered, the messages and
// file transfers, the participants who left and the end time. All times are
// Unix timestamps in milliseconds. Empty optional fields are omitted.

type actianceParticipant struct {
	XMLName     xml.Name
	LoginName   string `xml:"LoginName"`
	UserType    string `xml:"UserType,omitempty"`
	DateTimeUTC int64  `xml:"DateTimeUTC"`
}

type actianceMessage struct {
	XMLName            xml.Name `xml:"Message"`
	MessageId          string   `xml:"MessageId,omitempty"`
	LoginName          string   `xml:"LoginName"`
	UserType           string   `xml:"UserType,omitempty"`
	DateTimeUTC        int64    `xml:"DateTimeUTC"`
	UpdatedDateTimeUTC int64    `xml:"UpdatedDateTimeUTC,omitempty"`
	UpdatedType        string   `xml:"UpdatedType,omitempty"`
	EditedNewMsgId     string   `xml:"EditedNewMsgId,omitempty"`
	Content            string   `xml:"Content,omitempty"`
	PreviewsPost       string   `xml:"PreviewsPost,omitempty"`
}

type actianceFileTransfer struct {
	XMLName            xml.Name
	MessageId          string `xml:"MessageId,omitempty"`
	LoginName          string `xml:"LoginName"`
	UserType           string `xml:"UserType,omitempty"`
	DateTimeUTC        int64  `xml:"DateTimeUTC"`
	UpdatedDateTimeUTC int64  `xml:"UpdatedDateTimeUTC,omitempty"`
	UpdatedType        string `xml:"UpdatedType,omitempty"`
	UserFileName       string `xml:"UserFileName"`
	FileName           string `xml:"FileName"`
	Status             string `xml:"Status,omitempty"`
}

type timedElement struct {
	time  int64
	seq   int
	value any
}

// actianceConversationElements returns the ordered elements of a channel's
// conversation (without RoomID/StartTimeUTC/EndTimeUTC).
func actianceConversationElements(b *batch, c *channel) []any {
	var entered, body, left []timedElement
	seq := 0
	next := func() int { seq++; return seq }

	for _, p := range c.participants {
		for _, m := range p.Intervals {
			entered = append(entered, timedElement{time: m.JoinTime, seq: next(), value: actianceParticipant{
				XMLName:     xml.Name{Local: "ParticipantEntered"},
				LoginName:   p.loginName(),
				UserType:    p.userType(),
				DateTimeUTC: m.JoinTime,
			}})
			leaveTime := m.LeaveTime
			if leaveTime == 0 || leaveTime > b.end {
				// Still a member at the end of the conversation.
				leaveTime = b.end
			}
			left = append(left, timedElement{time: leaveTime, seq: next(), value: actianceParticipant{
				XMLName:     xml.Name{Local: "ParticipantLeft"},
				LoginName:   p.loginName(),
				UserType:    p.userType(),
				DateTimeUTC: leaveTime,
			}})
		}
	}

	for _, p := range c.posts {
		updatedType, editedNewMsgID := p.updatedType()
		var updatedAt int64
		if updatedType != "" {
			updatedAt = p.updateAt()
		}
		body = append(body, timedElement{time: p.createAt(), seq: next(), value: actianceMessage{
			MessageId:          p.id(),
			LoginName:          p.loginName(),
			UserType:           p.userType(),
			DateTimeUTC:        p.createAt(),
			UpdatedDateTimeUTC: updatedAt,
			UpdatedType:        updatedType,
			EditedNewMsgId:     editedNewMsgID,
			Content:            p.message(),
			PreviewsPost:       p.previewedID(),
		}})

		for _, info := range p.files {
			transfer := actianceFileTransfer{
				MessageId:          p.id(),
				LoginName:          p.loginName(),
				UserType:           p.userType(),
				DateTimeUTC:        p.createAt(),
				UpdatedDateTimeUTC: updatedAt,
				UpdatedType:        updatedType,
				UserFileName:       info.Name,
				FileName:           info.Path,
			}
			started := transfer
			started.XMLName = xml.Name{Local: "FileTransferStarted"}
			ended := transfer
			ended.XMLName = xml.Name{Local: "FileTransferEnded"}
			ended.Status = "Completed"
			body = append(body,
				timedElement{time: p.createAt(), seq: next(), value: started},
				timedElement{time: p.createAt(), seq: next(), value: ended})

			if info.DeleteAt > 0 {
				deleted := transfer
				deleted.XMLName = xml.Name{Local: "FileTransferEnded"}
				deleted.UpdatedDateTimeUTC = info.DeleteAt
				deleted.UpdatedType = UpdatedTypeFileDeleted
				deleted.Status = "Completed"
				body = append(body, timedElement{time: p.createAt(), seq: next(), value: deleted})
			}
		}
	}

	var out []any
	for _, group := range [][]timedElement{entered, body, left} {
		sort.SliceStable(group, func(i, j int) bool {
			if group[i].time != group[j].time {
				return group[i].time < group[j].time
			}
			return group[i].seq < group[j].seq
		})
		for _, e := range group {
			out = append(out, e.value)
		}
	}
	return out
}

// writeActianceXML writes the XML document of a batch.
func writeActianceXML(w io.Writer, b *batch) error {
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")

	root := xml.StartElement{
		Name: xml.Name{Local: "FileDump"},
		Attr: []xml.Attr{{Name: xml.Name{Local: "xmlns:xsi"}, Value: "http://www.w3.org/2001/XMLSchema-instance"}},
	}
	if err := enc.EncodeToken(root); err != nil {
		return err
	}

	for _, c := range b.channels {
		conv := xml.StartElement{
			Name: xml.Name{Local: "Conversation"},
			Attr: []xml.Attr{{Name: xml.Name{Local: "Perspective"}, Value: c.DisplayName}},
		}
		if err := enc.EncodeToken(conv); err != nil {
			return err
		}
		if err := enc.EncodeElement(c.roomID(), xml.StartElement{Name: xml.Name{Local: "RoomID"}}); err != nil {
			return err
		}
		if err := enc.EncodeElement(b.start, xml.StartElement{Name: xml.Name{Local: "StartTimeUTC"}}); err != nil {
			return err
		}
		for _, el := range actianceConversationElements(b, c) {
			if err := enc.Encode(el); err != nil {
				return err
			}
		}
		if err := enc.EncodeElement(b.end, xml.StartElement{Name: xml.Name{Local: "EndTimeUTC"}}); err != nil {
			return err
		}
		if err := enc.EncodeToken(conv.End()); err != nil {
			return err
		}
	}

	if err := enc.EncodeToken(root.End()); err != nil {
		return err
	}
	if err := enc.Flush(); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

// writeActianceBatch fills the archive of an Actiance batch:
// actiance_export.xml, metadata.json, the attachments (stored at their file
// store path, as referenced by the FileName elements) and warning.txt when
// needed.
func (r *exportRun) writeActianceBatch(zw *zip.Writer, b *batch) error {
	w, err := createZipEntry(zw, shared.ActianceExportFileName, r.now)
	if err != nil {
		return err
	}
	if err := writeActianceXML(w, b); err != nil {
		return err
	}
	if err := r.writeMetadata(zw, b); err != nil {
		return err
	}

	seen := map[string]bool{}
	for _, p := range b.posts {
		for _, info := range p.files {
			if seen[info.Path] || info.Path == "" {
				continue
			}
			seen[info.Path] = true
			if err := r.ctx.Err(); err != nil {
				return err
			}
			if err := r.copyAttachment(zw, info, info.Path); err != nil {
				return err
			}
		}
	}
	return r.writeWarnings(zw)
}
