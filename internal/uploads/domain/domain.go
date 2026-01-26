package uploadsdomain

import (
	"context"

	response "github.com/kgellert/hodatay-messenger/internal/lib"
)

type UploadStatus string

const (
	StatusPresigned UploadStatus = "presigned"
	StatusReady     UploadStatus = "ready"
	StatusFailed    UploadStatus = "failed"
)

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
