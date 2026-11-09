package server

import (
	"github.com/brian-huanggg/AwardRegistration/backend/internal/idea"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/storage"
)

// IdeaSlideRules adapts the storage slide rules to idea.SlideRules. It lives
// here because idea must not import storage (D29) and storage must not know
// idea's types.
func IdeaSlideRules() idea.SlideRules {
	return idea.SlideRules{
		ExpectedBase: storage.ExpectedFilenameBase,
		CheckLength:  storage.CheckFilenameLength,
		Validate: func(f idea.SlideFile, expectedBase string) (string, error) {
			// ReadMultipart guarantees exactly one part (step 1, 400).
			kind, err := storage.Validate([]storage.Part{{Filename: f.Filename, Size: f.Size, Data: f.Data}}, expectedBase)
			return string(kind), err
		},
	}
}
