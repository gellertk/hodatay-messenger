package messagesdomain

import (
	"context"
	"database/sql"
	"time"

	"github.com/kgellert/hodatay-messenger/internal/lib"
	"github.com/kgellert/hodatay-messenger/internal/uploads/domain"
)

type Repo interface {
	SendMessage(ctx context.Context, chatID, userID int64, text string, attachments []uploadsdomain.Attachment, replyToMessageID *int64) (Message, error)
	GetMessages(ctx context.Context, chatID int64) ([]Message, error)
	SetLastReadMessage(ctx context.Context, chatID, userID, lastReadMessageID int64) (int64, error)
}

type Message struct {
	ID           int64                      `json:"id" db:"id"`
	SenderUserID int64                      `json:"user_id" db:"sender_user_id"`
	Text         string                     `json:"text" db:"text"`
	CreatedAt    time.Time                  `json:"created_at" db:"created_at"`
	Attachments  []uploadsdomain.Attachment `json:"attachments" db:"attachments"`
	ReplyTo      *Message                   `json:"reply_to" db:"reply_to"`
}

type SetLastReadMessageRequest struct {
	LastReadMessageID int64 `json:"last_read_message_id"`
}

type CreateMessageRequest struct {
	Text             string                    `json:"text"`
	Attachments      []CreateMessageAttachment `json:"attachments"`
	ReplyToMessageID *int64                    `json:"reply_to_message_id"`
}

type CreateMessageAttachment struct {
	FileID string `json:"file_id"`
}

type CreateMessageResponse struct {
	response.Response
	Message `json:"message"`
}

type Response struct {
	response.Response
	Messages []Message `json:"messages,omitempty"`
}

type MsgRow struct {
	ID           int64     `db:"id"`
	SenderUserID int64     `db:"sender_user_id"`
	Text         string    `db:"text"`
	CreatedAt    time.Time `db:"created_at"`

	ReplyTo struct {
		ID           sql.NullInt64  `db:"id"`
		SenderUserID sql.NullInt64  `db:"sender_user_id"`
		Text         sql.NullString `db:"text"`
		CreatedAt    sql.NullTime   `db:"created_at"`
	} `db:"reply_to"`

	Attachment struct {
		ID          sql.NullInt64  `db:"id"`
		Key         sql.NullString `db:"key"`
		ContentType sql.NullString `db:"content_type"`
		Filename    sql.NullString `db:"filename"`
	} `db:"attachments"`

	ReplyToAttachment struct {
		ID          sql.NullInt64  `db:"id"`
		Key         sql.NullString `db:"key"`
		ContentType sql.NullString `db:"content_type"`
		Filename    sql.NullString `db:"filename"`
	} `db:"reply_to.attachments"`
}
