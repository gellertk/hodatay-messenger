package messagesdomain

import (
	"context"
	"database/sql"
	"time"

	response "github.com/kgellert/hodatay-messenger/internal/lib"
	uploadsdomain "github.com/kgellert/hodatay-messenger/internal/uploads/domain"
)

type Repo interface {
	SendMessage(ctx context.Context, chatID, userID int64, text string, attachments []CreateMessageAttachment, replyToMessageID *int64) (*Message, error)
	GetMessages(ctx context.Context, chatID int64) ([]Message, error)
	SetLastReadMessage(ctx context.Context, chatID, userID, lastReadMessageID int64) (int64, error)
}

func NewMessageFromRow(row MessageRow, attachments []uploadsdomain.AttachmentRow, replyAttachments []uploadsdomain.AttachmentRow) Message {
	atts := []uploadsdomain.Attachment{}
	for _, att := range attachments {
		uAtt := uploadsdomain.NewAttachmentFromRow(att)
		atts = append(atts, uAtt)
	}

	rAtts := []uploadsdomain.Attachment{}
	for _, att := range replyAttachments {
		uAtt := uploadsdomain.NewAttachmentFromRow(att)
		rAtts = append(rAtts, uAtt)
	}

	var rm *Message
	if row.ReplyTo.ID.Valid {
		rm = &Message{
			ID:           row.ReplyTo.ID.Int64,
			SenderUserID: row.ReplyTo.SenderUserID.Int64,
			Text:         row.ReplyTo.Text.String,
			CreatedAt:    row.ReplyTo.CreatedAt.Time,
			Attachments:  rAtts,
		}
	}

	return Message{
		ID:           row.ID,
		SenderUserID: row.SenderUserID,
		Text:         row.Text,
		CreatedAt:    row.CreatedAt,
		Attachments:  atts,
		ReplyTo:      rm,
	}
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

type GetMessagesResponse struct {
	response.Response
	Messages []Message `json:"messages"`
}

type MessageRow struct {
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

	Attachment        uploadsdomain.AttachmentRow `db:"attachment"`
	ReplyToAttachment uploadsdomain.AttachmentRow `db:"reply_to.attachment"`
}
