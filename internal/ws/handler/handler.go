package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
	"github.com/kgellert/hodatay-messenger/internal/lib/logger/sl"
	"github.com/kgellert/hodatay-messenger/internal/tempuser"
	"github.com/kgellert/hodatay-messenger/internal/ws/hub"
)

type ClientMsg struct {
	Type    string  `json:"type"`
	ChatIDs []int64 `json:"chat_ids"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func WSHandler(h *hub.Hub, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		const op = "handlers.messages.WSHandler"

		log = log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Error("ws upgrade error", sl.Err(err))
			return
		}
		defer conn.Close()

		userID := tempuser.UserID(r)

		hc := hub.NewConnection(conn, userID)
		go hc.WritePump()

		h.Register(hc)
		defer h.Unregister(hc)
			_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			conn.SetPongHandler(func(string) error {
			_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})

		hello, _ := json.Marshal(map[string]any{"type": "hello", "ok": true})
		hc.Send(hello)

		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				log.Error("ws read error", sl.Err(err))
				return
			}

			var msg ClientMsg
			if err := json.Unmarshal(data, &msg); err != nil {
				log.Error("ws bad json", sl.Err(err))
				continue
			}

			switch msg.Type {
			case "subscribe":
				h.Subscribe(hc, msg.ChatIDs)
			default:
				log.Info("ws unknown message type", slog.String("message type", msg.Type))
			}
		}
	}
}