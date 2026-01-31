package messagesrepo

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	messagesdomain "github.com/kgellert/hodatay-messenger/internal/messages"
	uploadsdomain "github.com/kgellert/hodatay-messenger/internal/uploads/domain"
)

type Repo struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) *Repo {
	return &Repo{db: db}
}

func (s *Repo) SendMessage(
	ctx context.Context,
	chatID,
	userID int64,
	text string,
	attachments []messagesdomain.CreateMessageAttachment,
	replyToMessageID *int64,
) (*messagesdomain.Message, error) {

	const op = "storage.postgres.SendMessage"

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: begin tx: %w", op, err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryxContext(
		ctx,
		`
		WITH inserted AS (
			INSERT INTO messages (chat_id, sender_user_id, text, reply_to_message_id)
			VALUES ($1, $2, $3, $4)
			RETURNING id, chat_id, sender_user_id, text, created_at, reply_to_message_id
		)
		SELECT
			i.id,
			i.sender_user_id,
			i.text,
			i.created_at,

			rm.id AS "reply_to.id",
			rm.sender_user_id AS "reply_to.sender_user_id",
			rm.text AS "reply_to.text",
			rm.created_at AS "reply_to.created_at",

			ra.id AS "reply_to.attachment.id",
			ra.file_id AS "reply_to.attachment.file_id",
			ra.content_type AS "reply_to.attachment.content_type",
			ra.filename AS "reply_to.attachment.filename",
			ra.size AS "reply_to.attachment.size",
			ra.width AS "reply_to.attachment.width",
			ra.height AS "reply_to.attachment.height"

		FROM inserted i
		LEFT JOIN messages rm ON i.reply_to_message_id = rm.id
		LEFT JOIN attachments ra ON ra.message_id = rm.id
		`,
		chatID, userID, text, replyToMessageID,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: query message: %w", op, err)
	}
	defer rows.Close()

	var resultRows []messagesdomain.MessageRow
	for rows.Next() {
		var r messagesdomain.MessageRow
		if err := rows.StructScan(&r); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", op, err)
		}
		resultRows = append(resultRows, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows error: %w", op, err)
	}

	if len(resultRows) == 0 {
		return nil, fmt.Errorf("%s: no rows returned", op)
	}

	firstRow := resultRows[0]
	msg := messagesdomain.Message{
		ID:           firstRow.ID,
		SenderUserID: firstRow.SenderUserID,
		Text:         firstRow.Text,
		CreatedAt:    firstRow.CreatedAt,
		Attachments:  []uploadsdomain.Attachment{},
	}

	if firstRow.ReplyTo.ID.Valid {
		replyTo := &messagesdomain.Message{
			ID:           firstRow.ReplyTo.ID.Int64,
			SenderUserID: firstRow.ReplyTo.SenderUserID.Int64,
			Text:         firstRow.ReplyTo.Text.String,
			CreatedAt:    firstRow.ReplyTo.CreatedAt.Time,
			Attachments:  []uploadsdomain.Attachment{},
		}

		seenAttIDs := map[int64]struct{}{}
		for _, r := range resultRows {
			if r.ReplyToAttachment.ID.Valid {
				aid := r.ReplyToAttachment.ID.Int64
				if _, seen := seenAttIDs[aid]; !seen {
					seenAttIDs[aid] = struct{}{}
					replyTo.Attachments = append(replyTo.Attachments, uploadsdomain.Attachment{
						FileID:      r.ReplyToAttachment.FileID.String,
						ContentType: r.ReplyToAttachment.ContentType.String,
						Filename:    r.ReplyToAttachment.Filename.String,
					})
				}
			}
		}

		msg.ReplyTo = replyTo
	}

	atts := []uploadsdomain.Attachment{}

	for _, att := range attachments {

		var upload struct {
			Size        int64  `db:"size"`
			Width       *int   `db:"width"`
			Height      *int   `db:"height"`
			ContentType string `db:"content_type"`
			Filename    string `db:"original_filename"`
			Status      string `db:"status"`
		}

		err := tx.GetContext(
			ctx,
			&upload,
			`
			SELECT size, width, height, content_type, original_filename, status 
			FROM uploads
			WHERE file_id = $1 AND owner_user_id = $2
			`,
			att.FileID,
			userID,
		)

		if err != nil {
			return nil, fmt.Errorf("%s: select upload: %w", op, err)
		}

		if upload.Status != string(uploadsdomain.StatusReady) {
			return nil, fmt.Errorf("%s: upload is not confirmed: %w", op, err)
		}

		var attachment uploadsdomain.Attachment

		err = tx.QueryRowxContext(
			ctx,
			`INSERT INTO attachments (message_id, file_id, content_type, filename, size, width, height)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING file_id, content_type, filename, size, width, height
			`,
			msg.ID, att.FileID, upload.ContentType, upload.Filename, upload.Size, upload.Width, upload.Height,
		).Scan(
			&attachment.FileID,
			&attachment.ContentType,
			&attachment.Filename,
			&attachment.Size,
			&attachment.Width,
			&attachment.Height,
		)

		if err != nil {
			return nil, fmt.Errorf("%s: insert attachment: %w", op, err)
		}

		atts = append(atts, attachment)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("%s: commit tx: %w", op, err)
	}

	msg.Attachments = atts

	return &msg, nil
}

func (s *Repo) SetLastReadMessage(ctx context.Context, chatID, userID, lastReadMessageID int64) (int64, error) {
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
		WHERE chat_id = $1 AND sender_user_id != $2
	`, chatID, userID); err != nil {
		return 0, fmt.Errorf("%s: select max: %w", op, err)
	}

	saved := max(min(lastReadMessageID, maxID), 0)

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

func (s *Repo) GetMessages(ctx context.Context, chatID int64) ([]messagesdomain.Message, error) {
	const op = "storage.postgres.GetMessages"

	rows, err := s.db.QueryxContext(ctx, `
		SELECT 
			m.id, m.sender_user_id, m.text, m.created_at,

			rm.id AS "reply_to.id",
			rm.sender_user_id AS "reply_to.sender_user_id",
			rm.text AS "reply_to.text",
			rm.created_at AS "reply_to.created_at",

			a.id AS "attachment.id",
			a.file_id AS "attachment.file_id",
			a.content_type AS "attachment.content_type",
			a.filename AS "attachment.filename",
			a.size AS "attachment.size",
			a.width AS "attachment.width",
			a.height AS "attachment.height",

			ra.id AS "reply_to.attachment.id",
			ra.file_id AS "reply_to.attachment.file_id",
			ra.content_type AS "reply_to.attachment.content_type",
			ra.filename AS "reply_to.attachment.filename",
			ra.size AS "reply_to.attachment.size",
			ra.width AS "reply_to.attachment.width",
			ra.height AS "reply_to.attachment.height"
		FROM messages m
		LEFT JOIN messages rm ON m.reply_to_message_id = rm.id
		LEFT JOIN attachments a ON a.message_id = m.id
		LEFT JOIN attachments ra ON ra.message_id = rm.id
		WHERE m.chat_id = $1
		ORDER BY m.created_at ASC
	`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messagesByID := map[int64]*messagesdomain.Message{}
	order := make([]int64, 0)

	seenMsgAtt := map[int64]map[int64]struct{}{}
	seenReplyAtt := map[int64]map[int64]struct{}{}

	for rows.Next() {
		var r messagesdomain.MessageRow
		if err := rows.StructScan(&r); err != nil {
			return nil, err
		}

		m := messagesByID[r.ID]
		if m == nil {
			m = &messagesdomain.Message{
				ID:           r.ID,
				SenderUserID: r.SenderUserID,
				Text:         r.Text,
				CreatedAt:    r.CreatedAt,
				Attachments:  []uploadsdomain.Attachment{},
			}
			messagesByID[r.ID] = m
			order = append(order, r.ID)
		}

		if r.Attachment.ID.Valid {
			if seenMsgAtt[r.ID] == nil {
				seenMsgAtt[r.ID] = map[int64]struct{}{}
			}
			aid := r.Attachment.ID.Int64
			var width, height *int
			if r.Attachment.Width.Valid {
				w := int(r.Attachment.Width.Int32)
				width = &w
			}
			if r.Attachment.Height.Valid {
				h := int(r.Attachment.Height.Int32)
				height = &h
			}
			if _, ok := seenMsgAtt[r.ID][aid]; !ok {
				seenMsgAtt[r.ID][aid] = struct{}{}
				m.Attachments = append(m.Attachments, uploadsdomain.Attachment{
					FileID:      r.Attachment.FileID.String,
					ContentType: r.Attachment.ContentType.String,
					Filename:    r.Attachment.Filename.String,
					Size:        r.Attachment.Size.Int64,
					Width:       width,
					Height:      height,
				})
			}
		}

		if r.ReplyTo.ID.Valid {
			if m.ReplyTo == nil {
				m.ReplyTo = &messagesdomain.Message{
					ID:           r.ReplyTo.ID.Int64,
					SenderUserID: r.ReplyTo.SenderUserID.Int64,
					Text:         r.ReplyTo.Text.String,
					CreatedAt:    r.ReplyTo.CreatedAt.Time,
					Attachments:  []uploadsdomain.Attachment{},
				}
			}

			if r.ReplyToAttachment.ID.Valid {
				if seenReplyAtt[r.ID] == nil {
					seenReplyAtt[r.ID] = map[int64]struct{}{}
				}
				raid := r.ReplyToAttachment.ID.Int64
				if _, ok := seenReplyAtt[r.ID][raid]; !ok {
					seenReplyAtt[r.ID][raid] = struct{}{}
					m.ReplyTo.Attachments = append(m.ReplyTo.Attachments, uploadsdomain.Attachment{
						FileID:      r.ReplyToAttachment.FileID.String,
						ContentType: r.ReplyToAttachment.ContentType.String,
						Filename:    r.ReplyToAttachment.Filename.String,
					})
				}
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// финальный slice в исходном порядке
	out := make([]messagesdomain.Message, 0, len(order))
	for _, id := range order {
		out = append(out, *messagesByID[id])
	}
	return out, nil
}
