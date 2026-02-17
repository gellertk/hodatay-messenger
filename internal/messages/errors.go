package messages

import (
	"errors"
)

var (
	ErrTextOrAttachmentsIsRequired = errors.New("text or attachments is required")
	ErrInvalidLastReadMessageId    = errors.New("invalid last_read_message_id")
	ErrMessageIsNil                = errors.New("message is nil")
	ErrMessageIsNotExist           = errors.New("message is not exist")
	ErrMessagesIsNotExist          = errors.New("messages is not exist")
	ErrInvalidPage                 = errors.New("invalid page")
	ErrInvalidLimit                = errors.New("invalid limit")
)
