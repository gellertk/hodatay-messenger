package chatsrepo

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jmoiron/sqlx"
	chatsdomain "github.com/kgellert/hodatay-messenger/internal/chats"
	messagesdomain "github.com/kgellert/hodatay-messenger/internal/messages"
	uploads "github.com/kgellert/hodatay-messenger/internal/uploads/domain"
	userdomain "github.com/kgellert/hodatay-messenger/internal/users/domain"
)

var (
	ErrEmptyParticipants = errors.New("no participants provided")
)

type Repo struct {
	db        *sqlx.DB
	usersRepo userdomain.Repo
}

func New(db *sqlx.DB, usersRepo userdomain.Repo) *Repo {
	return &Repo{db: db, usersRepo: usersRepo}
}

func (s *Repo) AddChat(ctx context.Context, matterID int64) (int64, error) {
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

func (s *Repo) AddChatParticipants(ctx context.Context, chatID int64, userIDs []int64) error {
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

func (s *Repo) GetChats(ctx context.Context, userID int64) ([]chatsdomain.ChatListItem, error) {
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
			cp.user_id                                   AS "user_id",

			lm.id                                        AS "last_message.id",
			lm.sender_user_id                            AS "last_message.sender_user_id",
			COALESCE(lm.text, '')                        AS "last_message.text",
			lm.created_at                                AS "last_message.created_at",

			COALESCE(uc.unread_count, 0)                 AS "unread_count",
			COALESCE(om.others_max_last_read_message_id, 0) AS "others_max_last_read_message_id"

		FROM chat_participants cp
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
		chats                      []chatsdomain.ChatListItem
		currentUsers               []userdomain.User
		lastChatID                 int64
		lastMessage                messagesdomain.Message
		unreadCount                int64
		othersMaxLastReadMessageID int64
		hasLast                    bool
	)

	for rows.Next() {
		var row chatsdomain.ChatsRow
		if err := rows.StructScan(&row); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", op, err)
		}

		if row.LastMessage.Attachments == nil {
			row.LastMessage.Attachments = []uploads.Attachment{}
		}

		if !hasLast {
			hasLast = true
			lastChatID = row.ChatID
			unreadCount = row.UnreadCount
			othersMaxLastReadMessageID = row.OthersMaxLastReadMessageID
			lastMessage = row.LastMessage
		}

		if row.ChatID != lastChatID {
			chats = append(chats, chatsdomain.ChatListItem{
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
		user, err := s.usersRepo.GetUser(ctx, row.UserID)
		if err != nil {
			return nil, fmt.Errorf("%s: get user: %w", op, err)
		}
		currentUsers = append(currentUsers, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", op, err)
	}

	if hasLast {
		chats = append(chats, chatsdomain.ChatListItem{
			ID:                         lastChatID,
			Users:                      slices.Clone(currentUsers),
			LastMessage:                lastMessage,
			UnreadCount:                unreadCount,
			OthersMaxLastReadMessageID: othersMaxLastReadMessageID,
		})
	}

	return chats, nil
}

func (s *Repo) GetChat(ctx context.Context, chatID int64) (chatsdomain.ChatInfo, error) {
	const op = "storage.postgres.GetChat"

	chatRows, err := s.db.QueryxContext(
		ctx,
		`
		SELECT
			cp.chat_id,
			cp.user_id,
		FROM
			chat_participants
		WHERE
			chat_id = $1
			`,
		chatID,
	)

	if err != nil {
		return chatsdomain.ChatInfo{}, fmt.Errorf("%s: get chatRows error: %w", op, err)
	}

	defer chatRows.Close()

	var users []userdomain.User

	for chatRows.Next() {
		var chatRow chatsdomain.ChatsRow
		if err := chatRows.StructScan(&chatRow); err != nil {
			return chatsdomain.ChatInfo{}, err
		}
		user, err := s.usersRepo.GetUser(ctx, chatRow.UserID)
		if err != nil {
			return chatsdomain.ChatInfo{}, fmt.Errorf("%s: get user: %w", op, err)
		}
		users = append(users, user)
	}

	if err := chatRows.Err(); err != nil {
		return chatsdomain.ChatInfo{}, fmt.Errorf("%s: rows error: %w", op, err)
	}

	chat := chatsdomain.ChatInfo{ID: chatID, Users: users}

	return chat, nil
}
