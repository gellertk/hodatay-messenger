package chats

import (
	"errors"
)

var (
	ErrEmptyParticipants = errors.New("no participants provided")
	ErrChatsNotFound     = errors.New("chats not found")
	ErrChatNotFound      = errors.New("chat not found")
	ErrChatIsNil         = errors.New("chat is nil")
)
