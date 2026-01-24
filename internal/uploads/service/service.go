package uploadsservice

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	uploadsdomain "github.com/kgellert/hodatay-messenger/internal/uploads/domain"
)

func New(bucket string, presigner *s3.PresignClient, s3Client *s3.Client, repo uploadsdomain.Repo) uploadsdomain.Service {
	return &service{bucket: bucket, presigner: presigner, s3Client: s3Client, repo: repo}
}

type service struct {
	bucket    string
	presigner *s3.PresignClient
	s3Client  *s3.Client
	repo      uploadsdomain.Repo
}

func (s *service) PresignUpload(ctx context.Context, userID int64, filename, contentType *string) (string, string, error) {

	key, err := uploadsdomain.GenerateKey()

	if err != nil {
		return "", "", err
	}

	req := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	if contentType != nil && *contentType != "" {
		req.ContentType = contentType
	}

	ps, err := s.presigner.PresignPutObject(ctx, req, func(po *s3.PresignOptions) {
		po.Expires = 15 * time.Minute
	})

	if err != nil {
		return "", "", err
	}

	s.repo.CreateUpload(ctx, key, userID, filename, contentType)

	return key, ps.URL, nil
}

func (s *service) PresignDownload(ctx context.Context, key string) (string, error) {

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

func (s *service) GetFileInfo(ctx context.Context, key string) (uploadsdomain.Attachment, error) {
	headObj, err := s.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return uploadsdomain.Attachment{}, err
	}

	filename := key
	if originName, ok := headObj.Metadata["original-filename"]; ok {
		filename = originName
	}

	contentType := ""
	if headObj.ContentType != nil {
		contentType = *headObj.ContentType
	}

	return uploadsdomain.Attachment{FileID: key, ContentType: contentType, Filename: filename}, nil
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
