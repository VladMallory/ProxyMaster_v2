package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/VladMallory/ProxyMaster_v2/internal/autoconnect"
)

const (
	testPanelURL     = "https://panel.example.com"
	testDomain       = "https://page.example.com"
	testSecret       = "test-link-secret"
	testAPIKey       = "key=test"
	testSubscription = "https://panel.example.com/sub/vlad"
)

// newPanel поднимает фейковую панель Remnawave, которая отвечает заданным
// статусом и телом, и считает запросы.
func newPanel(t *testing.T, status int, body string, calls *int) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls != nil {
			*calls++
		}

		if r.Method != http.MethodGet {
			t.Errorf("метод = %s, ожидался GET", r.Method)
		}

		if r.URL.Path != "/api/users/by-username/vlad" {
			t.Errorf("путь = %q, ожидался /api/users/by-username/vlad", r.URL.Path)
		}

		if r.URL.RawQuery != testAPIKey {
			t.Errorf("query = %q, ожидался %q", r.URL.RawQuery, testAPIKey)
		}

		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

// testConfig собирает настройки программы для тестового пользователя.
func testConfig(panelURL string) config {
	return config{
		baseURL:    panelURL,
		apiKey:     testAPIKey,
		token:      "test-token",
		subDomain:  testDomain,
		linkSecret: testSecret,
	}
}

// TestExecutePrintsBothImportLinks проверяет, что для найденного пользователя
// печатаются ровно две ссылки, а токен каждой расшифровывается в его подписку.
func TestExecutePrintsBothImportLinks(t *testing.T) {
	srv := newPanel(
		t,
		http.StatusOK,
		`{"response":{"subscriptionUrl":"https://panel.example.com/sub/vlad"}}`,
		nil,
	)
	defer srv.Close()

	var out strings.Builder

	err := execute(context.Background(), testConfig(srv.URL), " vlad ", &out)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("строк = %d, ожидалось 2: %q", len(lines), out.String())
	}

	wantPrefixes := []string{
		"Happ: " + testDomain + "/happ?t=",
		"Incy: " + testDomain + "/incy?t=",
	}

	for i, prefix := range wantPrefixes {
		if !strings.HasPrefix(lines[i], prefix) {
			t.Errorf("строка %d = %q, ожидался префикс %q", i, lines[i], prefix)
		}
	}

	opener := autoconnect.New(testDomain, testSecret)
	valuePrefixes := []string{"Happ: ", "Incy: "}

	for i, line := range lines {
		raw := strings.TrimPrefix(line, valuePrefixes[i])

		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("строка %d: разбор ссылки: %v", i, err)
		}

		got, ok := opener.Open(u.Query().Get("t"))
		if !ok {
			t.Errorf("строка %d: токен не расшифровался", i)

			continue
		}

		if got != testSubscription {
			t.Errorf("строка %d: подписка = %q, ожидалась %q", i, got, testSubscription)
		}
	}
}

// TestExecuteUserNotFound проверяет, что 404 панели превращается в понятную
// ошибку и ничего не печатается.
func TestExecuteUserNotFound(t *testing.T) {
	srv := newPanel(t, http.StatusNotFound, `{"message":"not found"}`, nil)
	defer srv.Close()

	var out strings.Builder

	err := execute(context.Background(), testConfig(srv.URL), "vlad", &out)
	if !errors.Is(err, errUserNotFound) {
		t.Fatalf("ошибка = %v, ожидалась errUserNotFound", err)
	}

	if out.Len() != 0 {
		t.Errorf("вывод = %q, ожидался пустой", out.String())
	}
}

// TestExecuteAutoconnectDisabled проверяет, что без настроенного
// автоподключения программа не ходит в панель и не печатает ссылок.
func TestExecuteAutoconnectDisabled(t *testing.T) {
	calls := 0
	srv := newPanel(t, http.StatusOK, "", &calls)
	defer srv.Close()

	cfg := testConfig(srv.URL)
	cfg.subDomain = ""

	var out strings.Builder

	err := execute(context.Background(), cfg, "vlad", &out)
	if err == nil {
		t.Fatal("ожидалась ошибка о ненастроенном автоподключении")
	}

	if calls != 0 {
		t.Errorf("запросов к панели = %d, ожидалось 0", calls)
	}

	if out.Len() != 0 {
		t.Errorf("вывод = %q, ожидался пустой", out.String())
	}
}

// TestExecuteEmptySubscription проверяет, что пользователь без ссылки
// подписки не даёт молча пустых ссылок автоподключения.
func TestExecuteEmptySubscription(t *testing.T) {
	srv := newPanel(t, http.StatusOK, `{"response":{"subscriptionUrl":"  "}}`, nil)
	defer srv.Close()

	var out strings.Builder

	err := execute(context.Background(), testConfig(srv.URL), "vlad", &out)
	if err == nil {
		t.Fatal("ожидалась ошибка об отсутствии ссылки подписки")
	}

	if out.Len() != 0 {
		t.Errorf("вывод = %q, ожидался пустой", out.String())
	}
}

// TestReadUsername проверяет, что ввод обрезается и пустой username
// не уходит в панель.
func TestReadUsername(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		fail  bool
	}{
		{name: "обрезает пробелы", input: " 12345 \n", want: "12345"},
		{name: "пустая строка", input: "\n", fail: true},
		{name: "конец ввода", input: "", fail: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder

			got, err := readUsername(strings.NewReader(tt.input), &out)
			if tt.fail {
				if err == nil {
					t.Fatalf("ожидалась ошибка, получено %q", got)
				}

				return
			}

			if err != nil {
				t.Fatalf("readUsername: %v", err)
			}

			if got != tt.want {
				t.Errorf("username = %q, ожидался %q", got, tt.want)
			}

			if out.String() != "Username: " {
				t.Errorf("приглашение = %q, ожидалось %q", out.String(), "Username: ")
			}
		})
	}
}

// TestParsePanel проверяет разбор ссылки панели: базовый адрес без query
// и ключ API из query.
func TestParsePanel(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantURL string
		wantKey string
		fail    bool
	}{
		{
			name:    "полная ссылка",
			raw:     "https://panel.example.com/auth/login?key=test",
			wantURL: testPanelURL,
			wantKey: testAPIKey,
		},
		{name: "без scheme", raw: "panel.example.com?key=test", fail: true},
		{name: "без query", raw: "https://panel.example.com/auth/login", fail: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseURL, apiKey, err := parsePanel(tt.raw)
			if tt.fail {
				if err == nil {
					t.Fatalf("ожидалась ошибка, получено %q и %q", baseURL, apiKey)
				}

				return
			}

			if err != nil {
				t.Fatalf("parsePanel: %v", err)
			}

			if baseURL != tt.wantURL {
				t.Errorf("baseURL = %q, ожидался %q", baseURL, tt.wantURL)
			}

			if apiKey != tt.wantKey {
				t.Errorf("apiKey = %q, ожидался %q", apiKey, tt.wantKey)
			}
		})
	}
}
