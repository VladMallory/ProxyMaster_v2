package incy

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	incylink "github.com/INCY-DEV/incy-link-encoder/go"

	"github.com/VladMallory/ProxyMaster_v2/internal/autoconnect"
)

var incyLinkRE = regexp.MustCompile(`incy://crypt1/[A-Za-z0-9_-]+`)

func TestHandlerServesDecodableIncyLink(t *testing.T) {
	t.Parallel()

	const (
		subscriptionURL = "https://sub.example.com/u?x=1&y=2"
		providerName    = "Shadowfade"
	)

	links := autoconnect.New("connect.example.com", "secret")
	handler := NewHandler(links, providerName)

	req := httptest.NewRequest(
		http.MethodGet,
		links.Build(autoconnect.ProviderIncy, subscriptionURL),
		nil,
	)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	raw := incyLinkRE.FindString(rec.Body.String())
	if raw == "" {
		t.Fatalf("body = %q, want incy://crypt1/... link", rec.Body.String())
	}

	decoded, err := incylink.DecryptLink(raw)
	if err != nil {
		t.Fatalf("DecryptLink: %v", err)
	}

	if decoded.URL != subscriptionURL {
		t.Fatalf("decoded URL = %q, want %q", decoded.URL, subscriptionURL)
	}

	if decoded.Name != providerName {
		t.Fatalf("decoded name = %q, want %q", decoded.Name, providerName)
	}
}
