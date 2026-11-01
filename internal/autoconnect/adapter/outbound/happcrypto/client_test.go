package happcrypto

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// realResponseShape повторяет ответ crypto.happ.su/api-v2.php. Payload
// обрезан, клиент проверяет только префикс, содержимое ему не важно.
const realResponseShape = `{"encrypted_link":"happ://crypt5/fzvdFSFd01w6+SEZzLt6pVDVjtUwj8HYeA4="}`

func TestEncrypt(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(realResponseShape))
	}))
	t.Cleanup(server.Close)

	got, err := NewClient(server.URL).Encrypt(context.Background(), "https://sub.example.com/u")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	want := "happ://crypt5/fzvdFSFd01w6+SEZzLt6pVDVjtUwj8HYeA4="
	if got != want {
		t.Fatalf("Encrypt = %q, want %q", got, want)
	}
}

func TestEncryptSendsSubscription(t *testing.T) {
	t.Parallel()

	var (
		gotMethod string
		gotType   string
		gotURL    string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotType = r.Header.Get("Content-Type")

		var body struct {
			URL string `json:"url"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}

		gotURL = body.URL

		_, _ = w.Write([]byte(realResponseShape))
	}))
	t.Cleanup(server.Close)

	if _, err := NewClient(server.URL).Encrypt(
		context.Background(),
		"https://sub.example.com/u?x=1&y=2",
	); err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}

	if gotType != "application/json" {
		t.Fatalf("content type = %q, want application/json", gotType)
	}

	if gotURL != "https://sub.example.com/u?x=1&y=2" {
		t.Fatalf("request url = %q, want subscription", gotURL)
	}
}

func TestEncryptRejectsNonOKStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	if _, err := NewClient(server.URL).Encrypt(context.Background(), "sub"); err == nil {
		t.Fatal("Encrypt = nil error, want error for status 500")
	}
}

func TestEncryptRejectsUnexpectedLink(t *testing.T) {
	t.Parallel()

	table := map[string]string{
		"crypt4":  `{"encrypted_link":"happ://crypt4/payload"}`,
		"пустой":  `{"encrypted_link":""}`,
		"не JSON": `happ://crypt5/payload`,
		"мусор":   `{"encrypted_link":"https://evil.example.com/"}`,
	}

	for name, body := range table {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			})
			server := httptest.NewServer(handler)
			t.Cleanup(server.Close)

			if _, err := NewClient(server.URL).Encrypt(context.Background(), "sub"); err == nil {
				t.Fatalf("Encrypt = nil error, want error for body %q", body)
			}
		})
	}
}

func TestEncryptHonoursCanceledContext(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(realResponseShape))
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewClient(server.URL).Encrypt(ctx, "sub")
	if err == nil {
		t.Fatal("Encrypt = nil error, want context canceled error")
	}

	if !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("error = %v, want context canceled", err)
	}
}
