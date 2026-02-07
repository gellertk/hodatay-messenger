package service

import (
	"context"

	messagesdomain "github.com/kgellert/hodatay-messenger/internal/messages"
	uploadsdomain "github.com/kgellert/hodatay-messenger/internal/uploads/domain"
)

type Repo interface {
	SendMessage(ctx context.Context, chatID, userID int64, text string, attachments []uploadsdomain.Attachment, replyToMessageID *int64) (messagesdomain.Message, error)
	GetMessages(ctx context.Context, chatID int64) ([]messagesdomain.Message, error)
	SetLastReadMessage(ctx context.Context, chatID, userID, lastReadMessageID int64) (int64, error)
	DeleteMessage(ctx context.Context, messageID int64) error
	DeleteMessages(ctx context.Context, messageIDs []int64) ([]int64, error)
}

