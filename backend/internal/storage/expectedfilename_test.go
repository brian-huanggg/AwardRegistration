package storage

import (
	"errors"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

func TestExpectedFilenameBaseRejectsUnknownCategory(t *testing.T) {
	_, err := ExpectedFilenameBase("OTHER", "AAID", "a", "000001", "b")
	var we *web.Error
	if !errors.As(err, &we) || we.Code != web.CodeInternalError {
		t.Fatalf("err = %v, want INTERNAL_ERROR", err)
	}
}
