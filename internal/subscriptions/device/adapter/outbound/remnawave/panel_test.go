//nolint:cyclop,funlen,golines,intrange,lll,nlreturn,paralleltest,staticcheck
package deviceremnawave

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	platformremnawave "github.com/VladMallory/ProxyMaster_v2/internal/platform/remnawave"
	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device"
	"go.uber.org/zap"
)

func newTestPanel(t *testing.T, handler http.HandlerFunc) *Panel {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client := platformremnawave.New(srv.URL, "token", zap.NewNop())

	return NewPanel(client, "key=secret")
}

func TestSubscriptionReadsProjection(t *testing.T) {
	expire := time.Date(2026, 11, 1, 12, 30, 0, 0, time.UTC)

	panel := newTestPanel(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/users/by-username/42" {
			t.Errorf("path = %q, want /api/users/by-username/42", r.URL.Path)
		}
		if r.URL.Query().Get("key") != "secret" {
			t.Errorf("key = %q, want secret", r.URL.Query().Get("key"))
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("authorization = %q, want Bearer token", r.Header.Get("Authorization"))
		}

		_, _ = io.WriteString(
			w,
			`{"response":{"id":7,"expireAt":"2026-11-01T12:30:00Z","hwidDeviceLimit":3}}`,
		)
	})

	got, err := panel.Subscription(context.Background(), "42")
	if err != nil {
		t.Fatalf("Subscription: %v", err)
	}

	if got.UserID != 7 || got.DeviceLimit != 3 || !got.ExpireAt.Equal(expire) {
		t.Fatalf("subscription = %+v, want user=7 limit=3 expire=%v", got, expire)
	}
}

func TestSubscriptionMapsNotFound(t *testing.T) {
	panel := newTestPanel(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := panel.Subscription(context.Background(), "42")
	if !errors.Is(err, device.ErrPanelNotFound) {
		t.Fatalf("error = %v, want ErrPanelNotFound", err)
	}
}

func TestListDevicesDeduplicatesAndNames(t *testing.T) {
	panel := newTestPanel(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/hwid/devices/7" {
			t.Errorf("path = %q, want /api/hwid/devices/7", r.URL.Path)
		}

		_, _ = io.WriteString(w, `{"response":{"total":3,"devices":[
			{"hwid":"a","deviceModel":"iPhone"},
			{"hwid":"a"},
			{"hwid":"b","platform":"Android","osVersion":"14"}
		]}}`)
	})

	got, err := panel.ListDevices(context.Background(), 7)
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("devices = %+v, want 2", got)
	}
	if got[0].HWID != "a" || got[0].Name != "iPhone" {
		t.Fatalf("first = %+v, want hwid=a name=iPhone", got[0])
	}
	if got[1].HWID != "b" || got[1].Name != "Android 14" {
		t.Fatalf("second = %+v, want hwid=b name=Android 14", got[1])
	}
}

func TestDeleteHWIDSendsUserAndHWID(t *testing.T) {
	panel := newTestPanel(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/hwid/devices/delete" {
			t.Errorf("path = %q, want /api/hwid/devices/delete", r.URL.Path)
		}

		var body struct {
			UserID int    `json:"userId"`
			HWID   string `json:"hwid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if body.UserID != 7 || body.HWID != "abc" {
			t.Errorf("body = %+v, want user=7 hwid=abc", body)
		}

		_, _ = io.WriteString(w, `{"response":true}`)
	})

	if err := panel.DeleteHWID(context.Background(), 7, "abc"); err != nil {
		t.Fatalf("DeleteHWID: %v", err)
	}
}

func TestDeleteHWIDNotFoundIsSuccess(t *testing.T) {
	panel := newTestPanel(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	if err := panel.DeleteHWID(context.Background(), 7, "abc"); err != nil {
		t.Fatalf("DeleteHWID: %v", err)
	}
}

func TestUpdateUserPatchesLimitAndExpiry(t *testing.T) {
	expire := time.Date(2026, 11, 1, 12, 30, 0, 0, time.UTC)

	panel := newTestPanel(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		if r.URL.Path != "/api/users" {
			t.Errorf("path = %q, want /api/users", r.URL.Path)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}

		if body["id"] != float64(7) {
			t.Errorf("id = %v, want 7", body["id"])
		}
		if body["hwidDeviceLimit"] != float64(2) {
			t.Errorf("hwidDeviceLimit = %v, want 2", body["hwidDeviceLimit"])
		}
		if body["expireAt"] != "2026-11-01T12:30:00Z" {
			t.Errorf("expireAt = %v, want 2026-11-01T12:30:00Z", body["expireAt"])
		}

		_, _ = io.WriteString(w, `{"response":{}}`)
	})

	if err := panel.UpdateUser(context.Background(), 7, expire, 2); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
}

func TestSubscriptionParsesFractionalExpireAt(t *testing.T) {
	want := time.Date(2026, 11, 1, 12, 30, 0, 123000000, time.UTC)

	panel := newTestPanel(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(
			w,
			`{"response":{"id":7,"expireAt":"2026-11-01T12:30:00.123Z","hwidDeviceLimit":3}}`,
		)
	})

	got, err := panel.Subscription(context.Background(), "42")
	if err != nil {
		t.Fatalf("Subscription: %v", err)
	}
	if !got.ExpireAt.Equal(want) {
		t.Fatalf("expireAt = %v, want %v", got.ExpireAt, want)
	}
}

func TestSubscriptionRejectsInvalidExpireAt(t *testing.T) {
	panel := newTestPanel(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(
			w,
			`{"response":{"id":7,"expireAt":"not-a-time","hwidDeviceLimit":3}}`,
		)
	})

	if _, err := panel.Subscription(context.Background(), "42"); err == nil {
		t.Fatal("Subscription error = nil, want parse error")
	}
}

func TestListDevicesSkipsEmptyValues(t *testing.T) {
	panel := newTestPanel(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(
			w,
			`{"response":{"total":3,"devices":[{"hwid":""},{"hwid":"a"},{"hwid":""}]}}`,
		)
	})

	got, err := panel.ListDevices(context.Background(), 7)
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if len(got) != 1 || got[0].HWID != "a" {
		t.Fatalf("devices = %+v, want [a]", got)
	}
}

func TestListDevicesMapsServerError(t *testing.T) {
	panel := newTestPanel(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	if _, err := panel.ListDevices(context.Background(), 7); err == nil {
		t.Fatal("ListDevices error = nil, want server error")
	}
}

func TestDeleteHWIDMapsServerError(t *testing.T) {
	panel := newTestPanel(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	if err := panel.DeleteHWID(context.Background(), 7, "abc"); err == nil {
		t.Fatal("DeleteHWID error = nil, want server error")
	}
}

func TestUpdateUserMapsServerError(t *testing.T) {
	panel := newTestPanel(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	if err := panel.UpdateUser(context.Background(), 7, time.Now().Add(time.Hour), 2); err == nil {
		t.Fatal("UpdateUser error = nil, want server error")
	}
}
