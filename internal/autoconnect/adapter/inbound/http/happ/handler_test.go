package happ

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VladMallory/ProxyMaster_v2/internal/autoconnect"
)

type stubEncrypter struct {
	link string
	err  error

	gotSubscription string
	gotContextValue any
}

type ctxKey struct{}

func (s *stubEncrypter) Encrypt(ctx context.Context, subscriptionURL string) (string, error) {
	s.gotSubscription = subscriptionURL
	s.gotContextValue = ctx.Value(ctxKey{})

	return s.link, s.err
}

func TestDeepLinkUsesEncryptedLink(t *testing.T) {
	t.Parallel()

	const encrypted = "happ://crypt5/payload"

	enc := &stubEncrypter{link: encrypted}
	build := deepLink(enc)

	link := autoconnect.Link{Subscription: "https://sub.example.com/u"}

	got, err := build(context.Background(), link)
	if err != nil {
		t.Fatalf("deepLink: %v", err)
	}

	if got != encrypted {
		t.Fatalf("deepLink = %q, want %q", got, encrypted)
	}

	if enc.gotSubscription != "https://sub.example.com/u" {
		t.Fatalf("encrypter got %q, want subscription", enc.gotSubscription)
	}
}

func TestDeepLinkFallsBackWhenEncryptionFails(t *testing.T) {
	t.Parallel()

	enc := &stubEncrypter{err: errors.New("api down")}
	build := deepLink(enc)

	got, err := build(
		context.Background(),
		autoconnect.Link{Subscription: "https://sub.example.com/u?x=1&y=2"},
	)
	if err != nil {
		t.Fatalf("deepLink: %v", err)
	}

	want := "happ://add/https%3A%2F%2Fsub.example.com%2Fu%3Fx%3D1%26y%3D2"
	if got != want {
		t.Fatalf("deepLink = %q, want fallback %q", got, want)
	}
}

func TestDeepLinkPassesContext(t *testing.T) {
	t.Parallel()

	enc := &stubEncrypter{link: "happ://crypt5/payload"}
	build := deepLink(enc)

	ctx := context.WithValue(context.Background(), ctxKey{}, "marker")

	if _, err := build(ctx, autoconnect.Link{Subscription: "sub"}); err != nil {
		t.Fatalf("deepLink: %v", err)
	}

	if enc.gotContextValue != "marker" {
		t.Fatalf("encrypter ctx value = %v, want marker", enc.gotContextValue)
	}
}

func TestHandlerServesEncryptedDeepLink(t *testing.T) {
	t.Parallel()

	const encrypted = "happ://crypt5/payload"

	links := autoconnect.New("connect.example.com", "secret")
	handler := NewHandler(links, &stubEncrypter{link: encrypted})

	req := httptest.NewRequest(
		http.MethodGet,
		links.Build(autoconnect.ProviderHapp, "https://sub.example.com/u"),
		nil,
	)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	want := `window.location.href = "happ://crypt5/payload";`
	if !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("body = %q, want to contain %q", rec.Body.String(), want)
	}
}

func TestHandlerFallsBackWhenEncryptionFails(t *testing.T) {
	t.Parallel()

	links := autoconnect.New("connect.example.com", "secret")
	handler := NewHandler(links, &stubEncrypter{err: errors.New("api down")})

	req := httptest.NewRequest(
		http.MethodGet,
		links.Build(autoconnect.ProviderHapp, "https://sub.example.com/u"),
		nil,
	)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	want := `window.location.href = "happ://add/https%3A%2F%2Fsub.example.com%2Fu";`
	if !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("body = %q, want to contain %q", rec.Body.String(), want)
	}
}
