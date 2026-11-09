package web

import (
	"crypto/sha256"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"unicode/utf8"
)

// maxTextPartBytes caps one text part. Text fields are short (PRD §6.6); a
// larger one is malformed, not an excuse to buffer.
const maxTextPartBytes = 64 << 10

// MultipartSpec describes the one multipart shape a route accepts.
type MultipartSpec struct {
	// FileField is the single file part; it is required and may appear once.
	FileField string
	// TextFields are required text parts, each exactly once. Other parts are
	// drained and ignored.
	TextFields []string
	// TooLarge is the code answered when the body limit trips (FILE_TOO_LARGE
	// on slide routes).
	TooLarge ErrorCode
	// TempDir is where the file is spooled; empty means os.TempDir() (the
	// pod's /tmp emptyDir).
	TempDir string
}

// SpooledFile is the file part written to a temporary file. Its size and
// SHA-256 were computed while streaming.
type SpooledFile struct {
	Filename string // as sent, UTF-8, path components not stripped (PRD §6.8)
	Size     int64
	SHA256   [sha256.Size]byte
	f        *os.File
}

// ReaderAt gives random access for the validator (ZIP central directory).
func (s *SpooledFile) ReaderAt() io.ReaderAt { return s.f }

// Open returns a fresh reader positioned at the start, for the upload.
func (s *SpooledFile) Open() io.Reader { return io.NewSectionReader(s.f, 0, s.Size) }

// Form is a spooled multipart request. The caller must Close it (defer), which
// removes the temporary file whether the request succeeded or failed.
type Form struct {
	Fields map[string]string
	File   *SpooledFile
}

// Close removes the temporary file. Safe on a nil Form and when called twice.
func (m *Form) Close() error {
	if m == nil || m.File == nil || m.File.f == nil {
		return nil
	}
	f := m.File.f
	m.File.f = nil
	err := f.Close()
	if rerr := os.Remove(f.Name()); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
		err = errors.Join(err, rerr)
	}
	return err
}

// ReadMultipart streams the body with r.MultipartReader (never
// ParseMultipartForm, which buffers in memory and creates temp files we do
// not control). The file part is copied into a temp file through
// io.MultiWriter(file, sha256). A non-multipart body, a missing or repeated
// part, or a file part without a filename is 400 MALFORMED_REQUEST; the body
// limit is spec.TooLarge. On any error nothing is left on disk.
func ReadMultipart(r *http.Request, spec MultipartSpec) (*Form, error) {
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, malformed("request is not valid multipart/form-data", err)
	}
	form := &Form{Fields: make(map[string]string, len(spec.TextFields))}
	ok := false
	defer func() {
		if !ok {
			_ = form.Close()
		}
	}()
	wanted := make(map[string]bool, len(spec.TextFields))
	for _, n := range spec.TextFields {
		wanted[n] = true
	}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, spec.readErr(err, "unreadable multipart body")
		}
		name := part.FormName()
		switch {
		case name == spec.FileField:
			if form.File != nil {
				return nil, malformed("repeated part: "+name, nil)
			}
			form.File, err = spec.spool(part, spec.TempDir)
			if err != nil {
				return nil, err
			}
		case wanted[name]:
			if _, dup := form.Fields[name]; dup {
				return nil, malformed("repeated part: "+name, nil)
			}
			b, err := io.ReadAll(io.LimitReader(part, maxTextPartBytes+1))
			if err != nil {
				return nil, spec.readErr(err, "unreadable multipart body")
			}
			if len(b) > maxTextPartBytes || !utf8.Valid(b) {
				return nil, malformed("invalid part: "+name, nil)
			}
			form.Fields[name] = string(b)
		default:
			if _, err := io.Copy(io.Discard, part); err != nil {
				return nil, spec.readErr(err, "unreadable multipart body")
			}
		}
	}
	if form.File == nil {
		return nil, malformed("missing part: "+spec.FileField, nil)
	}
	for _, n := range spec.TextFields {
		if _, have := form.Fields[n]; !have {
			return nil, malformed("missing part: "+n, nil)
		}
	}
	ok = true
	return form, nil
}

func (s MultipartSpec) readErr(err error, detail string) error {
	if IsBodyTooLarge(err) {
		return NewError(s.TooLarge, "Request body too large.")
	}
	return malformed(detail, err)
}

func (s MultipartSpec) spool(part *multipart.Part, dir string) (*SpooledFile, error) {
	// Part.FileName() applies filepath.Base; the filename must be taken as sent.
	_, params, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
	name, has := params["filename"]
	if err != nil || !has || !utf8.ValidString(name) {
		return nil, malformed("part "+s.FileField+" needs a UTF-8 filename", err)
	}
	f, err := os.CreateTemp(dir, "upload-*")
	if err != nil {
		return nil, err // plain error: mapped to 500 by the pipeline
	}
	out := &SpooledFile{Filename: name, f: f}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), part)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		if IsBodyTooLarge(err) {
			return nil, NewError(s.TooLarge, "Request body too large.")
		}
		var pe *os.PathError
		if errors.As(err, &pe) {
			return nil, err // disk failure, not the client's fault
		}
		return nil, malformed("unreadable multipart body", err)
	}
	out.Size = n
	h.Sum(out.SHA256[:0])
	return out, nil
}
