package chats

import (
	"context"

	"github.com/kgellert/hodatay-messenger/internal/messages"
	"github.com/kgellert/hodatay-messenger/internal/users"
)

type CreateChatRequest struct {
	UserIDs []int64 `json:"user_ids" db:"user_ids"`
}

type DeleteChatsRequest struct {
	ChatIDs []int64 `json:"chat_ids"`
}

type ChatRow struct {
	ChatID                     int64                       `db:"chat_id"`
	UserID                     int64                       `db:"user_id"`
	LastMessage                messages.ChatLastMessageRow `db:"last_message"`
	UnreadCount                int64                       `db:"unread_count"`
	OthersMaxLastReadMessageID int64                       `db:"others_max_last_read_message_id"`
}

type ChatListItem struct {
	ID                         int64             `json:"id" db:"chat_id"`
	Users                      []users.User      `json:"users"`
	LastMessage                *messages.Message `json:"last_message" db:"last_message"`
	UnreadCount                int64             `json:"unread_count" db:"unread_count"`
	OthersMaxLastReadMessageID int64             `json:"others_max_last_read_message_id" db:"others_max_last_read_message_id"`
}

type ChatInfo struct {
	ID    int64        `json:"id" db:"id"`
	Users []users.User `json:"users" db:"users"`
}

type GetChatsResponse struct {
	Chats []ChatListItem `json:"chats"`
}

type GetChatResponse struct {
	Chat ChatInfo `json:"chat"`
}

type DeleteChatsResponse struct {
	ChatIDs []int64 `json:"chat_ids"`
}

type GetUnreadMessagesCountResponse struct {
	UnreadCount int `json:"unread_count"`
}

type ChatsService interface {
	CreateChat(ctx context.Context, userIDs []int64) (*ChatInfo, error)
	DeleteChat(ctx context.Context, chatID int64) error
	DeleteChats(ctx context.Context, chatIDs []int64) ([]int64, error)
	GetChats(ctx context.Context, userID int64) ([]ChatListItem, error)
	GetChat(ctx context.Context, chatID int64) (*ChatInfo, error)
	GetUnreadMessagesCount(ctx context.Context, userID int64) (int, error)
}
