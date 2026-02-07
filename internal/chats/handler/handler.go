package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/kgellert/hodatay-messenger/internal/chats"
	"github.com/kgellert/hodatay-messenger/internal/logger/sl"
	"github.com/kgellert/hodatay-messenger/internal/transport/httpapi"
	userhandlers "github.com/kgellert/hodatay-messenger/internal/users/handlers"
)

type Handler struct {
	service chats.ChatsService
	log     *slog.Logger
}

func New(
	service chats.ChatsService,
	log *slog.Logger,
) *Handler {
	return &Handler{service: service, log: log}
}

func (h *Handler) GetChats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.chats.get.chats"

		log := h.log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		uid := userhandlers.UserID(r)

		chatList, err := h.service.GetChats(r.Context(), uid)

		if err != nil {
			log.Error("Failed to get chats", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		log.Info("Chats fetched", slog.Any("chats", chatList))

		render.JSON(w, r, chats.GetChatsResponse{
			Chats: chatList,
		})
	}
}

func (h *Handler) CreateChat() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.chats.create.chat"

		log := h.log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var req chats.CreateChatRequest
		if err := render.DecodeJSON(r.Body, &req); err != nil {
			log.Error("failed to decode request", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		chatInfo, err := h.service.CreateChat(r.Context(), req.UserIDs)
		if err != nil {
			log.Error("failed to create chat", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		if chatInfo == nil {
			log.Error("chatInfo is nil", sl.Err(chats.ErrEmptyParticipants))
			httpapi.WriteError(w, r, err)
			return
		}

		log.Info("Chat created", slog.Any("chat", chatInfo))

		render.Status(r, http.StatusCreated)
		render.JSON(w, r, chats.GetChatResponse{
			Chat: *chatInfo,
		})
	}
}

func (h *Handler) DeleteChat() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.chats.delete.chat"

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

		err = h.service.DeleteChat(r.Context(), chatID)
		if err != nil {
			log.Error("failed to delete chat", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		render.Status(r, http.StatusNoContent)
	}
}

func (h *Handler) DeleteChats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.chats.delete.chats"

		log := h.log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var req chats.DeleteChatsRequest
		if err := render.DecodeJSON(r.Body, &req); err != nil {
			log.Error("failed to decode request", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		chatIDs, err := h.service.DeleteChats(r.Context(), req.ChatIDs)
		if err != nil {
			log.Error("failed to delete chats", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		render.JSON(w, r, chats.DeleteChatsResponse{
			ChatIDs: chatIDs,
		})
	}
}

func (h *Handler) GetChat() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.chats.get.chat"

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

		chatInfo, err := h.service.GetChat(r.Context(), chatID)
		if err != nil {
			log.Error("failed to get chat", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		if chatInfo == nil {
			log.Error("chat info is nil", sl.Err(chats.ErrChatIsNil))
			httpapi.WriteError(w, r, err)
			return
		}

		render.JSON(w, r, chats.GetChatResponse{
			Chat: *chatInfo,
		})
	}
}

func (h *Handler) GetUnreadMessagesCount() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.chats.get.unread_count"

		log := h.log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		userID := userhandlers.UserID(r)

		unreadCount, err := h.service.GetUnreadMessagesCount(r.Context(), userID)

		if err != nil {
			log.Error("failed to get unread count", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		render.JSON(w, r, chats.GetUnreadMessagesCountResponse{
			UnreadCount: unreadCount,
		})
	}
}
