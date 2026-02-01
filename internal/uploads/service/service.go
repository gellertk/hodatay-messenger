package uploadsservice

import (
	"context"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/kgellert/hodatay-messenger/internal/config"
	media "github.com/kgellert/hodatay-messenger/internal/uploads"
	uploadsdomain "github.com/kgellert/hodatay-messenger/internal/uploads/domain"
)

func New(bucket string, presigner *s3.PresignClient, s3Client *s3.Client, repo uploadsdomain.Repo, presignConfig config.PresignTTLConfig) uploadsdomain.Service {
	return &service{bucket: bucket, presigner: presigner, s3Client: s3Client, repo: repo, config: presignConfig}
}

type service struct {
	bucket    string
	presigner *s3.PresignClient
	s3Client  *s3.Client
	repo      uploadsdomain.Repo
	config    config.PresignTTLConfig
}

func (s *service) PresignUpload(ctx context.Context, userID int64, contentType string, filename *string) (*uploadsdomain.PresignUploadInfo, error) {

	ttl := s.GetPresignTTL(contentType)

	key, err := uploadsdomain.GenerateKey()

	if err != nil {
		return nil, err
	}

	req := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}

	ps, err := s.presigner.PresignPutObject(ctx, req, func(po *s3.PresignOptions) {
		po.Expires = ttl
	})

	if err != nil {
		return nil, err
	}

	s.repo.CreateUpload(ctx, key, userID, contentType, filename)

	pInfo := uploadsdomain.PresignUploadInfo{
		FileID:    key,
		URL:       ps.URL,
		ExpiresIn: int(ttl.Seconds()),
	}

	return &pInfo, nil
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

	if strings.HasPrefix(contentType, "image/") {
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
			return s.repo.ConfirmUpload(ctx, userID, key, contentType, size, nil, nil, nil)
		}

		return s.repo.ConfirmUpload(ctx, userID, key, contentType, size, &config.Width, &config.Height, nil)
	} else if strings.HasPrefix(contentType, "audio/") {
		duration, err := media.DurationFromS3FFProbe(ctx, s.s3Client, s.bucket, key)
		if err != nil {
			s.repo.ConfirmUpload(ctx, userID, key, contentType, size, nil, nil, nil)
		} else {
			s.repo.ConfirmUpload(ctx, userID, key, contentType, size, nil, nil, (*int64)(&duration))
		}
	}

	return s.repo.ConfirmUpload(ctx, userID, key, contentType, size, nil, nil, nil)
}

func (s *service) GetPresignTTL(contentType string) time.Duration {
	var seconds int

	switch {
	case strings.HasPrefix(contentType, "audio/"):
		seconds = s.config.VoiceSec
	case strings.HasPrefix(contentType, "image/"):
		seconds = s.config.ImageSec
	case strings.HasPrefix(contentType, "video/"):
		seconds = s.config.VideoSec
	default:
		seconds = s.config.DocumentSec
	}

	return time.Duration(seconds) * time.Second
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
