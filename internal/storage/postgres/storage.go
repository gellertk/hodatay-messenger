package storage

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/kgellert/hodatay-messenger/internal/domain/chat"
	"github.com/kgellert/hodatay-messenger/internal/domain/message"
	"github.com/kgellert/hodatay-messenger/internal/domain/user"
)

var (
	ErrEmptyParticipants = errors.New("no participants provided")
)

type Storage struct {
	db *sqlx.DB
}

type GetChatRow struct {
	ChatID int64     `db:"chat_id"`
	User   user.User `db:"user"`
}

type ChatsRow struct {
	ChatID                     int64           `db:"chat_id"`
	User                       user.User       `db:"user"`
	LastMessage                message.Message `db:"last_message"`
	UnreadCount                int64           `db:"unread_count"`
	OthersMaxLastReadMessageID int64           `db:"others_max_last_read_message_id"`
}

func New(ctx context.Context, dsn string) (*Storage, error) {
	const op = "storage.postgres.New"

	db, err := sqlx.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("%s: open: %w", op, err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("%s: ping: %w", op, err)
	}

	return &Storage{db: db}, nil
}

func (s *Storage) AddChat(ctx context.Context, matterID int64) (int64, error) {
	const op = "storage.postgres.AddChat"

	var id int64
	err := s.db.QueryRowContext(
		ctx,
		`INSERT INTO chats (matter_id) VALUES ($1) RETURNING id`,
		matterID,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("%s: insert chat: %w", op, err)
	}

	return id, nil
}

func (s *Storage) AddChatParticipants(ctx context.Context, chatID int64, userIDs []int64) error {
	const op = "storage.postgres.AddChatParticipants"

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
		`INSERT INTO chat_participants (chat_id, user_id) VALUES ($1, $2)`,
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

func (s *Storage) SendMessage(
	ctx context.Context, chatID, userID int64, text string, attachments []message.Attachment,
) (message.Message, error) {

	const op = "storage.postgres.SendMessage"

	var msg message.Message

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return message.Message{}, fmt.Errorf("%s: begin tx: %w", op, err)
	}
	defer tx.Rollback()

	err = tx.QueryRowxContext(
		ctx,
		`INSERT INTO messages (chat_id, sender_user_id, text)
		VALUES ($1, $2, $3)
		RETURNING id, sender_user_id, text, created_at`,
		chatID, userID, text,
	).StructScan(&msg)

	if err != nil {
		return message.Message{}, fmt.Errorf("%s: insert message: %w", op, err)
	}

	for _, att := range attachments {
		_, err := tx.ExecContext(
			ctx,
			`INSERT INTO attachments (message_id, key, content_type, filename)
			VALUES ($1, $2, $3, $4)`,
			msg.ID, att.FileID, att.ContentType, att.Filename,
		)
		if err != nil {
			return message.Message{}, fmt.Errorf("%s: insert attachment: %w", op, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return message.Message{}, fmt.Errorf("%s: commit tx: %w", op, err)
	}

	msg.Attachments = attachments

	return msg, nil
}

func (s *Storage) SetLastReadMessage(ctx context.Context, chatID, userID, lastReadMessageID int64) (int64, error) {
	const op = "storage.postgres.SetLastReadMessage"

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("%s: begin: %w", op, err)
	}
	defer func() { _ = tx.Rollback() }()

	var maxID int64
	if err := tx.GetContext(ctx, &maxID, `
		SELECT COALESCE(MAX(id), 0)
		FROM messages
		WHERE chat_id = $1
	`, chatID); err != nil {
		return 0, fmt.Errorf("%s: select max: %w", op, err)
	}

	saved := lastReadMessageID
	if saved > maxID {
		saved = maxID
	}
	if saved < 0 {
		saved = 0
	}

	res, err := tx.ExecContext(ctx, `
	UPDATE chat_participants
	SET last_read_message_id = GREATEST(COALESCE(last_read_message_id, 0), $1)
	WHERE chat_id = $2 AND user_id = $3
	`, saved, chatID, userID)

	if err != nil {
		return 0, fmt.Errorf("%s: update: %w", op, err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		return 0, fmt.Errorf("%s: participant not found (chat_id=%d user_id=%d)", op, chatID, userID)
	}

	if err := tx.GetContext(ctx, &saved, `
		SELECT CASE
			WHEN last_read_message_id IS NULL OR last_read_message_id < 0 THEN 0
			ELSE last_read_message_id
		END
		FROM chat_participants
		WHERE chat_id = $1 AND user_id = $2
	`, chatID, userID); err != nil {
		return 0, fmt.Errorf("%s: select self last_read: %w", op, err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("%s: commit: %w", op, err)
	}

	return saved, nil
}

func (s *Storage) GetChats(ctx context.Context, userID int64) ([]chat.ChatListItem, error) {
	const op = "storage.postgres.GetChats"

	rows, err := s.db.QueryxContext(
		ctx,
		`
		WITH
			my_participation AS (
				SELECT
					chat_id,
					user_id,
					CASE
						WHEN last_read_message_id IS NULL OR last_read_message_id < 0 THEN 0
						ELSE last_read_message_id
					END AS last_read_message_id
				FROM chat_participants
				WHERE user_id = $1
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

			-- Telegram-like: максимум last_read_message_id среди всех участников, кроме меня
			others_max_read AS (
				SELECT
					cp.chat_id,
					COALESCE(MAX(
						CASE
							WHEN cp.last_read_message_id IS NULL OR cp.last_read_message_id < 0 THEN 0
							ELSE cp.last_read_message_id
						END
					), 0) AS others_max_last_read_message_id
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
			COALESCE(om.others_max_last_read_message_id, 0)
			                                             AS "others_max_last_read_message_id"

		FROM chat_participants cp
		JOIN users u ON u.id = cp.user_id
		JOIN my_participation mp ON mp.chat_id = cp.chat_id
		LEFT JOIN last_message lm ON lm.chat_id = cp.chat_id
		LEFT JOIN unread_counts uc ON uc.chat_id = cp.chat_id
		LEFT JOIN others_max_read om ON om.chat_id = cp.chat_id

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
		othersMaxLastReadMessageID int64
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
			othersMaxLastReadMessageID = row.OthersMaxLastReadMessageID
			lastMessage = row.LastMessage
		}

		if row.ChatID != lastChatID {
			chats = append(chats, chat.ChatListItem{
				ID:                         lastChatID,
				Users:                      slices.Clone(currentUsers),
				LastMessage:                lastMessage,
				UnreadCount:                unreadCount,
				OthersMaxLastReadMessageID: othersMaxLastReadMessageID,
			})

			currentUsers = currentUsers[:0]
			lastChatID = row.ChatID
			unreadCount = row.UnreadCount
			othersMaxLastReadMessageID = row.OthersMaxLastReadMessageID
			lastMessage = row.LastMessage
		}

		currentUsers = append(currentUsers, row.User)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", op, err)
	}

	if hasLast {
		chats = append(chats, chat.ChatListItem{
			ID:                         lastChatID,
			Users:                      slices.Clone(currentUsers),
			LastMessage:                lastMessage,
			UnreadCount:                unreadCount,
			OthersMaxLastReadMessageID: othersMaxLastReadMessageID,
		})
	}

	return chats, nil
}

func (s *Storage) GetChat(ctx context.Context, chatID int64) (chat.ChatInfo, error) {
	const op = "storage.postgres.GetChat"

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
			chat_id = $1
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
	const op = "storage.postgres.GetMessages"

	type MessageRow struct {
		ID           int64     `db:"id"`
		SenderUserID int64     `db:"sender_user_id"`
		Text         string    `db:"text"`
		CreatedAt    time.Time `db:"created_at"`
	}

	var rows []MessageRow
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT id, sender_user_id, text, created_at
		FROM messages
		WHERE chat_id = $1
		ORDER BY created_at ASC, id ASC
	`, chatID); err != nil {
		return nil, fmt.Errorf("%s: select messages: %w", op, err)
	}

	if len(rows) == 0 {
		return []message.Message{}, nil
	}

	ids := make([]int64, 0, len(rows))
	attachmentsByMessage := make(map[int64][]message.Attachment, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
		attachmentsByMessage[r.ID] = []message.Attachment{} // гарантируем []
	}

	type AttachmentRow struct {
		MessageID   int64  `db:"message_id"`
		Key         string `db:"key"`
		ContentType string `db:"content_type"`
		Filename    string `db:"filename"`
	}

	q, args, err := sqlx.In(`
		SELECT message_id, key, content_type, filename
		FROM attachments
		WHERE message_id IN (?)
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("%s: sqlx.In: %w", op, err)
	}
	q = s.db.Rebind(q)

	var arows []AttachmentRow
	if err := s.db.SelectContext(ctx, &arows, q, args...); err != nil {
		return nil, fmt.Errorf("%s: select attachments: %w", op, err)
	}

	for _, a := range arows {
		attachmentsByMessage[a.MessageID] = append(attachmentsByMessage[a.MessageID], message.Attachment{
			FileID:      a.Key,
			ContentType: a.ContentType,
			Filename:    a.Filename,
		})
	}

	result := make([]message.Message, 0, len(rows))
	for _, r := range rows {
		result = append(result, message.Message{
			ID:           r.ID,
			SenderUserID: r.SenderUserID,
			Text:         r.Text,
			CreatedAt:    r.CreatedAt,
			Attachments:  attachmentsByMessage[r.ID], // всегда []
		})
	}

	return result, nil
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
