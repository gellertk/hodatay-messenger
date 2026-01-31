package usersrepo

import (
	"context"
	"errors"

	"github.com/jmoiron/sqlx"
	userdomain "github.com/kgellert/hodatay-messenger/internal/users/domain"
)

var ErrUserNotFound = errors.New("user not found")

// Temporary in-memory storage until we have proper user table in DB
var users = []userdomain.User{
	{ID: 1, Name: "Роман Потапов", IsAdmin: true},
	{ID: 2, Name: "Иван Иванов", IsAdmin: false},
}

type Repo struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) GetUser(ctx context.Context, id int64) (userdomain.User, error) {
	for i := range users {
		if users[i].ID == id {
			return users[i], nil
		}
	}
	return userdomain.User{}, ErrUserNotFound
}

func (r *Repo) GetUsers(ctx context.Context, ids []int64) ([]userdomain.User, error) {
	usersByID := make(map[int64]userdomain.User, len(users))
	for _, u := range users {
		usersByID[u.ID] = u
	}

	result := make([]userdomain.User, 0, len(ids))
	for _, id := range ids {
		u, ok := usersByID[id]
		if !ok {
			return nil, ErrUserNotFound
		}
		result = append(result, u)
	}
	return result, nil
}
