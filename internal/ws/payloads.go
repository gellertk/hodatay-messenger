package ws

import "github.com/kgellert/hodatay-messenger/internal/messages"

type MessagesDeletePayload struct {
	IDs []int64 `json:"ids"`
}

type MessageNewPayload struct {
	Message messages.Message `json:"message"`
}

type MessageReadPayload struct {
	UserID                   		int64 `json:"user_id"`
	LastReadMessageID        		int64 `json:"last_read_message_id"`
	OthersMaxLastReadMessageID 	int64 `json:"others_max_last_read_message_id"`
}
