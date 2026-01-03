package message

import "time"

type Message struct {
	ID int64 `json:"id" db:"id"`
	SenderUserID int64 `json:"userID" db:"sender_user_id"`
	Text string `json:"text" db:"text"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}