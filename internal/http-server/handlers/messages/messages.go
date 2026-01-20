package messagesHandler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/kgellert/hodatay-messenger/internal/domain/message"
	resp "github.com/kgellert/hodatay-messenger/internal/lib/api/response"
	"github.com/kgellert/hodatay-messenger/internal/lib/logger/sl"
	"github.com/kgellert/hodatay-messenger/internal/tempuser"
	"github.com/kgellert/hodatay-messenger/internal/uploads"
	"github.com/kgellert/hodatay-messenger/internal/ws"
	"github.com/kgellert/hodatay-messenger/internal/ws/hub"
)

type MessagesHandler struct {
	Storage MessagesService
	UploadsService uploads.UploadsService
	Hub     *hub.Hub
	Log     *slog.Logger
}

type MessagesService interface {
	SendMessage(ctx context.Context, chatID, userID int64, text string, attachments []message.Attachment) (message.Message, error)
	GetMessages(ctx context.Context, chatID int64) ([]message.Message, error)
	SetLastReadMessage(ctx context.Context, chatID, userID, lastReadMessageID int64) (int64, error)
}

type SetLastReadMessageRequest struct {
	LastReadMessageID int64 `json:"last_read_message_id"`
}

type CreateMessageRequest struct {
	Text        string                    `json:"text" validate:"required,min=1,max=2000"`
	Attachments []CreateMessageAttachment `json:"attachments"`
}

type CreateMessageAttachment struct {
	FileID string `json:"file_id"`
}

type CreateMessageResponse struct {
	resp.Response
	message.Message `json:"message"`
}

type Response struct {
	resp.Response
	Messages []message.Message `json:"messages,omitempty"`
}

func New(
	storage MessagesService,
	uploadsService uploads.UploadsService,
	h *hub.Hub,
	log *slog.Logger,
) *MessagesHandler {
	return &MessagesHandler{Storage: storage, UploadsService: uploadsService, Hub: h, Log: log}
}

func (h *MessagesHandler) GetMessages() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.messages.GetMessages"

		log := h.Log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		chatIDStr := chi.URLParam(r, "chatId")
		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil || chatID <= 0 {
			log.Error("invalid chat_id", sl.Err(err))
			render.JSON(w, r, resp.Error("invalid chat_id"))
			return
		}

		messages, err := h.Storage.GetMessages(r.Context(), chatID)
		if err != nil {
			log.Error("failed to get messages", sl.Err(err))
			render.JSON(w, r, resp.Error("failed to get messages"))
			return
		}

		render.JSON(w, r, Response{
			Response: resp.OK(),
			Messages: messages,
		})
	}
}

func (h *MessagesHandler) SendMessage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.messages.SendMessage"

		log := h.Log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		chatIDStr := chi.URLParam(r, "chatId")
		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil || chatID <= 0 {
			render.JSON(w, r, resp.Error("invalid chat_id"))
			return
		}

		var req CreateMessageRequest
		if err := render.DecodeJSON(r.Body, &req); err != nil {
			render.JSON(w, r, resp.Error("invalid body"))
			return
		}

		if strings.TrimSpace(req.Text) == "" {
			render.JSON(w, r, resp.Error("text is required"))
			return
		}

		userID := tempuser.UserID(r)

		var messageAttachments []message.Attachment

		for _, att := range req.Attachments {
			fileInfo, err := h.UploadsService.GetFileInfo(r.Context(), att.FileID)
			if err != nil {
				log.Error("failed to get file info", sl.Err(err))
				render.JSON(w, r, resp.Error("failed to get file info"))
				return
			}
			messageAttachments = append(messageAttachments, fileInfo)
		}

		msg, err := h.Storage.SendMessage(r.Context(), chatID, userID, req.Text, messageAttachments)
		if err != nil {
			log.Error("failed to send message", sl.Err(err))
			render.JSON(w, r, resp.Error("failed to add message"))
			return
		}

		render.JSON(w, r, CreateMessageResponse{
			Response: resp.OK(),
			Message:  msg,
		})

		evt, err := ws.NewEvent(chatID, ws.MessageNew, ws.MessageNewPayload{Message: msg})
		if err != nil {
			log.Error("failed to build ws event", sl.Err(err))
			return
		}

		payload, err := json.Marshal(evt)
		if err != nil {
			log.Error("failed to marshal ws event", sl.Err(err))
			return
		}

		h.Hub.Broadcast(chatID, payload)
	}
}

func (h *MessagesHandler) SetLastReadMessage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.messages.SetLastReadMessage"

		log := h.Log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		chatIDStr := chi.URLParam(r, "chatId")
		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil || chatID <= 0 {
			render.JSON(w, r, resp.Error("invalid chatId"))
			return
		}

		var req SetLastReadMessageRequest
		if err := render.DecodeJSON(r.Body, &req); err != nil {
			render.JSON(w, r, resp.Error("invalid body"))
			return
		}
		if req.LastReadMessageID < 0 {
			render.JSON(w, r, resp.Error("invalid last_read_message_id"))
			return
		}

		userID := tempuser.UserID(r)

		savedLastRead, err := h.Storage.SetLastReadMessage(r.Context(), chatID, userID, req.LastReadMessageID)
		if err != nil {
			log.Error("failed to set last read message", sl.Err(err))
			render.JSON(w, r, resp.Error("failed to set last read message"))
			return
		}

		render.JSON(w, r, resp.OK())

		evt, err := ws.NewEvent(chatID, ws.MessageRead, ws.MessageReadPayload{
			UserID:            userID,
			LastReadMessageID: savedLastRead,
		})
		if err != nil {
			log.Error("failed to build ws event", sl.Err(err))
			return
		}

		payload, err := json.Marshal(evt)
		if err != nil {
			log.Error("failed to marshal ws event", sl.Err(err))
			return
		}

		h.Hub.Broadcast(chatID, payload)
	}
}
