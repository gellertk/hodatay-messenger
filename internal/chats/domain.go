package chatsdomain

import (
	"context"

	response "github.com/kgellert/hodatay-messenger/internal/lib"
	messagesdomain "github.com/kgellert/hodatay-messenger/internal/messages"
	"github.com/kgellert/hodatay-messenger/internal/users/domain"
)

// type ChatsRow struct {
// 	ChatID int64 `json:"chat_id" db:"chat_id"`
// 	UserID int64 `json:"user_id" db:"user_id"`
// }

type ChatsRow struct {
	ChatID                     int64                  `db:"chat_id"`
	UserID                     int64                  `db:"user_id"`
	LastMessage                messagesdomain.Message `db:"last_message"`
	UnreadCount                int64                  `db:"unread_count"`
	OthersMaxLastReadMessageID int64                  `db:"others_max_last_read_message_id"`
}

type ChatListItem struct {
	ID                         int64                  `json:"id" db:"chat_id"`
	Users                      []userdomain.User      `json:"users"`
	LastMessage                messagesdomain.Message `json:"last_message" db:"last_message"`
	UnreadCount                int64                  `json:"unread_count" db:"unread_count"`
	OthersMaxLastReadMessageID int64                  `json:"others_max_last_read_message_id" db:"others_max_last_read_message_id"`
}

type ChatInfo struct {
	ID    int64             `json:"id" db:"id"`
	Users []userdomain.User `json:"users" db:"users"`
}

type GetChatsResponse struct {
	response.Response
	Chats []ChatListItem `json:"chats,omitempty"`
}

type GetChatResponse struct {
	response.Response
	Chat ChatInfo `json:"chat"`
}

type ChatsService interface {
	GetChats(ctx context.Context, userID int64) ([]ChatListItem, error)
	GetChat(ctx context.Context, chatID int64) (ChatInfo, error)
}
