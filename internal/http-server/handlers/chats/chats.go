package chatsHandler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/kgellert/hodatay-messenger/internal/domain/chat"
	resp "github.com/kgellert/hodatay-messenger/internal/lib/api/response"
	"github.com/kgellert/hodatay-messenger/internal/lib/logger/sl"
	"github.com/kgellert/hodatay-messenger/internal/tempuser"
)

type GetChatsResponse struct {
	resp.Response
	Chats []chat.ChatListItem `json:"chats,omitempty"`
}

type GetChatResponse struct {
	resp.Response
	Chat chat.ChatInfo `json:"chat"`
}

type ChatsService interface {
    GetChats(ctx context.Context, userID int64) ([]chat.ChatListItem, error)
		GetChat(ctx context.Context, chatID int64) (chat.ChatInfo, error)
}

func GetChats(log *slog.Logger, chatsService ChatsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.chats.get.chats"

		log = log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		uid := tempuser.UserID(r)

		chats, err := chatsService.GetChats(r.Context(), uid)

		if err != nil { // обработать конкретные ошибки
			log.Error("Failed to get chats", sl.Err(err)) // Добавить кастомную обработку ошибки

			render.JSON(w, r, resp.Error("failed to get chats"))

			return
		}

		log.Info("Chats fetched", slog.Any("chats", chats))

		render.JSON(w, r, GetChatsResponse{
			Response: resp.OK(),
			Chats: chats,
		})
	}
}

func GetChat(log *slog.Logger, chatsService ChatsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.chats.get.chat"

		log = log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		chatIDStr := chi.URLParam(r, "chatId")
		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil || chatID <= 0 {
			log.Error("invalid chatID", sl.Err(err))
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, resp.Error("invalid chatId"))
			return
		}

		chatInfo, err := chatsService.GetChat(r.Context(), chatID)
		if err != nil {
			log.Error("failed to get chat", sl.Err(err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, resp.Error("failed to get chat"))
			return
		}

		render.JSON(w, r, GetChatResponse{
			Response: resp.OK(),
			Chat:     chatInfo,
		})
	}
}