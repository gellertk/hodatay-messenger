package message

import "time"

type Attachment struct {
	FileID string `json:"file_id" db:"file_id"`
	ContentType string `json:"content_type" db:"content_type"`
	Filename string `json:"filename" db:"filename"`
}

type Message struct {
	ID int64 `json:"id" db:"id"`
	SenderUserID int64 `json:"user_id" db:"sender_user_id"`
	Text string `json:"text" db:"text"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	Attachments []Attachment `json:"attachments" db:"attachments"`
}