package uploads

var allowedMimeTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",

	"application/pdf": ".pdf",
	"application/msword": ".doc",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": ".docx",
	"application/vnd.ms-excel": ".xls",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": ".xlsx",
	"application/vnd.ms-powerpoint": ".ppt",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",

	"application/zip": ".zip",

	"audio/mpeg": ".mp3",
	"audio/ogg":  ".ogg",
	"audio/webm": ".webm",
	"audio/wav":  ".wav",

	"video/mp4":  ".mp4",
	"video/webm": ".webm",
}

func ExtForMime(m string) (string, bool) {
  ext, ok := allowedMimeTypes[m]
  return ext, ok
}