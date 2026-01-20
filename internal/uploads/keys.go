package uploads

import (
	"errors"
	"path/filepath"

	"github.com/google/uuid"
)

func GenerateKey(filename, contentType string) (string, error) {
	ext, ok := ExtForMime(contentType)
	if !ok {
		return "", errors.New("unsupported content type")
	}

	if filename != "" {
		fExt := filepath.Ext(filename)
		if fExt != "" && fExt != ext {
			return "", errors.New("file extension does not match content type")
		}
	}

	u, err := uuid.NewV7()
	if err != nil {
		return "", err
	}

	return "uploads/" + u.String() + ext, nil
}