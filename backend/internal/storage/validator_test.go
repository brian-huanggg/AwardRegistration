package storage

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

func zipOf(t *testing.T, names ...string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, n := range names {
		f, err := w.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte("x"))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func part(name string, data []byte) Part {
	return Part{Filename: name, Size: int64(len(data)), Data: bytes.NewReader(data)}
}

func TestValidate(t *testing.T) {
	const base = "I_BSID_王小明_089290_成立供應商永續學院"
	pdf := []byte("%PDF-1.7 body")
	pptx := zipOf(t, "[Content_Types].xml", "ppt/presentation.xml")
	big := append([]byte("%PDF-"), make([]byte, MaxSlideBytes-5)...)
	tooBig := append(big, 0)

	tests := []struct {
		name    string
		parts   []Part
		want    Kind
		code    web.ErrorCode
		wantExp string
	}{
		{"pdf ok", []Part{part(base+".pdf", pdf)}, PDF, "", ""},
		{"pptx ok", []Part{part(base+".pptx", pptx)}, PPTX, "", ""},
		{"extension upper case ok", []Part{part(base+".PDF", pdf)}, PDF, "", ""},
		{"NFD filename ok", []Part{part(norm.NFD.String(base)+".pdf", pdf)}, PDF, "", ""},
		{"exactly 5 MiB ok", []Part{part(base+".pdf", big)}, PDF, "", ""},
		{"no part", nil, "", web.CodeMalformedRequest, ""},
		{"two parts", []Part{part(base+".pdf", pdf), part(base+".pdf", pdf)}, "", web.CodeMalformedRequest, ""},
		{"5 MiB + 1", []Part{part(base+".pdf", tooBig)}, "", web.CodeFileTooLarge, ""},
		{"size beats type", []Part{part("x.txt", tooBig)}, "", web.CodeFileTooLarge, ""},
		{"empty", []Part{part(base+".pdf", nil)}, "", web.CodeEmptyFile, ""},
		{"empty beats type", []Part{part("x.txt", nil)}, "", web.CodeEmptyFile, ""},
		{"unknown extension", []Part{part(base+".txt", pdf)}, "", web.CodeUnsupportedFileType, ""},
		{"no extension", []Part{part(base, pdf)}, "", web.CodeUnsupportedFileType, ""},
		{"pdf extension, pptx bytes", []Part{part(base+".pdf", pptx)}, "", web.CodeUnsupportedFileType, ""},
		{"pptx extension, pdf bytes", []Part{part(base+".pptx", pdf)}, "", web.CodeUnsupportedFileType, ""},
		{"pdf magic not at offset 0", []Part{part(base+".pdf", append([]byte(" "), pdf...))}, "", web.CodeUnsupportedFileType, ""},
		{"short pdf", []Part{part(base+".pdf", []byte("%PDF"))}, "", web.CodeUnsupportedFileType, ""},
		{"zip without presentation.xml", []Part{part(base+".pptx", zipOf(t, "[Content_Types].xml"))}, "", web.CodeUnsupportedFileType, ""},
		{"zip without content types", []Part{part(base+".pptx", zipOf(t, "ppt/presentation.xml"))}, "", web.CodeUnsupportedFileType, ""},
		{"docx is not pptx", []Part{part(base+".pptx", zipOf(t, "[Content_Types].xml", "word/document.xml"))}, "", web.CodeUnsupportedFileType, ""},
		{"type beats filename", []Part{part("wrong.pdf", pptx)}, "", web.CodeUnsupportedFileType, ""},
		{"wrong base", []Part{part("I_BSID_王小明_089290_其他.pdf", pdf)}, "", web.CodeFilenameMismatch, base + ".pdf"},
		{"expected carries upload extension", []Part{part("wrong.PPTX", pptx)}, "", web.CodeFilenameMismatch, base + ".pptx"},
		{"base is case sensitive", []Part{part(strings.ToLower(base)+".pdf", pdf)}, "", web.CodeFilenameMismatch, base + ".pdf"},
		{"path is not stripped", []Part{part("dir/"+base+".pdf", pdf)}, "", web.CodeFilenameMismatch, base + ".pdf"},
		{"surrounding space", []Part{part(" "+base+".pdf", pdf)}, "", web.CodeFilenameMismatch, base + ".pdf"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Validate(tc.parts, base)
			if tc.code == "" {
				if err != nil || got != tc.want {
					t.Fatalf("got (%q, %v), want (%q, nil)", got, err, tc.want)
				}
				return
			}
			var we *web.Error
			if !errors.As(err, &we) || we.Code != tc.code {
				t.Fatalf("got %v, want code %s", err, tc.code)
			}
			if tc.wantExp != "" && (len(we.Fields) != 1 || we.Fields[0].Expected != tc.wantExp || we.Fields[0].Field != "slides") {
				t.Fatalf("fields = %+v, want expected %q", we.Fields, tc.wantExp)
			}
		})
	}
}
