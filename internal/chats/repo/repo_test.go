package repo

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/kgellert/hodatay-messenger/internal/users"
)

func TestRepo_addChatParticipants(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		db        *sqlx.DB
		usersRepo users.Repo
		// Named input parameters for target function.
		q       sqlx.ExtContext
		chatID  int64
		userIDs []int64
		want    []users.User
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(tt.db, tt.usersRepo)
			got, gotErr := s.addChatParticipants(context.Background(), tt.q, tt.chatID, tt.userIDs)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("addChatParticipants() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("addChatParticipants() succeeded unexpectedly")
			}
			// TODO: update the condition below to compare got with tt.want.
			if true {
				t.Errorf("addChatParticipants() = %v, want %v", got, tt.want)
			}
		})
	}
}
