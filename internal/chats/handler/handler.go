package chatshandler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	chatsdomain "github.com/kgellert/hodatay-messenger/internal/chats"
	response "github.com/kgellert/hodatay-messenger/internal/lib"
	"github.com/kgellert/hodatay-messenger/internal/logger/sl"
	userhandlers "github.com/kgellert/hodatay-messenger/internal/users/handlers"
)

type Handler struct {
	service chatsdomain.ChatsService
	log     *slog.Logger
}

func New(
	service chatsdomain.ChatsService,
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

		chats, err := h.service.GetChats(r.Context(), uid)

		if err != nil { // обработать конкретные ошибки
			log.Error("Failed to get chats", sl.Err(err)) // Добавить кастомную обработку ошибки

			render.JSON(w, r, response.Error("failed to get chats"))

			return
		}

		log.Info("Chats fetched", slog.Any("chats", chats))

		render.JSON(w, r, chatsdomain.GetChatsResponse{
			Response: response.OK(),
			Chats:    chats,
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
			log.Error("invalid chatID", sl.Err(err))
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("invalid chatId"))
			return
		}

		chatInfo, err := h.service.GetChat(r.Context(), chatID)
		if err != nil {
			log.Error("failed to get chat", sl.Err(err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, response.Error("failed to get chat"))
			return
		}

		render.JSON(w, r, chatsdomain.GetChatResponse{
			Response: response.OK(),
			Chat:     chatInfo,
		})
	}
}
