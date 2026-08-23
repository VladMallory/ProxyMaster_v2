//go:build e2e

package remnawave

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"

	subdomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/domain"
)

// repoRoot — путь до корня репозитория (где лежит .env).
// runtime.Caller(0) даёт путь к текущему файлу, из него шагаем наверх,
// пока не найдём .env. Так тест не зависит от того, из какой директории его запустили.
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
		if parent == dir { // дошли до корня файловой системы
			break
		}
		dir = parent
	}

	t.Skip("e2e: не найден .env в родительских директориях")
	return ""
}

// e2eClient — собирает боевого клиента из реальных env-переменных.
// Если креды не заданы или .env не поднялся — тест пропускается, а не падает:
// e2e-тесты должны уметь "не запускаться", когда панели рядом нет.
func e2eClient(t *testing.T) *RemnawaveClient {
	t.Helper()

	root := repoRoot(t)
	if err := godotenv.Load(filepath.Join(root, ".env")); err != nil {
		t.Skipf("e2e: не удалось загрузить .env: %v", err)
	}

	baseURL := os.Getenv("REMNA_BASE_PANEL")
	token := os.Getenv("REMNA_TOKEN")
	apiKey := os.Getenv("REMNA_SECRET_TOKEN")

	if baseURL == "" || token == "" || apiKey == "" {
		t.Skip("e2e: не заданы REMNA_BASE_PANEL / REMNA_TOKEN / REMNA_SECRET_TOKEN")
	}

	c := NewRemnawaveClient(baseURL, token, apiKey)
	// Живая панель иногда отвечает дольше 10с — для e2e даём запас.
	c.client.Timeout = 30 * time.Second

	return c
}

// e2eDeleteUser — удаляет тестового юзера через прямой DELETE-запрос.
// В боевом клиенте метода удаления нет, поэтому тест делает сырой запрос,
// чтобы не мусорить созданными юзерами в панели.
func e2eDeleteUser(t *testing.T, client *RemnawaveClient, id string) {
	t.Helper()

	fullURL := strings.TrimRight(client.baseURL, "/") + "/api/users/" + id + "?" + client.apiKey

	req, err := http.NewRequest(http.MethodDelete, fullURL, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+client.token)

	resp, err := client.client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= http.StatusBadRequest {
		// 404 на уже удалённого юзера — норма, поэтому только логируем.
		t.Logf("e2e cleanup: delete %s -> %d", id, resp.StatusCode)
	}
}

// e2eDeleteUserByName — удаляет тестового юзера по имени через GetUUIDByUsername.
// Вызывается из t.Cleanup, поэтому регистрируется ДО создания юзера:
// даже если create упадёт по таймауту, панель не останется засорённой.
func e2eDeleteUserByName(t *testing.T, client *RemnawaveClient, username string) {
	t.Helper()

	ident, err := client.GetUUIDByUsername(context.Background(), username)
	if err != nil {
		if errors.Is(err, subdomain.ErrNoFindUser) {
			return // юзер не создался — чистить нечего
		}
		t.Logf("e2e cleanup: получить id для %s не вышло: %v", username, err)
		return
	}

	e2eDeleteUser(t, client, ident)
}

// probeRaw — сырой GET, который печатает JSON-ответ как есть.
// Именно он покажет, поменялся ли контракт: если структуры в domain/ не совпадают
// с реальностью, здесь будет видно, что панель на самом деле вернула.
func probeRaw(t *testing.T, client *RemnawaveClient, path string) {
	t.Helper()

	fullURL := strings.TrimRight(client.baseURL, "/") + path

	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+client.token)

	resp, err := client.client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	t.Logf("e2e raw %s -> %d\n%s", path, resp.StatusCode, raw)
}

// TestE2E_FullLifecycle — полный цикл: создать -> найти по username -> найти по uuid -> удалить.
// Если панель поменяла API, здесь же будет видно на каком именно шаге.
func TestE2E_FullLifecycle(t *testing.T) {
	client := e2eClient(t)

	ctx := context.Background()
	username := "e2e-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	const days = 7

	// Регистрируем очистку ДО создания: при таймауте create юзер всё равно
	// будет удалён из панели по имени.
	t.Cleanup(func() { e2eDeleteUserByName(t, client, username) })

	// 1. Создаём пользователя в живой панели.
	created, err := client.CreateUser(ctx, username, days)
	require.NoError(t, err, "контракт POST /api/users мог поменяться: %v", err)
	require.Equal(t, username, created.Name)
	require.Equal(t, days, created.Days)

	// 2. Берём идентификатор по имени (старая панель — uuid, новая — id строкой).
	ident, err := client.GetUUIDByUsername(ctx, username)
	require.NoError(t, err, "контракт GetUUIDByUsername мог поменяться: %v", err)
	require.NotEmpty(t, ident, "панель обязана вернуть идентификатор")

	t.Logf("created: name=%s ident=%s url=%s device=%d",
		created.Name, ident, created.URL, created.Device)

	// 3. Ищем по username.
	byName, err := client.GetByUsername(ctx, username)
	require.NoError(t, err, "контракт GET /api/users/by-username мог поменяться: %v", err)
	require.Equal(t, username, byName.Username)

	// 4. Сырой ответ — смотрим реальную форму панели.
	probeRaw(t, client, "/api/users/by-username/"+username+"?"+client.apiKey)

	// 5. Ищем по идентификатору.
	byID, err := client.GetByUUID(ctx, ident)
	require.NoError(t, err, "контракт GET /api/users/{id} мог поменяться: %v", err)
	require.Equal(t, username, byID.Username)
}

// TestE2E_NotFound — на несуществующего юзера клиент обязан вернуть ErrNoFindUser.
func TestE2E_NotFound(t *testing.T) {
	client := e2eClient(t)

	ctx := context.Background()
	username := "e2e-missing-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	t.Run("by username", func(t *testing.T) {
		_, err := client.GetByUsername(ctx, username)
		require.ErrorIs(t, err, subdomain.ErrNoFindUser)
	})

	t.Run("get uuid by username", func(t *testing.T) {
		_, err := client.GetUUIDByUsername(ctx, username)
		require.ErrorIs(t, err, subdomain.ErrNoFindUser)
	})

	t.Run("by uuid", func(t *testing.T) {
		_, err := client.GetByUUID(ctx, "999999999")
		require.ErrorIs(t, err, subdomain.ErrNoFindUser)
	})
}
