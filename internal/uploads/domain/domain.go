package uploadsdomain

import (
	"context"
	"database/sql"
	"time"

	response "github.com/kgellert/hodatay-messenger/internal/lib"
)

type UploadStatus string

const (
	StatusPresigned UploadStatus = "presigned"
	StatusReady     UploadStatus = "ready"
	StatusFailed    UploadStatus = "failed"
)

func NewAttachmentFromRow(row AttachmentRow) Attachment {
	var imageInfo *ImageInfo
	if row.Width.Valid && row.Height.Valid {
		imageInfo = &ImageInfo{
			Width:  int(row.Width.Int32),
			Height: int(row.Height.Int32),
		}
	}

	var audioInfo *AudioInfo
	if row.Duration.Valid {
		audioInfo = &AudioInfo{row.Duration.Int64}
	}

	return Attachment{
		FileID:      row.FileID.String,
		ContentType: row.ContentType.String,
		Filename:    row.Filename.String,
		Size:        row.Size.Int64,
		ImageInfo:   imageInfo,
		AudioInfo:   audioInfo,
	}
}

type UploadRow struct {
	Size        int64  `db:"size"`
	Width       *int   `db:"width"`
	Height      *int   `db:"height"`
	Duration    *int64 `db:"duration"`
	ContentType string `db:"content_type"`
	Filename    string `db:"original_filename"`
	Status      string `db:"status"`
}

type AttachmentRow struct {
	ID          sql.NullInt64  `db:"id"`
	FileID      sql.NullString `db:"file_id"`
	ContentType sql.NullString `db:"content_type"`
	Filename    sql.NullString `db:"filename"`
	Size        sql.NullInt64  `db:"size"`
	Duration    sql.NullInt64  `db:"duration"`
	Width       sql.NullInt32  `db:"width"`
	Height      sql.NullInt32  `db:"height"`
}

type Attachment struct {
	FileID      string     `json:"file_id"`
	ContentType string     `json:"content_type"`
	Filename    string     `json:"filename"`
	Size        int64      `json:"size"`
	ImageInfo   *ImageInfo `json:"image_info"`
	AudioInfo   *AudioInfo `json:"audio_info"`
}

type ImageInfo struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type AudioInfo struct {
	Duration int64 `json:"duration"`
	// Waveform string   `json:"waveform"`
}

type Repo interface {
	CreateUpload(ctx context.Context, fileID string, userID int64, contentType string, filename *string) error
	ConfirmUpload(
		ctx context.Context,
		userID int64,
		fileID string,
		contentType string,
		size int64,
		width, height *int,
		duration *int64,
	) error
}

type Service interface {
	PresignUpload(ctx context.Context, userID int64, contentType string, filename *string) (*PresignUploadInfo, error)
	PresignDownload(ctx context.Context, fileID string) (url string, err error)
	ConfirmUpload(ctx context.Context, userID int64, fileID string) error
	GetPresignTTL(contentType string) time.Duration
}

type PresignUploadInfo struct {
	FileID    string
	URL       string
	ExpiresIn int
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
	ExpiresIn int    `json:"expires_in"`
}

type PresignDownloadRequest struct {
	FileID string `json:"file_id"`
}

type PresignDownloadResponse struct {
	URL       string `json:"url"`
	ExpiresIn int    `json:"expires_in"`
}

type PresignUploadHTTPResponse struct {
	response.Response
	PresignUploadResponse `json:"presign_upload"`
}

type PresignDownloadHTTPResponse struct {
	response.Response
	PresignDownloadResponse `json:"presign_download"`
}
