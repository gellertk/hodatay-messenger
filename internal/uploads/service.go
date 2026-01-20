package uploads

import (
	"context"

	"github.com/kgellert/hodatay-messenger/internal/domain/message"
)

type UploadsService interface {
	PresignUpload(ctx context.Context, filename, contentType string) (key, url string, err error)
	PresignDownload(ctx context.Context, key string) (url string, err error)
	GetFileInfo(ctx context.Context, key string) (fileInfo message.Attachment, err error)
}