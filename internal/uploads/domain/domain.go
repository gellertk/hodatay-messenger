package uploadsdomain

import (
	"context"
	"database/sql"

	response "github.com/kgellert/hodatay-messenger/internal/lib"
)

type UploadStatus string

const (
	StatusPresigned UploadStatus = "presigned"
	StatusReady     UploadStatus = "ready"
	StatusFailed    UploadStatus = "failed"
)

func NewAttachmentFromRow(row AttachmentRow) Attachment {
	var width, height *int
	if row.Width.Valid {
		w := int(row.Width.Int32)
		width = &w
	}
	if row.Height.Valid {
		h := int(row.Height.Int32)
		height = &h
	}

	return Attachment{
		FileID:      row.FileID.String,
		ContentType: row.ContentType.String,
		Filename:    row.Filename.String,
		Size:        row.Size.Int64,
		Width:       width,
		Height:      height,
	}
}

type AttachmentRow struct {
	ID          sql.NullInt64  `db:"id"`
	FileID      sql.NullString `db:"file_id"`
	ContentType sql.NullString `db:"content_type"`
	Filename    sql.NullString `db:"filename"`
	Size        sql.NullInt64  `db:"size"`
	Width       sql.NullInt32  `db:"width"`
	Height      sql.NullInt32  `db:"height"`
}

type Attachment struct {
	FileID      string `json:"file_id" db:"file_id"`
	ContentType string `json:"content_type" db:"content_type"`
	Filename    string `json:"filename" db:"filename"`
	Size        int64  `json:"size" db:"size"`
	Width       *int   `json:"width" db:"width"`
	Height      *int   `json:"height" db:"height"`
}

type Repo interface {
	CreateUpload(ctx context.Context, fileID string, userID int64, contentType string, filename *string) error
	ConfirmUpload(ctx context.Context, userID int64, fileID string, contentType string, size int64, width, height *int) error
}

type Service interface {
	PresignUpload(ctx context.Context, userID int64, contentType string, filename *string) (fileID, url string, err error)
	PresignDownload(ctx context.Context, fileID string) (url string, err error)
	ConfirmUpload(ctx context.Context, userID int64, fileID string) error
}

type PresignUploadRequest struct {
	Filename    *string `json:"filename"`
	ContentType string  `json:"content_type"`
}

type ConfirmUploadRequest struct {
	FileID string `json:"file_id"`
}

type PresignUploadResponse struct {
	FileID    string `json:"file_id"`
	UploadURL string `json:"upload_url"`
}

type PresignDownloadRequest struct {
	FileID string `json:"file_id"`
}

type PresignDownloadResponse struct {
	URL string `json:"url"`
}

type PresignUploadHTTPResponse struct {
	response.Response
	PresignUploadResponse `json:"presign_upload"`
}

type PresignDownloadHTTPResponse struct {
	response.Response
	PresignDownloadResponse `json:"presign_download"`
}
