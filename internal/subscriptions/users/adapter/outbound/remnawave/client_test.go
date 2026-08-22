package remnawave

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	subdomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/domain"
	"github.com/stretchr/testify/require"
)

// fakeRoundTripper - ручная реализация http.RoundTripper для тестов.
// Поле-функция: не задал roundTripFunc, а метод вызвался -> nil pointer panic.
type fakeRoundTripper struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (f *fakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f.roundTripFunc(req)
}

// jsonResponse - собирает http.Response с JSON-телом.
// Значения всегда сериализуемые, поэтому паника вместо проброса ошибки.
func jsonResponse(status int, v any) *http.Response {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(bytes.NewReader(raw)),
		Header:     make(http.Header),
	}
}

// failingReadCloser - тело ответа, которое ломается на чтении.
type failingReadCloser struct{}

func (failingReadCloser) Read(_ []byte) (int, error) {
	return 0, errors.New("read boom")
}

func (failingReadCloser) Close() error {
	return nil
}

// nolint: funlen
func TestDoRequest(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name          string
		roundTrip     func(req *http.Request) (*http.Response, error)
		baseURL       string
		method        string
		path          string
		body          any
		wantErr       bool
		wantErrSubstr string
		want          subdomain.APIResponse
	}{
		{
			name: "GET без тела -> ответ распарсен, заголовки и путь корректны",
			roundTrip: func(req *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodGet, req.Method)
				require.Equal(t, "/api/users/u1?key=1", req.URL.Path+"?"+req.URL.RawQuery)
				require.Equal(t, "Bearer tok", req.Header.Get("Authorization"))
				require.Equal(t, "application/json", req.Header.Get("Content-Type"))

				return jsonResponse(http.StatusOK, subdomain.APIResponse{
					UserResponse: subdomain.UserResponse{Username: "vlad", UUID: "u1"},
				}), nil
			},
			baseURL: "https://remna.example/", // трейлинг-слеш должен отрезаться
			method:  http.MethodGet,
			path:    "/api/users/u1?key=1",
			want: subdomain.APIResponse{
				UserResponse: subdomain.UserResponse{Username: "vlad", UUID: "u1"},
			},
		},
		{
			name: "POST с телом -> тело замаршалилось и ушло в запрос",
			roundTrip: func(req *http.Request) (*http.Response, error) {
				var got map[string]string
				require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
				require.Equal(t, map[string]string{"key": "value"}, got)

				return jsonResponse(http.StatusOK, subdomain.APIResponse{}), nil
			},
			baseURL: "https://remna.example",
			method:  http.MethodPost,
			path:    "/api/users",
			body:    map[string]string{"key": "value"},
			want:    subdomain.APIResponse{},
		},
		{
			name: "несериализуемое тело -> ошибка marshal body",
			roundTrip: func(req *http.Request) (*http.Response, error) {
				t.Error("запрос не должен дойти до транспорта")

				return nil, errors.New("unreachable")
			},
			baseURL:       "https://remna.example",
			method:        http.MethodPost,
			path:          "/api/users",
			body:          make(chan int),
			wantErr:       true,
			wantErrSubstr: "marshal body",
		},
		{
			name: "невалидный HTTP-метод -> ошибка create request",
			roundTrip: func(req *http.Request) (*http.Response, error) {
				t.Error("запрос не должен дойти до транспорта")

				return nil, errors.New("unreachable")
			},
			baseURL:       "https://remna.example",
			method:        "GE T",
			path:          "/api/users",
			wantErr:       true,
			wantErrSubstr: "create request",
		},
		{
			name: "транспорт вернул ошибку -> ошибка do request",
			roundTrip: func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("connection refused")
			},
			baseURL:       "https://remna.example",
			method:        http.MethodGet,
			path:          "/api/users",
			wantErr:       true,
			wantErrSubstr: "do request",
		},
		{
			name: "тело ответа не читается -> ошибка read body",
			roundTrip: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Body:       failingReadCloser{},
					Header:     make(http.Header),
				}, nil
			},
			baseURL:       "https://remna.example",
			method:        http.MethodGet,
			path:          "/api/users",
			wantErr:       true,
			wantErrSubstr: "read body",
		},
		{
			name: "статус 404 -> ErrNoFindUser без попытки распарсить ответ",
			roundTrip: func(req *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusNotFound, "не валидный JSON, но это не важно"), nil
			},
			baseURL:       "https://remna.example",
			method:        http.MethodGet,
			path:          "/api/users",
			wantErr:       true,
			wantErrSubstr: subdomain.ErrNoFindUser.Error(),
		},
		{
			name: "статус 500 -> request failed с кодом и телом ответа",
			roundTrip: func(req *http.Request) (*http.Response, error) {
				return jsonResponse(
					http.StatusInternalServerError,
					map[string]string{"error": "boom"},
				), nil
			},
			baseURL:       "https://remna.example",
			method:        http.MethodGet,
			path:          "/api/users",
			wantErr:       true,
			wantErrSubstr: "request failed 500",
		},
		{
			name: "невалидный JSON в ответе -> ошибка unmarshal response",
			roundTrip: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Body:       io.NopCloser(strings.NewReader("not-a-json")),
					Header:     make(http.Header),
				}, nil
			},
			baseURL:       "https://remna.example",
			method:        http.MethodGet,
			path:          "/api/users",
			wantErr:       true,
			wantErrSubstr: "unmarshal response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &http.Client{Transport: &fakeRoundTripper{roundTripFunc: tt.roundTrip}}

			got, err := doRequest[subdomain.APIResponse](
				ctx,
				client,
				tt.baseURL,
				"tok",
				tt.method,
				tt.path,
				tt.body,
			)

			if tt.wantErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErrSubstr)

				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
