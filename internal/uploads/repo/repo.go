package uploadsrepo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
	uploadsdomain "github.com/kgellert/hodatay-messenger/internal/uploads/domain"
)

type Repo struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) CreateUpload(
	ctx context.Context, fileID string, userID int64, contentType string, filename *string,
) error {

	_, err := r.db.ExecContext(
		ctx,
		`
		INSERT INTO uploads (file_id, owner_user_id, original_filename, client_content_type)
		VALUES ($1, $2, $3, $4)
		`,
		fileID, userID, filename, contentType,
	)

	return err
}

func (r *Repo) ConfirmUpload(
	ctx context.Context,
	userID int64,
	key string,
	contentType string,
	size int64,
	width, height *int,
	duration *time.Duration,
	waveform []byte,
) error {

	var durationMs sql.NullInt64
	if duration != nil {
		durationMs = sql.NullInt64{
			Int64: duration.Milliseconds(),
			Valid: true,
		}
	}

	result, err := r.db.ExecContext(
		ctx,
		`
        UPDATE uploads
        SET content_type = $1,
            size = $2,
            width = $3,
            height = $4,
            status = $5,
            duration_ms = $6,
						waveform_u8 = $7
        WHERE file_id = $8 AND owner_user_id = $9
        `,
		contentType,
		size,
		width,
		height,
		uploadsdomain.StatusReady,
		durationMs,
		waveform,
		key,
		userID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New("upload not found or access denied")
	}

	return nil
}
