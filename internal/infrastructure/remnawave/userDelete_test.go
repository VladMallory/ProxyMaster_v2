package remnawave

import (
	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/pkg/logger"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeleteUser(t *testing.T) {
	// Prepare a fake UUID that GetUUIDByUsername will return
	fakeUUID := "11111111-2222-3333-4444-555555555555"

	tests := []struct {
		name            string
		deleteHTTPCode  int
		wantErr         bool
		wantErrContains string
	}{
		{name: "success-204", deleteHTTPCode: http.StatusNoContent, wantErr: false},
		{name: "success-200", deleteHTTPCode: http.StatusOK, wantErr: false},
		{name: "not-found", deleteHTTPCode: http.StatusNotFound, wantErr: true, wantErrContains: "not found"},
		{name: "unauthorized", deleteHTTPCode: http.StatusUnauthorized, wantErr: true, wantErrContains: "authorization"},
		{name: "server-error", deleteHTTPCode: http.StatusInternalServerError, wantErr: true, wantErrContains: "unexpected status code"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Создаем логгер для этого теста
			logClient, _ := logger.NewSlog("debug")
			logClient.Info("starting test case", logger.Field{Key: "test_name", Value: tc.name})

			// Server handles two endpoints: by-username and delete
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// логика роутинга для тестового сервера
				if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/users/by-username/") {
					// return JSON with response.uuid
					resp := map[string]any{"response": map[string]any{"uuid": fakeUUID, "username": "alice"}}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(resp)
					logClient.Debug("handled GET /api/users/by-username/", logger.Field{Key: "username", Value: "alice"})
					return
				}

				if r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/users/") {
					w.WriteHeader(tc.deleteHTTPCode)
					logClient.Debug("handled DELETE /api/users/",
						logger.Field{Key: "status_code", Value: tc.deleteHTTPCode},
						logger.Field{Key: "uuid", Value: fakeUUID})
					return
				}

				w.WriteHeader(http.StatusNotFound)
				logClient.Warn("unexpected request",
					logger.Field{Key: "method", Value: r.Method},
					logger.Field{Key: "path", Value: r.URL.Path})
			}))
			defer srv.Close()

			logClient.Info("test server created", logger.Field{Key: "url", Value: srv.URL})

			// Build minimal config
			cfg := &config.Config{
				RemnaPanelURL:       srv.URL,
				RemnaSecretURLToken: "testtoken",
				RemnaKey:            "testkey",
			}

			client := NewRemnaClient(cfg, logClient)

			// Execute
			logClient.Info("calling DeleteUser", logger.Field{Key: "username", Value: "alice"})
			err := client.DeleteUser("alice")

			if tc.wantErr {
				if err == nil {
					logClient.Error("expected error but got nil")
					t.Fatalf("expected error but got nil")
				}
				logClient.Info("error as expected", logger.Field{Key: "error", Value: err.Error()})
				if tc.wantErrContains != "" {
					if err != nil && !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.wantErrContains)) {
						logClient.Error("error message does not contain expected substring",
							logger.Field{Key: "error", Value: err.Error()},
							logger.Field{Key: "expected_contains", Value: tc.wantErrContains})
						t.Fatalf("error message %q does not contain %q", err.Error(), tc.wantErrContains)
					}
					logClient.Info("error contains expected substring",
						logger.Field{Key: "substring", Value: tc.wantErrContains})
				}
			} else {
				if err != nil {
					logClient.Error("unexpected error", logger.Field{Key: "error", Value: err})
					t.Fatalf("unexpected error: %v", err)
				}
				logClient.Info("test passed successfully")
			}
		})
	}
}
