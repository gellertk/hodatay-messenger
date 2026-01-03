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
	"github.com/kgellert/hodatay-messenger/internal/ws"
	"github.com/kgellert/hodatay-messenger/internal/ws/hub"
)

type MessagesHandler struct {
    Storage MessagesService
    Hub     *hub.Hub
    Log     *slog.Logger
}

type MessagesService interface {
	SendMessage(ctx context.Context, chatID, userID int64, text string) (message.Message, error)
    GetMessages(ctx context.Context, chatID int64) ([]message.Message, error)
	SetLastReadMessage(ctx context.Context, chatID, userID, lastReadMessageID int64) error
}

type SetLastReadMessageRequest struct {
    LastReadMessageID int64 `json:"lastReadMessageID"`
}

type CreateMessageRequest struct {
    Text string `json:"text" validate:"required,min=1,max=2000"`
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
    h *hub.Hub,
    log *slog.Logger,
) *MessagesHandler {
    return &MessagesHandler{Storage: storage, Hub: h, Log: log}
}

func (h *MessagesHandler) GetMessages() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.messages.GetMessages"

		log := h.Log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		chatIDStr := chi.URLParam(r, "chatID")
		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil {
			log.Error("invalid chat_id", sl.Err(err))
    		render.JSON(w, r, resp.Error("invalid chat_id"))
    		return
		}

		if chatID <= 0 {
    	render.JSON(w, r, resp.Error("invalid chat_id"))
    	return
		}

		log.Info("chat_id parsed", slog.Int64("chat_id", chatID))

		messages, err := h.Storage.GetMessages(r.Context(), chatID)

		if err != nil { // обработать конкретные ошибки
			log.Error("Failed to get messages", sl.Err(err)) // Добавить кастомную обработку ошибки
			render.JSON(w, r, resp.Error("failed to get messages"))
			return
		}

		log.Info("Messages fetched", slog.Any("messages", messages))

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

		chatIDStr := chi.URLParam(r, "chatID")
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

		msg, err := h.Storage.SendMessage(r.Context(), chatID, 2, req.Text)
		if err != nil {
			log.Error("failed to add message", sl.Err(err))
			render.JSON(w, r, resp.Error("failed to add message"))
			return
		}

		// 1) отвечаем по HTTP
		render.JSON(w, r, CreateMessageResponse{
			Response: resp.OK(),
			Message:  msg,
		})

		// 2) пушим событие по WS (не мешает HTTP-ответу)
		evt := ws.ServerEvent{
			Type:    "message.new",
			ChatID:  chatID,
			Message: &msg,
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

        chatIDStr := chi.URLParam(r, "chatID")
        chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
        if err != nil || chatID <= 0 {
            render.JSON(w, r, resp.Error("invalid chat_id"))
            return
        }

		var req SetLastReadMessageRequest
        if err := render.DecodeJSON(r.Body, &req); err != nil {
            render.JSON(w, r, resp.Error("invalid body"))
            return
        }

        if req.LastReadMessageID < 0 {
			log.Error("invalid last_read_message_id", sl.Err(err))
            render.JSON(w, r, resp.Error("invalid last_read_message_id"))
            return
        }

        err = h.Storage.SetLastReadMessage(
            r.Context(),
            chatID,
            1,
            req.LastReadMessageID,
        )

        if err != nil {
            log.Error("failed to add message", sl.Err(err))
            render.JSON(w, r, resp.Error("failed to add message"))
            return
        }

		render.JSON(w, r, resp.OK())
    }
}
