package uploads

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/kgellert/hodatay-messenger/internal/domain/message"
)

func NewService(bucket string, presigner *s3.PresignClient, s3Client *s3.Client) UploadsService {
	return &Storage{bucket: bucket, presigner: presigner, s3Client: s3Client}
}

type Storage struct {
	bucket    string
	presigner *s3.PresignClient
	s3Client  *s3.Client
}

func (s *Storage) PresignUpload(ctx context.Context, filename, contentType string) (string, string, error) {

	key, err := GenerateKey(filename, contentType)

	if err != nil {
		return "", "", err
	}

	req := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
		Metadata: map[string]string{
			"original-filename": filename,
		},
	}

	ps, err := s.presigner.PresignPutObject(ctx, req, func(po *s3.PresignOptions) {
		po.Expires = 15 * time.Minute
	})

	if err != nil {
		return "", "", err
	}

	return key, ps.URL, nil
}

func (s *Storage) PresignDownload(ctx context.Context, key string) (string, error) {

	err := validateKey(key)

	if err != nil {
		return "", err
	}

	req := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	ps, err := s.presigner.PresignGetObject(ctx, req, func(po *s3.PresignOptions) {
		po.Expires = 15 * time.Minute
	})

	if err != nil {
		return "", err
	}

	return ps.URL, nil
}

func (s *Storage) GetFileInfo(ctx context.Context, key string) (message.Attachment, error) {
	headObj, err := s.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return message.Attachment{}, err
	}

	filename := key
	if originName, ok := headObj.Metadata["original-filename"]; ok {
		filename = originName
	}

	contentType := ""
	if headObj.ContentType != nil {
		contentType = *headObj.ContentType
	}

	return message.Attachment{FileID: key, ContentType: contentType, Filename: filename}, nil
}

func validateKey(key string) error {
	if key == "" {
		return errors.New("invalid key")
	}
	if !strings.HasPrefix(key, "uploads/") {
		return errors.New("invalid key")
	}
	if strings.Contains(key, "..") {
		return errors.New("invalid key")
	}
	return nil
}
