package storage

import (
	"golang.org/x/text/unicode/norm"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// MaxFilenameBytes is the longest expected filename in UTF-8 bytes (PRD §6.2).
const MaxFilenameBytes = 255

// longestExt is the extension used for the length rule: the file type is
// unknown at create/edit time, so ".pptx" (the longer one) is assumed.
const longestExt = ".pptx"

// ExpectedFilenameBase builds {I|O}_{org}_{leaderName}_{leaderEmpNo}_{ideaName}
// without extension by plain concatenation (PRD §6.8). All inputs are the
// already cleaned values; they are NFC-normalized again so the result never
// depends on how the caller stored them. category is COOL_IDEA or
// OUTSTANDING_IMPACT; any other value is a programming error and is reported
// as INTERNAL_ERROR rather than guessed.
func ExpectedFilenameBase(category, org, leaderName, leaderEmpNo, ideaName string) (string, error) {
	var prefix string
	switch category {
	case "COOL_IDEA":
		prefix = "I"
	case "OUTSTANDING_IMPACT":
		prefix = "O"
	default:
		return "", web.NewError(web.CodeInternalError, "")
	}
	base := prefix + "_" + org + "_" + leaderName + "_" + leaderEmpNo + "_" + ideaName
	return norm.NFC.String(base), nil
}

// CheckFilenameLength enforces PRD §6.2: base plus ".pptx" must not exceed
// 255 UTF-8 bytes, else 422 FILENAME_TOO_LONG.
func CheckFilenameLength(base string) error {
	if len(base)+len(longestExt) > MaxFilenameBytes {
		return web.NewError(web.CodeFilenameTooLong, "Expected filename exceeds 255 UTF-8 bytes.")
	}
	return nil
}
