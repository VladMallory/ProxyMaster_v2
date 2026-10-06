package platformremnawave

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
)

type benchResp struct {
	Username string `json:"username"`
	UUID     string `json:"uuid"`
}

var benchSink int

type fakeRT struct{}

func (fakeRT) RoundTrip(_ *http.Request) (*http.Response, error) {
	raw := []byte(`{"username":"vlad","uuid":"uuid-123"}`)

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(raw)),
		Header:     make(http.Header),
	}, nil
}

func BenchmarkDoGet(b *testing.B) {
	c := &Client{
		baseURL: "https://remna.example",
		token:   "tok",
		http:    &http.Client{Transport: fakeRT{}},
	}
	ctx := context.Background()
	b.ResetTimer()
	for b.Loop() {
		got, err := Do[benchResp](ctx, c, http.MethodGet, "/api/users/u1", nil)
		if err != nil {
			b.Fatal(err)
		}

		benchSink += len(got.Username) + len(got.UUID)
	}
}

func BenchmarkDoPost(b *testing.B) {
	c := &Client{
		baseURL: "https://remna.example",
		token:   "tok",
		http:    &http.Client{Transport: fakeRT{}},
	}
	ctx := context.Background()
	body := map[string]string{"key": "value"}

	b.ResetTimer()
	for b.Loop() {
		got, err := Do[benchResp](ctx, c, http.MethodPost, "/api/users", body)
		if err != nil {
			b.Fatal(err)
		}

		benchSink += len(got.Username)
	}
}
