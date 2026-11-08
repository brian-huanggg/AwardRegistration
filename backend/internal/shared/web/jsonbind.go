package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"reflect"
	"strings"
)

const maxJSONDepth = 32

// DecodeJSON is the one JSON body decoder (Backend.md §3, D46, ADR O1).
//
// dst must be a non-nil pointer to a struct whose client-supplied fields are
// pointers with json tags. Rules, in order:
//  1. Content-Type must be application/json, else 415.
//  2. Duplicate keys at any level, trailing data or broken JSON → 400.
//  3. Field names match exactly (case-sensitive); other keys are ignored.
//  4. A pointer field that is absent or null → 400, unless its tag has
//     omitempty (optional). A wrong JSON type → 400; no scalar coercion.
//
// An oversized body (http.MaxBytesReader) surfaces as 413 REQUEST_TOO_LARGE.
func DecodeJSON(r *http.Request, dst any) error {
	if !isJSONContentType(r.Header.Get("Content-Type")) {
		return NewError(CodeUnsupportedMedia, "Content-Type must be application/json")
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return NewError(CodeRequestTooLarge, "request body too large")
		}
		return malformed("unreadable body", err)
	}
	return decodeStrict(body, dst)
}

func isJSONContentType(v string) bool {
	mt, _, err := mime.ParseMediaType(v)
	return err == nil && mt == "application/json"
}

func malformed(detail string, cause error) *Error {
	e := NewError(CodeMalformedRequest, detail)
	e.Cause = cause
	return e
}

func decodeStrict(body []byte, dst any) error {
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Pointer || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
		panic("web.DecodeJSON: dst must be a non-nil pointer to a struct")
	}
	if err := checkSyntax(body); err != nil {
		return err
	}
	return fill(body, rv.Elem(), "")
}

// checkSyntax walks the document once: valid JSON, no duplicate keys, nothing after the value.
func checkSyntax(body []byte) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	if err := walk(dec, 0); err != nil {
		return malformed("malformed JSON", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return malformed("unexpected data after JSON value", err)
	}
	return nil
}

func walk(dec *json.Decoder, depth int) error {
	if depth > maxJSONDepth {
		return errors.New("nesting too deep")
	}
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]struct{}{}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return err
			}
			key := kt.(string)
			if _, dup := seen[key]; dup {
				return fmt.Errorf("duplicate key %q", key)
			}
			seen[key] = struct{}{}
			if err := walk(dec, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err := walk(dec, depth+1); err != nil {
				return err
			}
		}
	}
	_, err = dec.Token() // closing delimiter
	return err
}

// fill sets the fields of the struct v from the JSON object in raw, matching names exactly.
func fill(raw []byte, v reflect.Value, path string) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return malformed(at(path, "expected a JSON object"), err)
	}
	t := v.Type()
	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		name, opts, _ := strings.Cut(sf.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = sf.Name
		}
		optional := strings.Contains(opts, "omitempty")
		fpath := path + "." + name
		val, present := obj[name]
		isNull := present && bytes.Equal(bytes.TrimSpace(val), []byte("null"))
		if !present || isNull {
			if optional {
				continue
			}
			return malformed(at(fpath, "required field is missing or null"), nil)
		}
		if err := setField(val, v.Field(i), fpath); err != nil {
			return err
		}
	}
	return nil
}

func setField(raw []byte, f reflect.Value, path string) error {
	target := f
	if f.Kind() == reflect.Pointer {
		f.Set(reflect.New(f.Type().Elem()))
		target = f.Elem()
	}
	if target.Kind() == reflect.Struct && !implementsUnmarshaler(target) {
		return fill(raw, target, path)
	}
	if err := json.Unmarshal(raw, target.Addr().Interface()); err != nil {
		return malformed(at(path, "wrong JSON type"), err)
	}
	return nil
}

var unmarshalerType = reflect.TypeFor[json.Unmarshaler]()

func implementsUnmarshaler(v reflect.Value) bool {
	return v.Addr().Type().Implements(unmarshalerType)
}

func at(path, msg string) string {
	if path == "" {
		return msg
	}
	return strings.TrimPrefix(path, ".") + ": " + msg
}
