package messageshandler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	response "github.com/kgellert/hodatay-messenger/internal/lib"
	"github.com/kgellert/hodatay-messenger/internal/logger/sl"
	messagesdomain "github.com/kgellert/hodatay-messenger/internal/messages"
	uploads "github.com/kgellert/hodatay-messenger/internal/uploads/domain"
	userhandlers "github.com/kgellert/hodatay-messenger/internal/users/handlers"
	"github.com/kgellert/hodatay-messenger/internal/ws"
	"github.com/kgellert/hodatay-messenger/internal/ws/hub"
)

type Handler struct {
	messagesRepo   messagesdomain.Repo
	uploadsService uploads.Service
	hub            *hub.Hub
	log            *slog.Logger
}

func New(
	messagesRepo messagesdomain.Repo,
	uploadsService uploads.Service,
	h *hub.Hub,
	log *slog.Logger,
) *Handler {
	return &Handler{messagesRepo: messagesRepo, uploadsService: uploadsService, hub: h, log: log}
}

func (h *Handler) GetMessages() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.messages.GetMessages"

		log := h.log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		chatIDStr := chi.URLParam(r, "chatId")
		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil || chatID <= 0 {
			log.Error("invalid chat_id", sl.Err(err))
			render.JSON(w, r, response.Error("invalid chat_id"))
			return
		}

		msgs, err := h.messagesRepo.GetMessages(r.Context(), chatID)
		if err != nil {
			log.Error("failed to get messages", sl.Err(err))
			render.JSON(w, r, response.Error("failed to get messages"))
			return
		}

		render.JSON(w, r, messagesdomain.GetMessagesResponse{
			Response: response.OK(),
			Messages: msgs,
		})
	}
}

func (h *Handler) SendMessage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.messages.SendMessage"

		log := h.log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		chatIDStr := chi.URLParam(r, "chatId")
		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil || chatID <= 0 {
			render.JSON(w, r, response.Error("invalid chat_id"))
			return
		}

		var req messagesdomain.CreateMessageRequest
		if err := render.DecodeJSON(r.Body, &req); err != nil {
			render.JSON(w, r, response.Error("invalid body"))
			return
		}

		if strings.TrimSpace(req.Text) == "" && len(req.Attachments) == 0 {
			render.JSON(w, r, response.Error("text or attachments is required"))
			return
		}

		userID := userhandlers.UserID(r)

		msg, err := h.messagesRepo.SendMessage(
			r.Context(),
			chatID,
			userID,
			req.Text,
			req.Attachments,
			req.ReplyToMessageID,
		)

		if err != nil {
			log.Error("failed to send message", sl.Err(err))
			render.JSON(w, r, response.Error("failed to add message"))
			return
		}

		render.JSON(w, r, messagesdomain.CreateMessageResponse{
			Response: response.OK(),
			Message:  *msg,
		})

		evt, err := ws.NewEvent(chatID, ws.MessageNew, ws.MessageNewPayload{Message: *msg})
		if err != nil {
			log.Error("failed to build ws event", sl.Err(err))
			return
		}

		payload, err := json.Marshal(evt)
		if err != nil {
			log.Error("failed to marshal ws event", sl.Err(err))
			return
		}

		h.hub.Broadcast(chatID, payload)
	}
}

func (h *Handler) SetLastReadMessage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.messages.SetLastReadMessage"

		log := h.log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		chatIDStr := chi.URLParam(r, "chatId")
		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil || chatID <= 0 {
			render.JSON(w, r, response.Error("invalid chatId"))
			return
		}

		var req messagesdomain.SetLastReadMessageRequest
		if err := render.DecodeJSON(r.Body, &req); err != nil {
			render.JSON(w, r, response.Error("invalid body"))
			return
		}
		if req.LastReadMessageID < 0 {
			render.JSON(w, r, response.Error("invalid last_read_message_id"))
			return
		}

		userID := userhandlers.UserID(r)

		savedLastRead, err := h.messagesRepo.SetLastReadMessage(r.Context(), chatID, userID, req.LastReadMessageID)
		if err != nil {
			log.Error("failed to set last read message", sl.Err(err))
			render.JSON(w, r, response.Error("failed to set last read message"))
			return
		}

		render.JSON(w, r, response.OK())

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

		h.hub.Broadcast(chatID, payload)
	}
}