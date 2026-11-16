package Web

import (
	"mime"
	"testing"
)

func TestAttachmentDispositionQuotesCommaFileNames(t *testing.T) {
	header := attachmentDisposition("HardcodedKey,AuthBypass,CryptoFlaw-0.md")
	mediaType, params, err := mime.ParseMediaType(header)
	if err != nil {
		t.Fatalf("parse content-disposition: %v", err)
	}
	if mediaType != "attachment" {
		t.Fatalf("unexpected media type: %s", mediaType)
	}
	if got := params["filename"]; got != "HardcodedKey,AuthBypass,CryptoFlaw-0.md" {
		t.Fatalf("unexpected filename: %q", got)
	}
}

func TestAttachmentDispositionSupportsUnicodeFileNames(t *testing.T) {
	header := attachmentDisposition("二进制分析报告,最终版.md")
	mediaType, params, err := mime.ParseMediaType(header)
	if err != nil {
		t.Fatalf("parse content-disposition: %v", err)
	}
	if mediaType != "attachment" {
		t.Fatalf("unexpected media type: %s", mediaType)
	}
	if got := params["filename"]; got != "二进制分析报告,最终版.md" {
		t.Fatalf("unexpected filename: %q", got)
	}
}

func TestSanitizeAttachmentFileNameDropsPathAndHeaderBreaks(t *testing.T) {
	got := sanitizeAttachmentFileName("nested/path/HardcodedKey,AuthBypass\r\n.md")
	if got != "HardcodedKey,AuthBypass.md" {
		t.Fatalf("unexpected sanitized filename: %q", got)
	}
}
