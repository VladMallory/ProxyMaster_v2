package remnawave

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	subdomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/domain"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestClient(roundTrip func(req *http.Request) (*http.Response, error)) *RemnawaveAdapter {
	pc := newPlatformClientForTest(
		"https://remna.example",
		"tok",
		&fakeRoundTripper{roundTripFunc: roundTrip},
	)

	return NewRemnawaveClient(pc, "apiKey=x", zap.NewNop(), stubNotifier{})
}

//nolint:funlen
func TestRemnawaveClient_CreateUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		days          int
		roundTrip     func(req *http.Request) (*http.Response, error)
		wantErr       bool
		wantErrSubstr string
		check         func(t *testing.T, got subdomain.User)
	}{
		{
			name: "сервер принял юзера -> корректный маппинг и тело запроса",
			days: 30,
			roundTrip: func(req *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodPost, req.Method)
				require.Equal(t, "/api/users", req.URL.Path)
				require.Equal(t, "apiKey=x", req.URL.RawQuery)
				require.Equal(t, "Bearer tok", req.Header.Get("Authorization"))

				var got subdomain.CreateUserRequest
				require.NoError(t, json.NewDecoder(req.Body).Decode(&got))

				require.Equal(t, "vlad", got.Username)
				require.Equal(t, "ACTIVE", got.Status)
				require.Len(t, got.UUID, 36)
				require.Len(t, got.VLESSUUID, 36)
				require.Len(t, got.TrojanPassword, 12)
				require.Len(t, got.SSPassword, 12)
				require.Zero(t, got.TrafficLimitBytes)
				require.Equal(t, "MONTH", got.TrafficLimitStrategy)
				require.Empty(t, got.ActiveInternalSquads)

				require.InDelta(t, time.Now().AddDate(0, 0, 30).Unix(), got.ExpireAt.Unix(), 5)
				require.InDelta(t, time.Now().Unix(), got.CreatedAt.Unix(), 5)

				parsed, err := time.Parse(time.RFC3339, got.LastTrafficResetAt)
				require.NoError(t, err)
				require.InDelta(t, time.Now().Unix(), parsed.Unix(), 5)

				return jsonResponse(http.StatusCreated, subdomain.APIResponse{
					UserResponse: subdomain.UserResponse{
						Username:        "vlad",
						UUID:            "uuid-123",
						HWIDDeviceLimit: 3,
						SubscriptionURL: "https://sub.example.com/vlad",
					},
				}), nil
			},
			check: func(t *testing.T, got subdomain.User) {
				require.Equal(t, "vlad", got.Name)
				require.Equal(t, "uuid-123", got.UUID)
				require.Equal(t, 30, got.Days)
				require.Equal(t, 3, got.Device)
				require.Equal(t, "https://sub.example.com/vlad", got.URL)
				require.InDelta(t, time.Now().Unix(), got.ExpireAt.Unix(), 5)
			},
		},
		{
			name: "нулевое количество дней -> expireAt совпадает с now, days маппится как есть",
			days: 0,
			roundTrip: func(req *http.Request) (*http.Response, error) {
				var got subdomain.CreateUserRequest
				require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
				require.InDelta(t, time.Now().Unix(), got.ExpireAt.Unix(), 5)

				return jsonResponse(http.StatusCreated, subdomain.APIResponse{
					UserResponse: subdomain.UserResponse{Username: "vlad", UUID: "uuid-123"},
				}), nil
			},
			check: func(t *testing.T, got subdomain.User) {
				require.Equal(t, "vlad", got.Name)
				require.Zero(t, got.Days)
				require.InDelta(t, time.Now().Unix(), got.ExpireAt.Unix(), 5)
			},
		},
		{
			name: "отрицательное количество дней -> expireAt в прошлом, days маппится как есть",
			days: -1,
			roundTrip: func(req *http.Request) (*http.Response, error) {
				var got subdomain.CreateUserRequest
				require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
				require.InDelta(t, time.Now().AddDate(0, 0, -1).Unix(), got.ExpireAt.Unix(), 5)

				return jsonResponse(http.StatusCreated, subdomain.APIResponse{
					UserResponse: subdomain.UserResponse{Username: "vlad", UUID: "uuid-123"},
				}), nil
			},
			check: func(t *testing.T, got subdomain.User) {
				require.Equal(t, -1, got.Days)
				require.InDelta(t, time.Now().Unix(), got.ExpireAt.Unix(), 5)
			},
		},
		{
			name: "сервер ответил 404 -> ErrNoFindUser пробрасывается как есть",
			roundTrip: func(_ *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusNotFound, "{}"), nil
			},
			wantErr:       true,
			wantErrSubstr: subdomain.ErrNoFindUser.Error(),
		},
		{
			name: "сервер ответил 500 -> ошибка request failed пробрасывается как есть",
			roundTrip: func(_ *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusInternalServerError, "boom"), nil
			},
			wantErr:       true,
			wantErrSubstr: "ошибка запроса: 500",
		},
		{
			name: "битый JSON в ответе -> ошибка invalid character",
			roundTrip: func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusCreated,
					Status:     http.StatusText(http.StatusCreated),
					Body:       io.NopCloser(strings.NewReader("not-json")),
					Header:     make(http.Header),
				}, nil
			},
			wantErr:       true,
			wantErrSubstr: "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newTestClient(tt.roundTrip).CreateUser(
				context.Background(),
				"vlad",
				tt.days,
			)

			if tt.wantErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErrSubstr)

				return
			}
			require.NoError(t, err)
			tt.check(t, got)
		})
	}
}

//nolint:funlen
func TestRemnawaveClient_GetByUsername(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		username      string
		roundTrip     func(req *http.Request) (*http.Response, error)
		wantErr       bool
		wantErrSubstr string
		want          subdomain.UserResponse
	}{
		{
			name:     "юзер существует -> ответ маппится как есть",
			username: "vlad",
			roundTrip: func(req *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodGet, req.Method)
				require.Equal(t, "/api/users/by-username/vlad", req.URL.Path)
				require.Equal(t, "apiKey=x", req.URL.RawQuery)
				require.Nil(t, req.Body)

				return jsonResponse(http.StatusOK, subdomain.APIResponse{
					UserResponse: subdomain.UserResponse{
						UUID:            "uuid-123",
						Username:        "vlad",
						Status:          "ACTIVE",
						HWIDDeviceLimit: 3,
						SubscriptionURL: "https://sub.example.com/vlad",
					},
				}), nil
			},
			want: subdomain.UserResponse{
				UUID:            "uuid-123",
				Username:        "vlad",
				Status:          "ACTIVE",
				HWIDDeviceLimit: 3,
				SubscriptionURL: "https://sub.example.com/vlad",
			},
		},
		{
			name:     "пустой username -> запрос уходит с трейлинг-слешем (валидации нет)",
			username: "",
			roundTrip: func(req *http.Request) (*http.Response, error) {
				require.Equal(t, "/api/users/by-username/", req.URL.Path)
				require.Equal(t, "apiKey=x", req.URL.RawQuery)

				return jsonResponse(http.StatusOK, subdomain.APIResponse{
					UserResponse: subdomain.UserResponse{Username: "", UUID: "uuid-123"},
				}), nil
			},
			want: subdomain.UserResponse{UUID: "uuid-123"},
		},
		{
			name: "юзер не найден -> ErrNoFindUser как есть",
			roundTrip: func(_ *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusNotFound, "{}"), nil
			},
			wantErr:       true,
			wantErrSubstr: subdomain.ErrNoFindUser.Error(),
		},
		{
			name: "сервер ответил 500 -> ошибка пробрасывается без обёртки",
			roundTrip: func(_ *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusInternalServerError, "boom"), nil
			},
			wantErr:       true,
			wantErrSubstr: "ошибка запроса: 500",
		},
		{
			name: "битый JSON в ответе -> ошибка invalid character",
			roundTrip: func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Body:       io.NopCloser(strings.NewReader("not-json")),
					Header:     make(http.Header),
				}, nil
			},
			wantErr:       true,
			wantErrSubstr: "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newTestClient(tt.roundTrip).GetByUsername(context.Background(), tt.username)

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

//nolint:funlen
func TestRemnawaveClient_GetByID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		roundTrip     func(req *http.Request) (*http.Response, error)
		wantErr       bool
		wantErrSubstr string
		want          subdomain.UserResponse
	}{
		{
			name: "юзер существует -> ответ маппится как есть",
			roundTrip: func(req *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodGet, req.Method)
				require.Equal(t, "/api/users/1813", req.URL.Path)
				require.Equal(t, "apiKey=x", req.URL.RawQuery)
				require.Nil(t, req.Body)

				return jsonResponse(http.StatusOK, subdomain.APIResponse{
					UserResponse: subdomain.UserResponse{
						ID:              1813,
						Username:        "vlad",
						HWIDDeviceLimit: 3,
						SubscriptionURL: "https://sub.example.com/vlad",
					},
				}), nil
			},
			want: subdomain.UserResponse{
				ID:              1813,
				Username:        "vlad",
				HWIDDeviceLimit: 3,
				SubscriptionURL: "https://sub.example.com/vlad",
			},
		},
		{
			name: "юзер не найден -> ErrNoFindUser как есть",
			roundTrip: func(_ *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusNotFound, "{}"), nil
			},
			wantErr:       true,
			wantErrSubstr: subdomain.ErrNoFindUser.Error(),
		},
		{
			name: "сервер ответил 500 -> ошибка пробрасывается без обёртки",
			roundTrip: func(_ *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusInternalServerError, "boom"), nil
			},
			wantErr:       true,
			wantErrSubstr: "ошибка запроса: 500",
		},
		{
			name: "битый JSON в ответе -> ошибка invalid character",
			roundTrip: func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Body:       io.NopCloser(strings.NewReader("not-json")),
					Header:     make(http.Header),
				}, nil
			},
			wantErr:       true,
			wantErrSubstr: "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newTestClient(tt.roundTrip).GetByID(context.Background(), 1813)

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
