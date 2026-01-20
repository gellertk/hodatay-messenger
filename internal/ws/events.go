package ws

import (
	"encoding/json"
	"fmt"
)

type EventType string

const (
	MessageNew  EventType = "message.new"
	MessageRead EventType = "message.read"
)

type ServerEvent struct {
	Type   EventType       `json:"type"`
	ChatID int64           `json:"chat_id"`
	Data   json.RawMessage `json:"data,omitempty"`
}

func NewEvent(chatID int64, typ EventType, payload any) (ServerEvent, error) {
	var raw json.RawMessage

	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return ServerEvent{}, fmt.Errorf("marshal payload: %w", err)
		}
		raw = b
	}
	return ServerEvent{
		Type: typ,
		ChatID: chatID,
		Data: raw,
	}, nil
}
