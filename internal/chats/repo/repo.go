package chatsrepo

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jmoiron/sqlx"
	chatsdomain "github.com/kgellert/hodatay-messenger/internal/chats"
	messagesdomain "github.com/kgellert/hodatay-messenger/internal/messages"
	uploadsdomain "github.com/kgellert/hodatay-messenger/internal/uploads/domain"
	userdomain "github.com/kgellert/hodatay-messenger/internal/users/domain"
	"github.com/lib/pq"
)

var (
	ErrEmptyParticipants = errors.New("no participants provided")
	ErrChatsNotFound     = errors.New("chats not found")
)

type Repo struct {
	db        *sqlx.DB
	usersRepo userdomain.Repo
}

func New(db *sqlx.DB, usersRepo userdomain.Repo) *Repo {
	return &Repo{db: db, usersRepo: usersRepo}
}

func (s *Repo) CreateChat(ctx context.Context, userIDs []int64) (*chatsdomain.ChatInfo, error) {
	const op = "storage.postgres.CreateChat"

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: begin tx: %w", op, err)
	}
	defer tx.Rollback()

	var chatID int64
	err = tx.QueryRowxContext(
		ctx,
		`INSERT INTO chats (matter_id) VALUES ($1)
		RETURNING id`,
		1,
	).Scan(&chatID)

	if err != nil {
		return nil, fmt.Errorf("%s: insert chat: %w", op, err)
	}

	users, err := s.addChatParticipants(ctx, tx, chatID, userIDs)
	if err != nil {
		return nil, fmt.Errorf("%s: add participants: %w", op, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("%s: commit tx: %w", op, err)
	}

	chatInfo := &chatsdomain.ChatInfo{
		ID:    chatID,
		Users: users,
	}

	return chatInfo, nil
}

func (s *Repo) AddChatParticipants(ctx context.Context, chatID int64, userIDs []int64) ([]userdomain.User, error) {
	return s.addChatParticipants(ctx, s.db, chatID, userIDs)
}

func (s *Repo) addChatParticipants(
	ctx context.Context,
	q sqlx.ExtContext,
	chatID int64,
	userIDs []int64,
) ([]userdomain.User, error) {

	const op = "storage.postgres.AddChatParticipants"

	if len(userIDs) == 0 {
		return nil, ErrEmptyParticipants
	}

	userIDs = uniquePositiveInts(userIDs)

	query := `
			INSERT INTO chat_participants (chat_id, user_id)
			VALUES ($1, $2)
			ON CONFLICT (chat_id, user_id) DO NOTHING
    `

	users := make([]userdomain.User, 0, len(userIDs))

	for _, userID := range userIDs {
		if _, err := q.ExecContext(ctx, query, chatID, userID); err != nil {
			return nil, fmt.Errorf("%s: insert user %d: %w", op, userID, err)
		}

		u, err := s.usersRepo.GetUser(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("%s: get user %d: %w", op, userID, err)
		}

		users = append(users, u)
	}

	return users, nil
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

func containsAttachment(attachments []uploadsdomain.AttachmentRow, att uploadsdomain.AttachmentRow) bool {
	for _, a := range attachments {
		if a.FileID.String == att.FileID.String {
			return true
		}
	}
	return false
}

func containsUser(users []userdomain.User, user userdomain.User) bool {
	for _, u := range users {
		if u.ID == user.ID {
			return true
		}
	}
	return false
}

func (s *Repo) GetChats(ctx context.Context, userID int64) ([]chatsdomain.ChatListItem, error) {
	const op = "storage.postgres.GetChats"

	rows, err := s.db.QueryxContext(
		ctx,
		`
		WITH my_participation AS (SELECT chat_id,
                                 user_id,
                                 CASE
                                     WHEN last_read_message_id IS NULL OR last_read_message_id < 0 THEN 0
                                     ELSE last_read_message_id
                                     END AS last_read_message_id
                          FROM chat_participants
                          WHERE user_id = $1),

     last_message AS (SELECT chat_id,
                             id,
                             sender_user_id,
                             text,
                             created_at
                      FROM (SELECT m.chat_id,
                                   m.id,
                                   m.sender_user_id,
                                   m.text,
                                   m.created_at,
                                   ROW_NUMBER() OVER (
                                       PARTITION BY m.chat_id
                                       ORDER BY m.created_at DESC, m.id DESC
                                       ) AS rn
                            FROM messages m)
                      WHERE rn = 1),

     unread_counts AS (SELECT mp.chat_id,
                              COUNT(*) AS unread_count
                       FROM messages m
                                JOIN my_participation mp ON mp.chat_id = m.chat_id
                       WHERE m.id > mp.last_read_message_id
                         AND m.sender_user_id <> mp.user_id
                       GROUP BY mp.chat_id),

     others_max_read AS (SELECT cp.chat_id,
                                COALESCE(MAX(
                                                 CASE
                                                     WHEN cp.last_read_message_id IS NULL OR cp.last_read_message_id < 0
                                                         THEN 0
                                                     ELSE cp.last_read_message_id
                                                     END
                                         ), 0) AS others_max_last_read_message_id
                         FROM chat_participants cp
                                  JOIN my_participation mp ON mp.chat_id = cp.chat_id
                         WHERE cp.user_id <> mp.user_id
                         GROUP BY cp.chat_id)

SELECT cp.chat_id                                         AS "chat_id",
       cp.user_id                                         AS "user_id",

       COALESCE(lm.id, 0)                                 AS "last_message.id",
       COALESCE(lm.sender_user_id, 0)                     AS "last_message.sender_user_id",
       COALESCE(lm.text, '')                              AS "last_message.text",
       COALESCE(lm.created_at, '1970-01-01'::timestamptz) AS "last_message.created_at",

       COALESCE(att.file_id, '')                          AS "last_message.attachment.file_id",
       COALESCE(att.content_type, '')                     AS "last_message.attachment.content_type",
       COALESCE(att.filename, '')                         AS "last_message.attachment.filename",
       COALESCE(att.size, 0)                              AS "last_message.attachment.size",
       COALESCE(att.width, 0)                             AS "last_message.attachment.width",
       COALESCE(att.height, 0)                            AS "last_message.attachment.height",

       COALESCE(uc.unread_count, 0)                       AS "unread_count",
       COALESCE(om.others_max_last_read_message_id, 0)    AS "others_max_last_read_message_id"

FROM chat_participants cp
         JOIN my_participation mp ON mp.chat_id = cp.chat_id
         LEFT JOIN last_message lm ON lm.chat_id = cp.chat_id
         LEFT JOIN unread_counts uc ON uc.chat_id = cp.chat_id
         LEFT JOIN others_max_read om ON om.chat_id = cp.chat_id
         LEFT JOIN attachments att ON att.message_id = lm.id

ORDER BY CASE WHEN lm.created_at IS NULL THEN 1 ELSE 0 END,
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

	chats := []chatsdomain.ChatListItem{}

	var (
		currentUsers                  []userdomain.User
		lastMessageRow                messagesdomain.MessageRow
		lastMessageAttachments        []uploadsdomain.AttachmentRow
		lastMessageReplyToAttachments []uploadsdomain.AttachmentRow
		lastChatID                    int64
		unreadCount                   int64
		othersMaxLastReadMessageID    int64
		hasLast                       bool
	)

	for rows.Next() {
		var row chatsdomain.ChatRow
		if err := rows.StructScan(&row); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", op, err)
		}

		if !hasLast {
			hasLast = true
			lastChatID = row.ChatID
			unreadCount = row.UnreadCount
			othersMaxLastReadMessageID = row.OthersMaxLastReadMessageID
			lastMessageRow = row.LastMessage
		}

		if row.ChatID != lastChatID {
			lm := messagesdomain.NewMessageFromRow(
				row.LastMessage,
				slices.Clone(lastMessageAttachments),
				slices.Clone(lastMessageReplyToAttachments),
			)
			chats = append(chats, chatsdomain.ChatListItem{
				ID:                         lastChatID,
				Users:                      slices.Clone(currentUsers),
				LastMessage:                lm,
				UnreadCount:                unreadCount,
				OthersMaxLastReadMessageID: othersMaxLastReadMessageID,
			})

			currentUsers = currentUsers[:0]
			lastMessageAttachments = lastMessageAttachments[:0]
			lastMessageReplyToAttachments = lastMessageReplyToAttachments[:0]
			lastChatID = row.ChatID
			unreadCount = row.UnreadCount
			othersMaxLastReadMessageID = row.OthersMaxLastReadMessageID
			lastMessageRow = row.LastMessage
		}
		user, err := s.usersRepo.GetUser(ctx, row.UserID)
		if err != nil {
			return nil, fmt.Errorf("%s: get user: %w", op, err)
		}
		if !containsUser(currentUsers, user) {
			currentUsers = append(currentUsers, user)
		}

		// Добавляем attachment только если он валидный и ещё не добавлен
		if row.LastMessage.Attachment.FileID.Valid {
			if !containsAttachment(lastMessageAttachments, row.LastMessage.Attachment) {
				lastMessageAttachments = append(lastMessageAttachments, row.LastMessage.Attachment)
			}
		}
		if row.LastMessage.ReplyToAttachment.FileID.Valid {
			if !containsAttachment(lastMessageReplyToAttachments, row.LastMessage.ReplyToAttachment) {
				lastMessageReplyToAttachments = append(lastMessageReplyToAttachments, row.LastMessage.ReplyToAttachment)
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", op, err)
	}

	if hasLast {
		lm := messagesdomain.NewMessageFromRow(
			lastMessageRow,
			slices.Clone(lastMessageAttachments),
			slices.Clone(lastMessageReplyToAttachments),
		)
		chats = append(chats, chatsdomain.ChatListItem{
			ID:                         lastChatID,
			Users:                      slices.Clone(currentUsers),
			LastMessage:                lm,
			UnreadCount:                unreadCount,
			OthersMaxLastReadMessageID: othersMaxLastReadMessageID,
		})
	}

	return chats, nil
}

func (s *Repo) GetChat(ctx context.Context, chatID int64) (*chatsdomain.ChatInfo, error) {
	const op = "storage.postgres.GetChat"

	rows, err := s.db.QueryContext(
		ctx,
		`
		SELECT chat_id, user_id
		FROM chat_participants
		WHERE chat_id = $1
		`,
		chatID,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: query error: %w", op, err)
	}
	defer rows.Close()

	var userIDs []int64
	var foundChatID int64

	for rows.Next() {
		var cID, uID int64
		if err := rows.Scan(&cID, &uID); err != nil {
			return nil, fmt.Errorf("%s: scan error: %w", op, err)
		}
		foundChatID = cID
		userIDs = append(userIDs, uID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows error: %w", op, err)
	}

	if len(userIDs) == 0 {
		return nil, fmt.Errorf("%s: chat not found", op)
	}

	users, err := s.usersRepo.GetUsers(ctx, userIDs)
	if err != nil {
		return nil, fmt.Errorf("%s: get users error: %w", op, err)
	}

	return &chatsdomain.ChatInfo{
		ID:    foundChatID,
		Users: users,
	}, nil
}

func (s *Repo) GetUnreadMessagesCount(ctx context.Context, userID int64) (int, error) {

	const op = "storage.postgres.GetUnreadMessagesCount"

	var unreadCount int
	err := s.db.QueryRowxContext(
		ctx,
		`SELECT COUNT(*) AS unreadCount
			FROM chat_participants cp
		JOIN messages m
		ON m.chat_id = cp.chat_id
		WHERE cp.user_id = $1
		AND m.sender_user_id <> $1
		AND m.id > COALESCE(cp.last_read_message_id, 0)
		`,
		userID,
	).Scan(&unreadCount)

	if err != nil {
		return 0, err
	}

	return unreadCount, nil
}

func (s *Repo) DeleteChats(ctx context.Context, chatIDs []int64) ([]int64, error) {

	const op = "storage.postgres.DeleteChats"

	deletedChatIds := []int64{}
	err := s.db.SelectContext(
		ctx,
		&deletedChatIds,
		`
		DELETE FROM chats 
		WHERE id = ANY($1)
		RETURNING id
		`,
		pq.Array(chatIDs),
	)

	if err != nil {
		return []int64{}, err
	}

	if len(deletedChatIds) == 0 {
		return []int64{}, ErrChatsNotFound
	}

	return deletedChatIds, nil
}
