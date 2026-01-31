package userdomain

import (
	"context"

	response "github.com/kgellert/hodatay-messenger/internal/lib"
)

type User struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	IsAdmin bool   `json:"is_admin"`
}

type SignInResponse struct {
	response.Response
	User User `json:"user"`
}

type Repo interface {
	GetUser(ctx context.Context, id int64) (User, error)
	GetUsers(ctx context.Context, ids []int64) ([]User, error)
}
