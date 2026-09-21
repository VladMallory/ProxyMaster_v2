//go:build e2e

package remnawave

import (
	"context"
	"errors"
	"io"
	"log"
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

	platformremnawave "github.com/VladMallory/ProxyMaster_v2/internal/platform/remnawave"
	zaplogger "github.com/VladMallory/ProxyMaster_v2/pkg/zap"
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

// e2eFixture — адаптер + сырые креды для прямых HTTP-запросов.
// Поля platform-клиента приватные, поэтому baseURL/token/apiKey храним рядом:
// они нужны для DELETE/probe, которых нет в боевом адаптере.
type e2eFixture struct {
	adapter *RemnawaveAdapter
	baseURL string
	token   string
	apiKey  string
}

// e2eClient — собирает боевого клиента из реальных env-переменных.
// Если креды не заданы или .env не поднялся — тест пропускается, а не падает:
// e2e-тесты должны уметь "не запускаться", когда панели рядом нет.
func e2eClient(t *testing.T) *e2eFixture {
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

	// Прод-клиент: таймаут 30с уже внутри platformremnawave.New, отдельно выставлять не надо.
	pc := platformremnawave.New(baseURL, token, zaplogger.New(zaplogger.Config{
		LogLevel: "error",
		Encoding: "console",
	}))

	return &e2eFixture{
		adapter: NewRemnawaveClient(pc, apiKey),
		baseURL: baseURL,
		token:   token,
		apiKey:  apiKey,
	}
}

// e2eDeleteUser — удаляет тестового юзера через прямой DELETE-запрос.
// В боевом клиенте метода удаления нет, поэтому тест делает сырой запрос,
// чтобы не мусорить созданными юзерами в панели.
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
		// 404 на уже удалённого юзера — норма, поэтому только логируем.
		t.Logf("e2e cleanup: delete %s -> %d", id, resp.StatusCode)
	}
}

// e2eDeleteUserByName — удаляет тестового юзера по имени через GetByUsername.
// Вызывается из t.Cleanup, поэтому регистрируется ДО создания юзера:
// даже если create упадёт по таймауту, панель не останется засорённой.
func e2eDeleteUserByName(t *testing.T, fx *e2eFixture, username string) {
	t.Helper()

	byName, err := fx.adapter.GetByUsername(context.Background(), username)
	if err != nil {
		if errors.Is(err, platformremnawave.ErrNotFound) {
			return // юзер не создался — чистить нечего
		}
		t.Logf("e2e cleanup: получить id для %s не вышло: %v", username, err)
		return
	}

	e2eDeleteUser(t, fx, strconv.Itoa(byName.ID))
}

// probeRaw — сырой GET, который печатает JSON-ответ как есть.
// Именно он покажет, поменялся ли контракт: если структуры в domain/ не совпадают
// с реальностью, здесь будет видно, что панель на самом деле вернула.
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

// TestE2E_FullLifecycle — полный цикл: создать -> найти по username -> найти по id -> удалить.
// Если панель поменяла API, здесь же будет видно на каком именно шаге.
func TestE2E_FullLifecycle(t *testing.T) {
	fx := e2eClient(t)
	client := fx.adapter

	ctx := context.Background()
	username := "e2e-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	const days = 7

	// Регистрируем очистку ДО создания: при таймауте create юзер всё равно
	// будет удалён из панели по имени.
	t.Cleanup(func() { e2eDeleteUserByName(t, fx, username) })

	// 1. Создаём пользователя в живой панели.
	created, err := client.CreateUser(ctx, username, days)
	require.NoError(t, err, "контракт POST /api/users мог поменяться: %v", err)
	require.Equal(t, username, created.Name)
	require.Equal(t, days, created.Days)

	// 2. Ищем по username, забираем числовой id (единственный идентификатор в v3).
	byName, err := client.GetByUsername(ctx, username)
	require.NoError(t, err, "контракт GET /api/users/by-username мог поменяться: %v", err)
	require.Equal(t, username, byName.Username)
	require.NotZero(t, byName.ID, "панель обязана вернуть числовой id")

	t.Logf("created: name=%s id=%d url=%s device=%d",
		created.Name, byName.ID, created.URL, created.Device)

	// 3. Сырой ответ — смотрим реальную форму панели.
	probeRaw(t, fx, "/api/users/by-username/"+username+"?"+fx.apiKey)

	// 4. Ищем по числовому id.
	byID, err := client.GetByID(ctx, byName.ID)
	require.NoError(t, err, "контракт GET /api/users/{userId} мог поменяться: %v", err)
	require.Equal(t, username, byID.Username)
}

// TestE2E_NotFound — на несуществующего юзера клиент обязан вернуть ErrNotFound.
func TestE2E_NotFound(t *testing.T) {
	fx := e2eClient(t)
	client := fx.adapter

	ctx := context.Background()
	username := "e2e-missing-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	t.Run("by username", func(t *testing.T) {
		_, err := client.GetByUsername(ctx, username)
		require.ErrorIs(t, err, platformremnawave.ErrNotFound)
	})

	t.Run("by id", func(t *testing.T) {
		_, err := client.GetByID(ctx, 999999999)
		require.ErrorIs(t, err, platformremnawave.ErrNotFound)
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
