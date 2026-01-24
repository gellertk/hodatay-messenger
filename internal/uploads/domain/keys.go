package uploadsdomain

import (
	"github.com/google/uuid"
)

func GenerateKey() (string, error) {
	u, err := uuid.NewV7()
	if err != nil {
		return "", err
	}

	return "uploads/" + u.String(), nil
}
