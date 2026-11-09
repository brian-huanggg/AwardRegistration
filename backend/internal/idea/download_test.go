package idea_test

import (
	"mime"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/idea"
)

func TestContentDisposition(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"I_BSID_Amy_100001_Plan.pdf", `attachment; filename="I_BSID_Amy_100001_Plan.pdf"; filename*=UTF-8''I_BSID_Amy_100001_Plan.pdf`},
		{"O_AAID_王小明_100002_減碳.pptx", `attachment; filename="O_AAID_____100002___.pptx"; filename*=UTF-8''O_AAID_%E7%8E%8B%E5%B0%8F%E6%98%8E_100002_%E6%B8%9B%E7%A2%B3.pptx`},
		{`a "b"\c;d%e.pdf`, `attachment; filename="a _b__c;d%e.pdf"; filename*=UTF-8''a%20%22b%22%5Cc%3Bd%25e.pdf`},
	} {
		got := idea.ContentDisposition(tc.name)
		if got != tc.want {
			t.Errorf("%q:\n got %s\nwant %s", tc.name, got, tc.want)
		}
		// Go's own RFC 2231/8187 parser recovers the exact name from filename*.
		if _, params, err := mime.ParseMediaType(got); err != nil || params["filename"] != tc.name {
			t.Errorf("%q: parsed %q, %v", tc.name, params["filename"], err)
		}
	}
}
