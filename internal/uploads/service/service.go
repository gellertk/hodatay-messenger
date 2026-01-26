package uploadsservice

import (
	"context"
	"errors"
	"image"
	"strings"
	"time"
	_ "image/jpeg"
	_ "image/png"

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

func (s *service) PresignUpload(ctx context.Context, userID int64, contentType string, filename *string) (string, string, error) {

	key, err := uploadsdomain.GenerateKey()

	if err != nil {
		return "", "", err
	}

	req := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		ContentType: aws.String(contentType),
	}

	ps, err := s.presigner.PresignPutObject(ctx, req, func(po *s3.PresignOptions) {
		po.Expires = 15 * time.Minute
	})

	if err != nil {
		return "", "", err
	}

	s.repo.CreateUpload(ctx, key, userID, contentType, filename)

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

func (s *service) ConfirmUpload(ctx context.Context, userID int64, key string) error {

	err := validateKey(key)

	if err != nil {
		return err
	}

	headObjc, err := s.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return err
	}

	contentType := ""
	if headObjc.ContentType != nil {
		contentType = *headObjc.ContentType
	}

	var size int64
	if headObjc.ContentLength != nil {
		size = *headObjc.ContentLength
	}

	if !strings.HasPrefix(contentType, "image/") {
		return s.repo.ConfirmUpload(ctx, userID, key, contentType, size, nil, nil)
	}

	result, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Range:  aws.String("bytes=0-65535"),
	})

	if err != nil {
		return err
	}

	defer result.Body.Close()

	config, _, err := image.DecodeConfig(result.Body)
	if err != nil {
		return s.repo.ConfirmUpload(ctx, userID, key, contentType, size, nil, nil)
	}

	return s.repo.ConfirmUpload(ctx, userID, key, contentType, size, &config.Width, &config.Height)
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
