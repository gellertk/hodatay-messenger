package uploadsdomain

var allowedContentTypes = map[string]string{
	"image/jpeg":     ".jpg",
	"image/png":      ".png",
	"image/webp":     ".webp",
	"image/heic":     ".heic",
	"image/heif":     ".heif",
	"image/avif":     ".avif",
	"image/tiff":     ".tiff",
	"image/bmp":      ".bmp",
	"image/x-ms-bmp": ".bmp",

	"application/pdf":    ".pdf",
	"application/msword": ".doc",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": ".docx",
	"application/vnd.ms-excel": ".xls",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         ".xlsx",
	"application/vnd.ms-powerpoint":                                             ".ppt",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",

	"application/zip": ".zip",

	"audio/mpeg": ".mp3",
	"audio/ogg":  ".ogg",
	"audio/webm": ".webm",
	"audio/wav":  ".wav",

	"video/mp4":  ".mp4",
	"video/webm": ".webm",
}

func IsValidContentType(ct string) bool {
	_, exist := allowedContentTypes[ct]
	return exist
}
