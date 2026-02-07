package uploads

import (
	"errors"
)

var (
	ErrInvalidFileId         = errors.New("invalid file id")
	ErrContentTypeIsRequired = errors.New("contentType is required")
	ErrInvalidContentType    = errors.New("invalid contentType")
)
