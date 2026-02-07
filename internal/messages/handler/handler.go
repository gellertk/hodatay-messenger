package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/kgellert/hodatay-messenger/internal/logger/sl"
	"github.com/kgellert/hodatay-messenger/internal/messages"
	"github.com/kgellert/hodatay-messenger/internal/transport/httpapi"
	uploads "github.com/kgellert/hodatay-messenger/internal/uploads/domain"
	userhandlers "github.com/kgellert/hodatay-messenger/internal/users/handlers"
	"github.com/kgellert/hodatay-messenger/internal/ws"
	"github.com/kgellert/hodatay-messenger/internal/ws/hub"
)

type Handler struct {
	messagesRepo   messages.Repo
	uploadsService uploads.Service
	hub            *hub.Hub
	log            *slog.Logger
}

func New(
	messagesRepo messages.Repo,
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
			httpapi.WriteError(w, r, err)
			return
		}

		msgs, err := h.messagesRepo.GetMessages(r.Context(), chatID)
		if err != nil {
			log.Error("failed to get messages", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		render.JSON(w, r, messages.GetMessagesResponse{
			Messages: msgs,
		})
	}
}

func (h *Handler) SendMessage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.messages.send"

		log := h.log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		chatIDStr := chi.URLParam(r, "chatId")
		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil || chatID <= 0 {
			log.Error("invalid chat_id", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		var req messages.CreateMessageRequest
		if err := render.DecodeJSON(r.Body, &req); err != nil {
			log.Error("decode request error", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		if strings.TrimSpace(req.Text) == "" && len(req.Attachments) == 0 {
			httpapi.WriteError(w, r, messages.ErrTextOrAttachmentsIsRequired)
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
			httpapi.WriteError(w, r, err)
			return
		}

		if msg == nil {
			log.Error("failed to send message", sl.Err(messages.ErrMessageIsNil))
			httpapi.WriteError(w, r, err)
			return
		}

		render.JSON(w, r, messages.CreateMessageResponse{
			Message: *msg,
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
		const op = "handlers.messages.set.last_read"

		log := h.log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		chatIDStr := chi.URLParam(r, "chatId")
		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil || chatID <= 0 {
			log.Error("invalid chatId", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		var req messages.SetLastReadMessageRequest
		if err := render.DecodeJSON(r.Body, &req); err != nil {
			log.Error("failed to send message", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		if req.LastReadMessageID < 0 {
			log.Error(messages.ErrInvalidLastReadMessageId.Error(), sl.Err(err))
			httpapi.WriteError(w, r, messages.ErrInvalidLastReadMessageId)
			return
		}

		userID := userhandlers.UserID(r)

		savedLastRead, err := h.messagesRepo.SetLastReadMessage(r.Context(), chatID, userID, req.LastReadMessageID)
		if err != nil {
			log.Error("failed to set last read message", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		render.Status(r, http.StatusNoContent)

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

func (h *Handler) DeleteMessage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.messages.delete"

		log := h.log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		chatIDStr := chi.URLParam(r, "chatId")
		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil || chatID <= 0 {
			log.Error("invalid chat_id", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		messageIDStr := chi.URLParam(r, "messageId")
		messageID, err := strconv.ParseInt(messageIDStr, 10, 64)
		if err != nil || messageID <= 0 {
			log.Error("invalid messageId", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		err = h.messagesRepo.DeleteMessage(
			r.Context(),
			chatID,
			messageID,
		)

		if err != nil {
			log.Error("failed to delete message", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		render.Status(r, http.StatusNoContent)

		evt, err := ws.NewEvent(chatID, ws.MessagesDeleted, ws.MessagesDeletePayload{IDs: []int64{messageID}})

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

func (h *Handler) DeleteMessages() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.messages.delete.batch"

		log := h.log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		chatIDStr := chi.URLParam(r, "chatId")
		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil || chatID <= 0 {
			log.Error("invalid chat_id", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		var req messages.DeleteMessagesRequestResponse
		err = render.DecodeJSON(r.Body, &req)
		if err != nil {
			log.Error("invalid messageIDs", sl.Err(err))
			httpapi.WriteError(w, r, err)
		}

		deletedIDs, err := h.messagesRepo.DeleteMessages(
			r.Context(),
			chatID,
			req.MessageIDs,
		)

		if err != nil {
			log.Error("failed to delete messages", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		render.JSON(w, r, messages.DeleteMessagesRequestResponse{
			MessageIDs: deletedIDs,
		})

		evt, err := ws.NewEvent(chatID, ws.MessagesDeleted, ws.MessagesDeletePayload{IDs: deletedIDs})

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
