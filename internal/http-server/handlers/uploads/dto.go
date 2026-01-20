package uploadsHandler

import "github.com/kgellert/hodatay-messenger/internal/lib/api/response"

type presignUploadRequest struct {
  Filename    string `json:"filename"`
  ContentType string `json:"content_type"`
}

type presignUploadResponse struct {
  Key       string `json:"key"`
  UploadURL string `json:"upload_url"`
}

type presignDownloadRequest struct {
  Key string `json:"key"`
}

type presignDownloadResponse struct {
  URL string `json:"url"`
}

type PresignUploadHTTPResponse struct {
  response.Response
  PresignUploadResponse presignUploadResponse `json:"presign_upload,omitempty"`
}

type PresignDownloadHTTPResponse struct {
  response.Response
  PresignDownloadResponse presignDownloadResponse `json:"presign_download,omitempty"`
}