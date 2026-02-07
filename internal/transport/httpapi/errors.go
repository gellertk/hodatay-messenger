package httpapi

import (
	"errors"
	"net/http"

	"github.com/kgellert/hodatay-messenger/internal/chats"
	"github.com/kgellert/hodatay-messenger/internal/messages"
)

func MapError(err error) (status int, code, msg string) {
	switch {
	case errors.Is(err, chats.ErrChatNotFound):
		return http.StatusNotFound, "chat_not_found", err.Error()

	case errors.Is(err, chats.ErrChatsNotFound):
		return http.StatusNotFound, "chats_not_found", err.Error()

	case errors.Is(err, chats.ErrEmptyParticipants):
		return http.StatusBadRequest, "empty_participants", err.Error()

	case errors.Is(err, messages.ErrTextOrAttachmentsIsRequired):
		return http.StatusBadRequest, "text_or_attachments_required", err.Error()

	case errors.Is(err, messages.ErrInvalidLastReadMessageId):
		return http.StatusBadRequest, "invalid_last_read_message_id", err.Error()
	}

	return http.StatusInternalServerError, "internal_error", "internal server error"
}
