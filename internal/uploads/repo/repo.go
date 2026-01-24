package uploadsrepo

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// type uploadsRepo interface {
// 	CreateUpload(
// 		ctx context.Context, key string, userID int64, filename, contentType *string,
// 	) error
// }

type Repo struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) CreateUpload(
	ctx context.Context, key string, userID int64, filename, contentType *string,
) error {

	_, err := r.db.ExecContext(
		ctx,
		`
		INSERT INTO uploads (key, owner_user_id, original_filename, client_content_type)
		VALUES ($1, $2, $3, $4)
		`,
		key, userID, filename, contentType,
	)

	return err
}
