package ws

import "github.com/kgellert/hodatay-messenger/internal/domain/message"

type MessageNewPayload struct {
	Message message.Message `json:"message"`
}

type MessageReadPayload struct {
	UserID                   		int64 `json:"user_id"`
	LastReadMessageID        		int64 `json:"last_read_message_id"`
	OthersMaxLastReadMessageID 	int64 `json:"others_max_last_read_message_id"`
}
