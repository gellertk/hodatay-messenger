package hub

import (
	"sync"

	"github.com/gorilla/websocket"
)

type Connection struct {
	conn      *websocket.Conn
	send      chan []byte
	chatIDs   map[int64]struct{}
	userID    int64
	closeOnce sync.Once
}

func (c *Connection) UserID() int64 { return c.userID }

type SubscribeCmd struct {
	c       *Connection
	chatIDs []int64
}

type BroadcastCmd struct {
	ChatID      int64
	Payload     []byte
	ExcludeUser int64
}

type Hub struct {
	register   chan *Connection
	unregister chan *Connection
	subscribe  chan SubscribeCmd
	broadcast  chan BroadcastCmd
	chats      map[int64]map[*Connection]struct{}
}

func NewConnection(conn *websocket.Conn, userID int64) *Connection {
	return &Connection{
		conn:    conn,
		send:    make(chan []byte, 128),
		chatIDs: make(map[int64]struct{}),
		userID:  userID,
	}
}

func NewHub() *Hub {
	return &Hub{
		register:   make(chan *Connection, 64),
		unregister: make(chan *Connection, 64),
		subscribe:  make(chan SubscribeCmd, 64),
		broadcast:  make(chan BroadcastCmd, 256),
		chats:      make(map[int64]map[*Connection]struct{}),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			_ = c

		case c := <-h.unregister:
			for chatID := range c.chatIDs {
				room := h.chats[chatID]
				if room == nil {
					continue
				}
				delete(room, c)
				if len(room) == 0 {
					delete(h.chats, chatID)
				}
			}
			c.CloseSend()

		case cmd := <-h.subscribe:
			for _, chatID := range cmd.chatIDs {
				room := h.chats[chatID]
				if room == nil {
					room = make(map[*Connection]struct{})
					h.chats[chatID] = room
				}
				room[cmd.c] = struct{}{}
				cmd.c.chatIDs[chatID] = struct{}{}
			}

		case b := <-h.broadcast:
			room := h.chats[b.ChatID]
			if room == nil {
				continue
			}

			for c := range room {
				if b.ExcludeUser != 0 && c.userID == b.ExcludeUser {
					continue
				}
				c.Send(b.Payload)
			}
		}
	}
}

func (h *Hub) Register(c *Connection) {
	h.register <- c
}

func (h *Hub) Unregister(c *Connection) {
	h.unregister <- c
}

func (h *Hub) Subscribe(c *Connection, chatIDs []int64) {
	h.subscribe <- SubscribeCmd{
		c:       c,
		chatIDs: chatIDs,
	}
}

func (h *Hub) Broadcast(chatID int64, payload []byte) {
	h.broadcast <- BroadcastCmd{
		ChatID:  chatID,
		Payload: payload,
	}
}

func (h *Hub) BroadcastExceptUser(chatID int64, payload []byte, excludeUserID int64) {
	h.broadcast <- BroadcastCmd{
		ChatID:      chatID,
		Payload:     payload,
		ExcludeUser: excludeUserID,
	}
}

func (c *Connection) Send(b []byte) {
	select {
	case c.send <- b:
	default:
	}
}

func (c *Connection) CloseSend() {
	c.closeOnce.Do(func() {
		close(c.send)
	})
}
