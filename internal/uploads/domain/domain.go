package uploadsdomain

import (
	"context"
)

type Attachment struct {
	FileID      string `json:"file_id" db:"file_id"`
	ContentType string `json:"content_type" db:"content_type"`
	Filename    string `json:"filename" db:"filename"`
	Size        int64  `json:"size" db:"size"`
	Width       int    `json:"width" db:"width"`
	Height      int    `json:"height" db:"height"`
}

type Repo interface {
	CreateUpload(ctx context.Context, key string, userID int64, filename, contentType *string) error
}

type Service interface {
	PresignUpload(ctx context.Context, userID int64, filename, contentType *string) (key, url string, err error)
	PresignDownload(ctx context.Context, key string) (url string, err error)
	GetFileInfo(ctx context.Context, key string) (fileInfo Attachment, err error)
}