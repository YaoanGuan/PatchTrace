package Web

import (
	"mime"
	"path"
	"strings"
)

func attachmentDisposition(fileName string) string {
	safeName := sanitizeAttachmentFileName(fileName)
	disposition := mime.FormatMediaType("attachment", map[string]string{
		"filename": safeName,
	})
	if disposition == "" {
		return `attachment; filename="download"`
	}
	return disposition
}

func sanitizeAttachmentFileName(fileName string) string {
	fileName = strings.TrimSpace(fileName)
	fileName = strings.ReplaceAll(fileName, "\\", "/")
	fileName = path.Base(fileName)
	if fileName == "." || fileName == "/" {
		fileName = ""
	}
	if fileName == "" {
		return "download"
	}
	var b strings.Builder
	for _, r := range fileName {
		if r == 0 || r == '\r' || r == '\n' {
			continue
		}
		b.WriteRune(r)
	}
	fileName = strings.TrimSpace(b.String())
	if fileName == "" {
		return "download"
	}
	return fileName
}
