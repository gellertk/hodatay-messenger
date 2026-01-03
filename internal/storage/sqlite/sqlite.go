package sqlite

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/jmoiron/sqlx"
	"github.com/kgellert/hodatay-messenger/internal/domain/chat"
	"github.com/kgellert/hodatay-messenger/internal/domain/message"
	"github.com/kgellert/hodatay-messenger/internal/domain/user"
	_ "github.com/mattn/go-sqlite3"
)

var (
	ErrEmptyParticipants = errors.New("no participants provided")
)

type Storage struct {
	db *sqlx.DB
}

type GetChatRow struct {
	ChatID int64   `db:"chat_id"`
	User user.User `db:"user"`
}

type ChatsRow struct {
	ChatID                     int64          	`db:"chat_id"`
	User                       user.User      	`db:"user"`
	LastMessage                message.Message 	`db:"last_message"`
	UnreadCount                int64          	`db:"unread_count"`
	OthersMinLastReadMessageID int64          	`db:"others_min_last_read_message_id"`
}

func New(storagePath string) (*Storage, error) {
	const op = "storage.sqlite.New"

	db, err := sqlx.Open("sqlite3", storagePath+"?_loc=auto")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	schemaBytes, err := os.ReadFile("internal/storage/sqlite/schema.sql")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("%s: read schema: %w", op, err)
	}
	schema := string(schemaBytes)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("%s: apply schema: %w", op, err)
	}

	return &Storage{db: db}, nil
}

func (s *Storage) AddChat(ctx context.Context, matterID int64) (int64, error) {
	const op = "storage.sqlite.AddChat"

	res, err := s.db.ExecContext(
		ctx,
		`INSERT INTO chats (matter_id) VALUES (?)`,
		matterID,
	)
	if err != nil {
		return 0, fmt.Errorf("%s: insert chat: %w", op, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("%s: get last insert id: %w", op, err)
	}

	return id, nil
}

func (s *Storage) AddChatParticipants(ctx context.Context, chatID int64, userIDs []int64) error {
	const op = "storage.sqlite.AddChatParticipants"

	if len(userIDs) == 0 {
		return ErrEmptyParticipants
	}

	uniquedUserIDs := uniquePositiveInts(userIDs)

	tx, err := s.db.BeginTx(ctx, nil)

	if err != nil {
		return fmt.Errorf("%s: failed to create transaction: %w", op, err)
	}

	defer tx.Rollback()

	stmt, err := tx.PrepareContext(
		ctx,
		`INSERT INTO chat_participants (chat_id, user_id) VALUES (?, ?)`,
	)

	if err != nil {
		return fmt.Errorf("%s: failed to prepare context: %w", op, err)
	}

	defer stmt.Close()

	for _, userID := range uniquedUserIDs {
		_, err := stmt.ExecContext(ctx, chatID, userID)

		if err != nil {
			return fmt.Errorf("%s: failed to exec context for userID: %d : %w", op, userID, err)
		}
	}

	err = tx.Commit()

	if err != nil {
		return fmt.Errorf("%s: failed commit: %w", op, err)
	}

	return nil
}

func (s *Storage) SendMessage(ctx context.Context, chatID, userID int64, text string) (message.Message, error) {
	const op = "storage.sqlite.SendMessage"

	var msg message.Message

	err := s.db.GetContext(
		ctx,
		&msg,
		`INSERT INTO
		messages (chat_id, sender_user_id, text)
		VALUES
		(?, ?, ?) RETURNING id,
		sender_user_id,
		text,
		created_at`,
		chatID,
		userID,
		text,
	)

	if err != nil {
		return message.Message{}, fmt.Errorf("%s: add message: %w", op, err)
	}

	return msg, nil
}

func (s *Storage) SetLastReadMessage(ctx context.Context, chatID, userID, lastReadMessageID int64) error {
	const op = "storage.sqlite.SetLastReadMessage"

	_, err := s.db.ExecContext(
		ctx, `
		UPDATE chat_participants
		SET
			last_read_message_id = CASE
				WHEN last_read_message_id < ? THEN ?
				ELSE last_read_message_id
			END
		WHERE
			chat_id = ?
			AND user_id = ?
	`,
		lastReadMessageID,
		lastReadMessageID,
		chatID,
		userID,
	)
	
	if err != nil {
		return fmt.Errorf("%s: set last read message: %w", op, err)
	}

	return nil
}

func (s *Storage) GetChats(ctx context.Context, userID int64) ([]chat.ChatListItem, error) {
	const op = "storage.sqlite.GetChats"

	rows, err := s.db.QueryxContext(
		ctx,
		`
		WITH
			my_participation AS (
				SELECT
					chat_id,
					user_id,
					last_read_message_id
				FROM chat_participants
				WHERE user_id = ?
			),

			-- последнее сообщение в каждом чате
			last_message AS (
				SELECT
					chat_id,
					id,
					sender_user_id,
					text,
					created_at
				FROM (
					SELECT
						m.chat_id,
						m.id,
						m.sender_user_id,
						m.text,
						m.created_at,
						ROW_NUMBER() OVER (
							PARTITION BY m.chat_id
							ORDER BY m.created_at DESC, m.id DESC
						) AS rn
					FROM messages m
				)
				WHERE rn = 1
			),

			-- непрочитанные для текущего пользователя
			unread_counts AS (
				SELECT
					mp.chat_id,
					COUNT(*) AS unread_count
				FROM messages m
				JOIN my_participation mp ON mp.chat_id = m.chat_id
				WHERE
					m.id > mp.last_read_message_id
					AND m.sender_user_id <> mp.user_id
				GROUP BY mp.chat_id
			),

			-- минимум last_read_message_id среди всех участников, кроме меня
			others_min_read AS (
				SELECT
					cp.chat_id,
					MIN(cp.last_read_message_id) AS others_min_last_read_message_id
				FROM chat_participants cp
				JOIN my_participation mp ON mp.chat_id = cp.chat_id
				WHERE cp.user_id <> mp.user_id
				GROUP BY cp.chat_id
			)

		SELECT
			cp.chat_id                                   AS "chat_id",
			cp.user_id                                   AS "user.id",
			u.name                                       AS "user.name",

			lm.id                                        AS "last_message.id",
			lm.sender_user_id                            AS "last_message.sender_user_id",
			COALESCE(lm.text, '')                        AS "last_message.text",
			lm.created_at                                AS "last_message.created_at",

			COALESCE(uc.unread_count, 0)                 AS "unread_count",
			COALESCE(om.others_min_last_read_message_id, 0)
			                                             AS "others_min_last_read_message_id"

		FROM chat_participants cp
		JOIN users u ON u.id = cp.user_id
		JOIN my_participation mp ON mp.chat_id = cp.chat_id
		LEFT JOIN last_message lm ON lm.chat_id = cp.chat_id
		LEFT JOIN unread_counts uc ON uc.chat_id = cp.chat_id
		LEFT JOIN others_min_read om ON om.chat_id = cp.chat_id

		-- сортировка как в мессенджерах: по времени последнего сообщения (пустые чаты вниз)
		ORDER BY
			CASE WHEN lm.created_at IS NULL THEN 1 ELSE 0 END,
			lm.created_at DESC,
			lm.id DESC,
			cp.chat_id,
			cp.user_id
		`,
		userID,
	)

	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", op, err)
	}
	defer rows.Close()

	var (
		chats                      []chat.ChatListItem
		currentUsers               []user.User
		lastChatID                 int64
		lastMessage                message.Message
		unreadCount                int64
		othersMinLastReadMessageID int64
		hasLast                    bool
	)

	for rows.Next() {
		var row ChatsRow
		if err := rows.StructScan(&row); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", op, err)
		}

		if !hasLast {
			hasLast = true
			lastChatID = row.ChatID

			unreadCount = row.UnreadCount
			othersMinLastReadMessageID = row.OthersMinLastReadMessageID
			lastMessage = row.LastMessage
		}

		if row.ChatID != lastChatID {
			chats = append(chats, chat.ChatListItem{
				ID:                        lastChatID,
				Users:                     slices.Clone(currentUsers),
				LastMessage:               lastMessage,
				UnreadCount:               unreadCount,
				OthersMinLastReadMessageID: othersMinLastReadMessageID,
			})

			currentUsers = currentUsers[:0]
			lastChatID = row.ChatID

			unreadCount = row.UnreadCount
			othersMinLastReadMessageID = row.OthersMinLastReadMessageID
			lastMessage = row.LastMessage
		}

		currentUsers = append(currentUsers, row.User)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", op, err)
	}

	if hasLast {
		chats = append(chats, chat.ChatListItem{
			ID:                        lastChatID,
			Users:                     slices.Clone(currentUsers),
			LastMessage:               lastMessage,
			UnreadCount:               unreadCount,
			OthersMinLastReadMessageID: othersMinLastReadMessageID,
		})
	}

	return chats, nil
}

func (s *Storage) GetChat(ctx context.Context, chatID int64) (chat.ChatInfo, error) {
	const op = "storage.sqlite.GetChat"

	chatRows, err := s.db.QueryxContext(
		ctx,
		`
		SELECT
			cp.chat_id,
			cp.user_id as "user.id",
			u.name as "user.name"
		FROM
			chat_participants cp LEFT JOIN users u ON cp.user_id = u.id
		WHERE
			chat_id = ?
		 `,
		chatID,
	)

	if err != nil {
		return chat.ChatInfo{}, fmt.Errorf("%s: get chatRows error: %w", op, err)
	}

	defer chatRows.Close()

	var users []user.User

	for chatRows.Next() {
		var chatRow ChatsRow
		if err := chatRows.StructScan(&chatRow); err != nil {
			return chat.ChatInfo{}, err
		}
		users = append(users, chatRow.User)
	}

	if err := chatRows.Err(); err != nil {
		return chat.ChatInfo{}, fmt.Errorf("%s: rows error: %w", op, err)
	}

	chat := chat.ChatInfo{ID: chatID, Users: users}

	return chat, nil
}

func (s *Storage) GetMessages(ctx context.Context, chatID int64) ([]message.Message, error) {
	const op = "storage.sqlite.GetMessages"

	messageRows, err := s.db.QueryxContext(
		ctx,
		`
			SELECT
				id,
				sender_user_id,
				text,
				created_at
			FROM
				messages
			WHERE
				chat_id = ?
		 `,
		chatID,
	)

	if err != nil {
		return nil, fmt.Errorf("%s: get messageRows error: %w", op, err)
	}

	defer messageRows.Close()

	var messages []message.Message

	for messageRows.Next() {
		var message message.Message
		if err := messageRows.StructScan(&message); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}

	if err := messageRows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows error: %w", op, err)
	}

	return messages, nil
}

func uniquePositiveInts(input []int64) []int64 {
	seen := make(map[int64]bool)
	result := []int64{}

	for _, v := range input {
		if v <= 0 {
			continue
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		result = append(result, v)
	}
	return result
}
