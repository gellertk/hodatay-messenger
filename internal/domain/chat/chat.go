package chat

import (
	"github.com/kgellert/hodatay-messenger/internal/domain/message"
	"github.com/kgellert/hodatay-messenger/internal/domain/user"
)

type ChatListItem struct {
	ID              	int64       		`json:"id" db:"chat_id"`
	Users           	[]user.User 		`json:"users"`
	LastMessage     	message.Message `json:"last_message" db:"last_message"`
	UnreadCount 			int64 					`json:"unread_count" db:"unread_count"`
	OthersMaxLastReadMessageID int64  `json:"others_max_last_read_message_id" db:"others_max_last_read_message_id"`
}

type ChatInfo struct {
  ID    int64       `json:"id" db:"id"`
  Users []user.User `json:"users" db:"users"`
}