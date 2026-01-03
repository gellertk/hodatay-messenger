package ws

import "github.com/kgellert/hodatay-messenger/internal/domain/message"

type ServerEvent struct {
	Type   string         		`json:"type"`
	ChatID int64          		`json:"chatId"`
	Message *message.Message 	`json:"message,omitempty"`
}