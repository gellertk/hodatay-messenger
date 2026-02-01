package media

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
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

func WaveformU8FromS3FFmpeg(
	ctx context.Context,
	s3Client *s3.Client,
	bucket string,
	key string,
	points int,
) ([]byte, error) {
	if points <= 0 || points > 512 {
		return nil, errors.New("invalid points")
	}

	// 1) Скачиваем файл во временное место
	tmpDir := os.TempDir()
	tmpPath := filepath.Join(tmpDir, "audio_"+sanitizeKey(key))
	if err := downloadToFile(ctx, s3Client, bucket, key, tmpPath); err != nil {
		return nil, err
	}
	defer os.Remove(tmpPath)

	// 2) ffmpeg -> raw PCM float32 mono 16kHz в stdout
	//    -nostdin чтобы не зависнуть
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-nostdin",
		"-i", tmpPath,
		"-ac", "1",
		"-ar", "16000",
		"-f", "f32le",
		"pipe:1",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	peaks, err := peaksFromF32LE(stdout, points)

	// Дождаться завершения процесса обязательно
	waitErr := cmd.Wait()
	if err == nil && waitErr != nil {
		// ffmpeg мог упасть после того, как мы прочитали не всё
		err = errors.New(stderr.String())
	}
	if err != nil {
		if stderr.Len() > 0 {
			return nil, errors.New(stderr.String())
		}
		return nil, err
	}

	// peaks уже 0..1 float64
	u8 := quantizePeaksU8(peaks)
	return u8, nil
}

func peaksFromF32LE(r io.Reader, points int) ([]float64, error) {
	// Считаем весь поток float32 в память — для голосовых в чате это нормально.
	// Если у тебя могут быть большие файлы — скажи, сделаю streaming-версию без полного буфера.
	br := bufio.NewReaderSize(r, 128*1024)

	var samples []float32
	buf := make([]byte, 4*4096) // 4096 float32
	for {
		n, err := br.Read(buf)
		if n > 0 {
			// обрезаем до кратности 4
			n -= n % 4
			for i := 0; i < n; i += 4 {
				bits := binary.LittleEndian.Uint32(buf[i : i+4])
				f := math.Float32frombits(bits)
				samples = append(samples, f)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	if len(samples) == 0 {
		return make([]float64, points), nil
	}

	// Считаем peak по окнам
	peaks := make([]float64, points)
	total := len(samples)

	for i := 0; i < points; i++ {
		start := (i * total) / points
		end := ((i + 1) * total) / points
		if end <= start {
			end = min(start+1, total)
		}

		var peak float64
		for j := start; j < end; j++ {
			v := float64(samples[j])
			if v < 0 {
				v = -v
			}
			if v > peak {
				peak = v
			}
		}
		// peak float может быть >1 на некоторых декодерах — ограничим
		if peak > 1 {
			peak = 1
		}
		peaks[i] = peak
	}

	// Нормализация по 95 перцентилю (устойчиво к “хлопкам”)
	p95 := percentile(peaks, 0.95)
	if p95 <= 1e-9 {
		return peaks, nil
	}
	for i := range peaks {
		v := peaks[i] / p95
		if v > 1 {
			v = 1
		}
		// лёгкая “подсветка” тихих мест
		peaks[i] = math.Sqrt(v)
	}

	return peaks, nil
}

func quantizePeaksU8(peaks []float64) []byte {
	out := make([]byte, len(peaks))
	for i, v := range peaks {
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		out[i] = byte(math.Round(v * 255))
	}
	return out
}

func percentile(xs []float64, p float64) float64 {
	cp := make([]float64, len(xs))
	copy(cp, xs)
	sort.Float64s(cp)

	if len(cp) == 0 {
		return 0
	}
	if p <= 0 {
		return cp[0]
	}
	if p >= 1 {
		return cp[len(cp)-1]
	}

	pos := p * float64(len(cp)-1)
	i := int(math.Floor(pos))
	j := int(math.Ceil(pos))
	if i == j {
		return cp[i]
	}
	frac := pos - float64(i)
	return cp[i]*(1-frac) + cp[j]*frac
}

func downloadToFile(ctx context.Context, s3Client *s3.Client, bucket, key, path string) error {
	out, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	defer out.Body.Close()

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, out.Body)
	return err
}

func sanitizeKey(key string) string {
	// минимально безопасно для имени файла
	b := []byte(key)
	for i := range b {
		if b[i] == '/' || b[i] == '\\' {
			b[i] = '_'
		}
	}
	return string(b)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
