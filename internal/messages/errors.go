package messages

import (
	"errors"
)

var (
	ErrTextOrAttachmentsIsRequired = errors.New("text or attachments is required")
	ErrInvalidLastReadMessageId    = errors.New("invalid lastReadMessageId")
	ErrMessageIsNil                = errors.New("message is nil")
	ErrMessageIsNotExist           = errors.New("message is not exist")
	ErrMessagesIsNotExist          = errors.New("messages is not exist")
)
