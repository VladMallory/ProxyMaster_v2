package page

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VladMallory/ProxyMaster_v2/internal/autoconnect"
)

const validToken = "token"

type stubOpener struct {
	subscription string
	valid        bool
}

func (s stubOpener) Open(token string) (string, bool) {
	return s.subscription, s.valid && token == validToken
}

func okBuilder() DeepLinkBuilder {
	return func(context.Context, autoconnect.Link) (string, error) {
		return "app://add/sub", nil
	}
}

func TestServeHTTPServesDeepLink(t *testing.T) {
	t.Parallel()

	handler := NewHandler(stubOpener{subscription: "sub", valid: true}, okBuilder())

	req := httptest.NewRequest(http.MethodGet, "/?t="+validToken, nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	want := `window.location.href = "app://add/sub";`
	if !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("body = %q, want to contain %q", rec.Body.String(), want)
	}

	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}

func TestServeHTTPPassesOpenedSubscriptionToBuilder(t *testing.T) {
	t.Parallel()

	var got autoconnect.Link

	handler := NewHandler(
		stubOpener{subscription: "https://sub.example.com/u", valid: true},
		func(_ context.Context, link autoconnect.Link) (string, error) {
			got = link

			return "app://add", nil
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/?t="+validToken, nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if got.Subscription != "https://sub.example.com/u" {
		t.Fatalf("builder got %q, want opened subscription", got.Subscription)
	}
}

func TestServeHTTPPassesRequestContextToBuilder(t *testing.T) {
	t.Parallel()

	type ctxKey struct{}

	var gotValue any

	handler := NewHandler(
		stubOpener{subscription: "sub", valid: true},
		func(ctx context.Context, _ autoconnect.Link) (string, error) {
			gotValue = ctx.Value(ctxKey{})

			return "app://add", nil
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/?t="+validToken, nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxKey{}, "marker"))

	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotValue != "marker" {
		t.Fatalf("builder ctx value = %v, want marker", gotValue)
	}
}

func TestServeHTTPRejectsInvalidToken(t *testing.T) {
	t.Parallel()

	handler := NewHandler(stubOpener{subscription: "sub", valid: true}, okBuilder())

	req := httptest.NewRequest(http.MethodGet, "/?t=forged", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestServeHTTPRejectsPost(t *testing.T) {
	t.Parallel()

	handler := NewHandler(stubOpener{subscription: "sub", valid: true}, okBuilder())

	req := httptest.NewRequest(http.MethodPost, "/?t="+validToken, nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}

	if got := rec.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("Allow = %q, want GET, HEAD", got)
	}
}

func TestServeHTTPHandlesBuilderError(t *testing.T) {
	t.Parallel()

	handler := NewHandler(
		stubOpener{subscription: "sub", valid: true},
		func(context.Context, autoconnect.Link) (string, error) {
			return "", errors.New("boom")
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/?t="+validToken, nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestRegisterServesAllPaths(t *testing.T) {
	t.Parallel()

	handler := NewHandler(stubOpener{subscription: "sub", valid: true}, okBuilder())

	mux := http.NewServeMux()
	handler.Register(mux, "/one", "/two")

	for _, path := range []string{"/one", "/two"} {
		req := httptest.NewRequest(http.MethodGet, path+"?t="+validToken, nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", path, rec.Code, http.StatusOK)
		}
	}
}
