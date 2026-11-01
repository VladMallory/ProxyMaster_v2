package autoconnect

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
)

const testSubscription = "https://sub.example.com/u?x=1&y=2"

func tokenFrom(t *testing.T, link string) string {
	t.Helper()

	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parse link %q: %v", link, err)
	}

	return parsed.Query().Get("t")
}

func TestBuildHidesSubscriptionBehindToken(t *testing.T) {
	t.Parallel()

	svc := New("connect.example.com", "secret")

	link := svc.Build(ProviderHapp, testSubscription)

	if !strings.HasPrefix(link, "https://connect.example.com/happ?") {
		t.Fatalf("Build = %q, want prefix https://connect.example.com/happ?", link)
	}

	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parse link %q: %v", link, err)
	}

	query := parsed.Query()
	if len(query) != 1 {
		t.Fatalf("query = %v, want exactly one parameter", query)
	}

	if query.Get("t") == "" {
		t.Fatalf("query = %v, want token in t", query)
	}

	if strings.Contains(link, testSubscription) {
		t.Fatalf("link %q leaks raw subscription", link)
	}

	if strings.Contains(link, url.QueryEscape(testSubscription)) {
		t.Fatalf("link %q leaks escaped subscription", link)
	}
}

func TestOpenReturnsSubscription(t *testing.T) {
	t.Parallel()

	svc := New("connect.example.com", "secret")
	link := svc.Build(ProviderHapp, testSubscription)

	got, ok := svc.Open(tokenFrom(t, link))
	if !ok {
		t.Fatal("Open = false, want true for own token")
	}

	if got != testSubscription {
		t.Fatalf("Open = %q, want %q", got, testSubscription)
	}
}

func TestOpenRejectsTokenFromAnotherSecret(t *testing.T) {
	t.Parallel()

	foreign := New("connect.example.com", "other-secret")
	link := foreign.Build(ProviderHapp, testSubscription)

	if _, ok := New("connect.example.com", "secret").Open(tokenFrom(t, link)); ok {
		t.Fatal("Open = true, want false for token from another secret")
	}
}

func TestOpenRejectsTamperedToken(t *testing.T) {
	t.Parallel()

	svc := New("connect.example.com", "secret")
	token := tokenFrom(t, svc.Build(ProviderHapp, testSubscription))

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("decode token: %v", err)
	}

	raw[len(raw)-1] ^= 0xff
	tampered := base64.RawURLEncoding.EncodeToString(raw)

	if _, ok := svc.Open(tampered); ok {
		t.Fatal("Open = true, want false for tampered token")
	}
}

func TestOpenRejectsEmptyAndBrokenTokens(t *testing.T) {
	t.Parallel()

	svc := New("connect.example.com", "secret")

	for name, token := range map[string]string{
		"пустой токен":    "",
		"не base64":       "!!!",
		"короче nonce":    base64.RawURLEncoding.EncodeToString([]byte("short")),
		"только nonce":    base64.RawURLEncoding.EncodeToString(make([]byte, nonceSize)),
		"случайные байты": base64.RawURLEncoding.EncodeToString(make([]byte, nonceSize+32)),
	} {
		if _, ok := svc.Open(token); ok {
			t.Fatalf("%s: Open = true, want false", name)
		}
	}
}

func TestDisabledServiceBuildsAndOpensNothing(t *testing.T) {
	t.Parallel()

	table := []struct {
		name string
		svc  *Service
	}{
		{name: "пустой домен", svc: New("", "secret")},
		{name: "пустой секрет", svc: New("connect.example.com", "")},
	}

	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.svc.Build(ProviderHapp, testSubscription); got != "" {
				t.Fatalf("Build = %q, want empty", got)
			}

			if _, ok := tc.svc.Open("token"); ok {
				t.Fatal("Open = true, want false for disabled service")
			}
		})
	}
}

func TestBuildReturnsEmptyForEmptySubscription(t *testing.T) {
	t.Parallel()

	for _, provider := range []string{ProviderHapp, ProviderIncy} {
		if got := New("connect.example.com", "secret").Build(provider, ""); got != "" {
			t.Fatalf("%s: Build = %q, want empty", provider, got)
		}
	}
}

func TestBuildNormalizesBaseURL(t *testing.T) {
	t.Parallel()

	table := []struct {
		name string
		base string
	}{
		{name: "без схемы", base: "connect.example.com"},
		{name: "со слэшем", base: "connect.example.com/"},
		{name: "полный URL", base: "https://connect.example.com/"},
		{name: "пробелы", base: "  connect.example.com  "},
	}

	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := New(tc.base, "secret").Build(ProviderHapp, testSubscription)
			if !strings.HasPrefix(got, "https://connect.example.com/happ?") {
				t.Fatalf("Build = %q, want prefix https://connect.example.com/happ?", got)
			}
		})
	}
}

func TestBuildUsesProviderPath(t *testing.T) {
	t.Parallel()

	svc := New("connect.example.com", "secret")

	for _, provider := range []string{ProviderHapp, ProviderIncy} {
		got := svc.Build(provider, testSubscription)
		if !strings.HasPrefix(got, "https://connect.example.com/"+provider+"?") {
			t.Fatalf("%s: Build = %q, want provider path", provider, got)
		}
	}
}
