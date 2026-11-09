// Package storage holds the slide rules of PRD §6.8 that run before any object
// is written: SlideValidator, and later the MinIO port and ExpectedFilename.
package storage

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"

	"golang.org/x/text/unicode/norm"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// MaxSlideBytes is the exact size limit of one slide file (5 MiB).
const MaxSlideBytes int64 = 5 * 1024 * 1024

// Kind is the accepted slide type.
type Kind string

const (
	PDF  Kind = "pdf"
	PPTX Kind = "pptx"
)

// Part is one file part named "slides" of the multipart request.
type Part struct {
	Filename string // as sent, UTF-8, path components not stripped
	Size     int64
	Data     io.ReaderAt // the spooled part; only the head or central directory is read
}

// Validate runs PRD §6.8 steps 1–5 in order and stops at the first failure:
// exactly one part (400), size (413), not empty (422), magic bytes agree with
// the extension (415), expected filename (422). expectedBase is the expected
// name without extension. It reads no more than the PDF head or the ZIP
// central directory.
func Validate(parts []Part, expectedBase string) (Kind, error) {
	if len(parts) != 1 {
		return "", web.NewError(web.CodeMalformedRequest, "Exactly one slides file part is required.")
	}
	p := parts[0]
	if p.Size > MaxSlideBytes {
		return "", web.NewError(web.CodeFileTooLarge, "Slides exceed 5 MiB.")
	}
	if p.Size == 0 {
		return "", web.NewError(web.CodeEmptyFile, "Slides file is empty.")
	}
	kind, ok := kindFromExtension(p.Filename)
	if !ok || !magicMatches(kind, p) {
		return "", web.NewError(web.CodeUnsupportedFileType, "Slides must be a PDF or PPTX file whose content matches its extension.")
	}
	want := expectedBase + "." + string(kind)
	// The extension is ASCII, so the split point survives NFC; only the base is
	// compared exactly, the extension case-insensitively (kindFromExtension).
	got := norm.NFC.String(p.Filename)
	if got[:len(got)-len(kind)-1] != expectedBase {
		e := web.NewError(web.CodeFilenameMismatch, "Uploaded filename does not match the expected filename.")
		e.Fields = []web.FieldError{{Field: "slides", Code: web.CodeFilenameMismatch, Expected: want}}
		return "", e
	}
	return kind, nil
}

func kindFromExtension(name string) (Kind, bool) {
	l := strings.ToLower(name)
	switch {
	case strings.HasSuffix(l, ".pdf"):
		return PDF, true
	case strings.HasSuffix(l, ".pptx"):
		return PPTX, true
	}
	return "", false
}

func magicMatches(kind Kind, p Part) bool {
	switch kind {
	case PDF:
		head := make([]byte, 5)
		n, _ := p.Data.ReadAt(head, 0)
		return n == 5 && bytes.Equal(head, []byte("%PDF-"))
	case PPTX:
		zr, err := zip.NewReader(p.Data, p.Size)
		if err != nil {
			return false
		}
		var ct, pres bool
		for _, f := range zr.File {
			switch f.Name {
			case "[Content_Types].xml":
				ct = true
			case "ppt/presentation.xml":
				pres = true
			}
		}
		return ct && pres
	}
	return false
}
