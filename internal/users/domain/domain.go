package userdomain

import "context"

type User struct {
	ID   int64  `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

type Repo interface {
	GetUser(ctx context.Context, id int64) (User, error)
	GetUsers(ctx context.Context, ids []int64) ([]User, error)
}