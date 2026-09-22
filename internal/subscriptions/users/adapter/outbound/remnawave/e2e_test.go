//go:build e2e

package remnawave

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"

	platformremnawave "github.com/VladMallory/ProxyMaster_v2/internal/platform/remnawave"
	subdomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/domain"
	zaplogger "github.com/VladMallory/ProxyMaster_v2/pkg/zap"
	"go.uber.org/zap"
)

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller не дал путь к файлу")

	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, ".env")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	t.Skip("e2e: не найден .env в родительских директориях")
	return ""
}

type e2eFixture struct {
	adapter *RemnawaveAdapter
	baseURL string
	token   string
	apiKey  string
}

func e2eClient(t *testing.T) *e2eFixture {
	t.Helper()

	root := repoRoot(t)
	if err := godotenv.Load(filepath.Join(root, ".env")); err != nil {
		t.Skipf("e2e: не удалось загрузить .env: %v", err)
	}

	panelURL := os.Getenv("REMNA_PANEL")
	token := os.Getenv("REMNA_TOKEN")

	if panelURL == "" || token == "" {
		t.Skip("e2e: не заданы REMNA_PANEL / REMNA_TOKEN")
	}

	u, err := url.Parse(panelURL)
	require.NoError(t, err, "e2e: REMNA_PANEL не парсится как URL")
	require.NotEmpty(t, u.Host, "e2e: в REMNA_PANEL нет хоста")
	require.NotEmpty(t, u.RawQuery, "e2e: в REMNA_PANEL нет query с секретным токеном")

	baseURL := (&url.URL{Scheme: u.Scheme, Host: u.Host}).String()
	apiKey := u.RawQuery

	pc := platformremnawave.New(baseURL, token, zaplogger.New(zaplogger.Config{
		LogLevel: "error",
		Encoding: "console",
	}))

	return &e2eFixture{
		adapter: NewRemnawaveClient(pc, apiKey, zap.NewNop(), stubNotifier{}),
		baseURL: baseURL,
		token:   token,
		apiKey:  apiKey,
	}
}

func e2eDeleteUser(t *testing.T, fx *e2eFixture, id string) {
	t.Helper()

	fullURL := strings.TrimRight(fx.baseURL, "/") + "/api/users/" + id + "?" + fx.apiKey

	req, err := http.NewRequest(http.MethodDelete, fullURL, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+fx.token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= http.StatusBadRequest {
		t.Logf("e2e cleanup: delete %s -> %d", id, resp.StatusCode)
	}
}

func e2eDeleteUserByName(t *testing.T, fx *e2eFixture, username string) {
	t.Helper()

	byName, err := fx.adapter.GetByUsername(context.Background(), username)
	if err != nil {
		if errors.Is(err, subdomain.ErrNoFindUser) {
			return
		}
		t.Logf("e2e cleanup: получить id для %s не вышло: %v", username, err)
		return
	}

	e2eDeleteUser(t, fx, strconv.Itoa(byName.ID))
}

func probeRaw(t *testing.T, fx *e2eFixture, path string) {
	t.Helper()

	fullURL := strings.TrimRight(fx.baseURL, "/") + path

	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+fx.token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	t.Logf("e2e raw %s -> %d\n%s", path, resp.StatusCode, raw)
}

func TestE2E_FullLifecycle(t *testing.T) {
	fx := e2eClient(t)
	client := fx.adapter

	ctx := context.Background()
	username := "e2e-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	const days = 7

	t.Cleanup(func() { e2eDeleteUserByName(t, fx, username) })

	created, err := client.CreateUser(ctx, username, days)
	require.NoError(t, err, "контракт POST /api/users мог поменяться: %v", err)
	require.Equal(t, username, created.Name)
	require.Equal(t, days, created.Days)

	byName, err := client.GetByUsername(ctx, username)
	require.NoError(t, err, "контракт GET /api/users/by-username мог поменяться: %v", err)
	require.Equal(t, username, byName.Username)
	require.NotZero(t, byName.ID, "панель обязана вернуть числовой id")

	t.Logf("created: name=%s id=%d url=%s device=%d",
		created.Name, byName.ID, created.URL, created.Device)

	probeRaw(t, fx, "/api/users/by-username/"+username+"?"+fx.apiKey)

	byID, err := client.GetByID(ctx, byName.ID)
	require.NoError(t, err, "контракт GET /api/users/{userId} мог поменяться: %v", err)
	require.Equal(t, username, byID.Username)
}

func TestE2E_NotFound(t *testing.T) {
	fx := e2eClient(t)
	client := fx.adapter

	ctx := context.Background()
	username := "e2e-missing-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	t.Run("by username", func(t *testing.T) {
		_, err := client.GetByUsername(ctx, username)
		require.ErrorIs(t, err, subdomain.ErrNoFindUser)
	})

	t.Run("by id", func(t *testing.T) {
		_, err := client.GetByID(ctx, 999999999)
		require.ErrorIs(t, err, subdomain.ErrNoFindUser)
	})
}

func TestE2E_ExtendExpire(t *testing.T) {
	fx := e2eClient(t)
	client := fx.adapter

	ctx := context.Background()
	username := "e2e-ext-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	t.Cleanup(func() { e2eDeleteUserByName(t, fx, username) })

	_, err := client.CreateUser(ctx, username, 1)
	log.Println("user create: ", username)
	require.NoError(t, err)

	byName, err := client.GetByUsername(ctx, username)
	require.NoError(t, err)
	require.NotZero(t, byName.ID)

	newExpire := time.Now().AddDate(0, 0, 30)

	err = client.ExtendExpire(ctx, byName.ID, newExpire)
	log.Println("user extend: ", byName.ID)
	require.NoError(t, err)

	got, err := client.GetByID(ctx, byName.ID)
	require.NoError(t, err)
	parsed, err := time.Parse(time.RFC3339, got.ExpireAt)
	require.NoError(t, err)
	require.InDelta(t, newExpire.Unix(), parsed.Unix(), 60)
}
