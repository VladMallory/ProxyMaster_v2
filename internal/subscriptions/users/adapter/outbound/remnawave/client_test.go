package remnawave

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	platformremnawave "github.com/VladMallory/ProxyMaster_v2/internal/platform/remnawave"
	subdomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/domain"
	zaplogger "github.com/VladMallory/ProxyMaster_v2/pkg/zap"
	"github.com/stretchr/testify/require"
)

type fakeRoundTripper struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (f *fakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f.roundTripFunc(req)
}

type stubNotifier struct{}

func (stubNotifier) Notify(_ context.Context, _ error, _ ErrorMeta) {
}

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

func newPlatformClientForTest(
	baseURL, token string,
	rt http.RoundTripper,
) *platformremnawave.Client {
	logger := zaplogger.New(zaplogger.Config{
		LogLevel: "error",
		Encoding: "console",
	})
	c := platformremnawave.New(baseURL, token, logger)

	v := reflect.ValueOf(c).Elem().FieldByName("http")
	reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().Set(
		reflect.ValueOf(&http.Client{Transport: rt}),
	)

	return c
}

type failingReadCloser struct{}

func (failingReadCloser) Read(_ []byte) (int, error) {
	return 0, errors.New("read boom")
}

func (failingReadCloser) Close() error {
	return nil
}

//nolint:funlen
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
			baseURL: "https://remna.example/",
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
			name: "несериализуемое тело -> ошибка marshal",
			roundTrip: func(_ *http.Request) (*http.Response, error) {
				t.Error("запрос не должен дойти до транспорта")

				return nil, errors.New("unreachable")
			},
			baseURL:       "https://remna.example",
			method:        http.MethodPost,
			path:          "/api/users",
			body:          make(chan int),
			wantErr:       true,
			wantErrSubstr: "unsupported type",
		},
		{
			name: "невалидный HTTP-метод -> ошибка создания запроса",
			roundTrip: func(_ *http.Request) (*http.Response, error) {
				t.Error("запрос не должен дойти до транспорта")

				return nil, errors.New("unreachable")
			},
			baseURL:       "https://remna.example",
			method:        "GE T",
			path:          "/api/users",
			wantErr:       true,
			wantErrSubstr: "invalid method",
		},
		{
			name: "транспорт вернул ошибку -> ошибка пробрасывается как есть",
			roundTrip: func(_ *http.Request) (*http.Response, error) {
				return nil, errors.New("connection refused")
			},
			baseURL:       "https://remna.example",
			method:        http.MethodGet,
			path:          "/api/users",
			wantErr:       true,
			wantErrSubstr: "connection refused",
		},
		{
			name: "тело ответа не читается -> ошибка чтения пробрасывается как есть",
			roundTrip: func(_ *http.Request) (*http.Response, error) {
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
			wantErrSubstr: "read boom",
		},
		{
			name: "статус 404 -> ErrNotFound без попытки распарсить ответ",
			roundTrip: func(_ *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusNotFound, "не валидный JSON, но это не важно"), nil
			},
			baseURL:       "https://remna.example",
			method:        http.MethodGet,
			path:          "/api/users",
			wantErr:       true,
			wantErrSubstr: platformremnawave.ErrNotFound.Error(),
		},
		{
			name: "статус 500 -> ошибка запроса с кодом и телом ответа",
			roundTrip: func(_ *http.Request) (*http.Response, error) {
				return jsonResponse(
					http.StatusInternalServerError,
					map[string]string{"error": "boom"},
				), nil
			},
			baseURL:       "https://remna.example",
			method:        http.MethodGet,
			path:          "/api/users",
			wantErr:       true,
			wantErrSubstr: "ошибка запроса: 500",
		},
		{
			name: "невалидный JSON в ответе -> ошибка unmarshal пробрасывается как есть",
			roundTrip: func(_ *http.Request) (*http.Response, error) {
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
			wantErrSubstr: "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pc := newPlatformClientForTest(
				tt.baseURL,
				"tok",
				&fakeRoundTripper{roundTripFunc: tt.roundTrip},
			)

			got, err := platformremnawave.Do[subdomain.APIResponse](
				ctx,
				pc,
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
