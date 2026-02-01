package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// DurationFromS3FFProbe скачивает аудиофайл из S3 во временный файл и получает duration через ffprobe.
// Работает с большинством форматов (ogg/opus, mp3, m4a/aac, wav, webm и т.д.).
func DurationFromS3FFProbe(
	ctx context.Context,
	s3c *s3.Client,
	bucket, key string,
) (time.Duration, error) {
	// 1) GetObject
	obj, err := s3c.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return 0, fmt.Errorf("s3 get object: %w", err)
	}
	defer obj.Body.Close()

	// 2) Temp file
	tmp, err := os.CreateTemp("", "voice-*")
	if err != nil {
		return 0, fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()

	if _, err := io.Copy(tmp, obj.Body); err != nil {
		return 0, fmt.Errorf("copy to temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return 0, fmt.Errorf("sync temp: %w", err)
	}

	// 3) ffprobe JSON
	// Берём duration формата (в секундах строкой). Это самый совместимый вариант.
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "json",
		tmpPath,
	)

	out, err := cmd.Output()
	if err != nil {
		// Если ffprobe отсутствует или файл не распознаётся — будет ошибка тут.
		var ee *exec.Error
		if errors.As(err, &ee) {
			return 0, fmt.Errorf("ffprobe not available: %w", err)
		}
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	// 4) Parse
	type ffprobeOut struct {
		Format struct {
			Duration string `json:"duration"` // seconds, e.g. "8.342000"
		} `json:"format"`
	}

	var parsed ffprobeOut
	if err := json.Unmarshal(out, &parsed); err != nil {
		return 0, fmt.Errorf("parse ffprobe json: %w", err)
	}

	if parsed.Format.Duration == "" {
		return 0, fmt.Errorf("ffprobe: empty duration")
	}

	secs, err := strconv.ParseFloat(parsed.Format.Duration, 64)
	if err != nil {
		return 0, fmt.Errorf("parse duration float: %w", err)
	}
	if secs < 0 {
		return 0, fmt.Errorf("ffprobe: negative duration")
	}

	// Convert to time.Duration with ms precision
	d := time.Duration(secs * float64(time.Second))
	return d, nil
}
