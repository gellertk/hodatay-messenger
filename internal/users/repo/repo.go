package usersrepo

import (
	"context"

	"github.com/jmoiron/sqlx"
	userdomain "github.com/kgellert/hodatay-messenger/internal/users/domain"
)

// Temporary in-memory storage until we have proper user table in DB
var users = map[int64]string{
	1: "Роман Потапов",
	2: "Иван Иванов",
}

type Repo struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) GetUser(ctx context.Context, id int64) (userdomain.User, error) {
	// TODO: Replace with actual DB query when users table is ready
	// For now using in-memory map
	name := users[id]
	return userdomain.User{ID: id, Name: name}, nil
}

func (r *Repo) GetUsers(ctx context.Context, ids []int64) ([]userdomain.User, error) {
	// TODO: Replace with actual DB query when users table is ready
	result := make([]userdomain.User, 0, len(ids))
	for _, id := range ids {
		name := users[id]
		result = append(result, userdomain.User{ID: id, Name: name})
	}
	return result, nil
}
