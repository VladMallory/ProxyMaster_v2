//go:build e2e

// E2E-тесты мест: настоящие Postgres и Remnawave, настоящий сервис.
// Каждый тест создаёт своего пользователя в панели и удаляет его после себя,
// база живёт в отдельной схеме и сносится целиком.
//
//nolint:cyclop,funlen,golines,lll,nlreturn,paralleltest,staticcheck
package device_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
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
	"go.uber.org/zap"

	platform "github.com/VladMallory/ProxyMaster_v2/internal/platform/postgres"
	platformremnawave "github.com/VladMallory/ProxyMaster_v2/internal/platform/remnawave"
	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device"
	devicepostgres "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device/adapter/outbound/postgres"
	deviceremnawave "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device/adapter/outbound/remnawave"
	usersremnawave "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/adapter/outbound/remnawave"
)

const defaultDSN = "postgres://app:change_me@localhost:5432/app?sslmode=disable"

// noopNotifier нужен адаптеру пользователей: в тесте падать некому.
type noopNotifier struct{}

func (noopNotifier) Notify(context.Context, error, usersremnawave.ErrorMeta) {}

type e2eEnv struct {
	svc     *device.Service
	panel   *deviceremnawave.Panel
	users   *usersremnawave.RemnawaveAdapter
	db      *sql.DB
	baseURL string
	token   string
	apiKey  string
	base    int
	max     int
}

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

func envInt(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}

	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}

	return n
}

func setup(t *testing.T) *e2eEnv {
	t.Helper()

	root := repoRoot(t)
	require.NoError(t, godotenv.Load(filepath.Join(root, ".env")))

	panelURL := os.Getenv("REMNA_PANEL")
	token := os.Getenv("REMNA_TOKEN")
	squad := os.Getenv("REMNA_SQUAD_UUID")

	if panelURL == "" || token == "" || squad == "" {
		t.Skip("e2e: не заданы REMNA_PANEL / REMNA_TOKEN / REMNA_SQUAD_UUID")
	}

	u, err := url.Parse(panelURL)
	require.NoError(t, err, "REMNA_PANEL не парсится как URL")
	require.NotEmpty(t, u.Host, "в REMNA_PANEL нет хоста")
	require.NotEmpty(t, u.RawQuery, "в REMNA_PANEL нет query с секретным токеном")

	baseURL := (&url.URL{Scheme: u.Scheme, Host: u.Host}).String()
	apiKey := u.RawQuery

	client := platformremnawave.New(baseURL, token, zap.NewNop())
	db := openDB(t)
	panel := deviceremnawave.NewPanel(client, apiKey)

	base := envInt("DEVICE_LIMIT", 2)
	maxLimit := envInt("MAX_DEVICE_LIMIT", 5)

	return &e2eEnv{
		svc:     device.NewService(devicepostgres.NewDeviceDB(db), panel, base, maxLimit, zap.NewNop()),
		panel:   panel,
		users:   usersremnawave.NewRemnawaveClient(client, apiKey, squad, zap.NewNop(), noopNotifier{}),
		db:      db,
		baseURL: baseURL,
		token:   token,
		apiKey:  apiKey,
		base:    base,
		max:     maxLimit,
	}
}

// openDB поднимает отдельную схему, чтобы тесты не видели боевые данные.
func openDB(t *testing.T) *sql.DB {
	t.Helper()

	ctx := context.Background()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = defaultDSN
	}

	admin, err := sql.Open("pgx", dsn)
	require.NoError(t, err)

	if err := admin.PingContext(ctx); err != nil {
		_ = admin.Close()
		t.Fatalf("e2e: Postgres недоступен (%v), подними docker compose", err)
	}

	schema := fmt.Sprintf("device_e2e_%d", time.Now().UnixNano())

	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		_ = admin.Close()
		t.Fatalf("e2e: создать схему: %v", err)
	}

	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		_ = admin.Close()
	})

	u, err := url.Parse(dsn)
	require.NoError(t, err)

	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()

	db, err := platform.Open(ctx, u.String())
	require.NoError(t, err)

	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, platform.Migrate(ctx, db), "миграции должны накатываться на пустую базу")

	return db
}

func createUser(t *testing.T, env *e2eEnv, days int) string {
	t.Helper()

	username := fmt.Sprintf("e2e-dev-%d", time.Now().UnixNano())

	_, err := env.users.CreateUser(context.Background(), username, days, 0)
	require.NoError(t, err, "контракт POST /api/users мог поменяться")

	t.Cleanup(func() { deleteUser(t, env, username) })

	return username
}

func deleteUser(t *testing.T, env *e2eEnv, username string) {
	t.Helper()

	ctx := context.Background()

	sub, err := env.panel.Subscription(ctx, username)
	if errors.Is(err, device.ErrPanelNotFound) {
		return
	}
	if err != nil {
		t.Logf("e2e cleanup: не смог прочитать %s: %v", username, err)

		return
	}

	fullURL := strings.TrimRight(env.baseURL, "/") + "/api/users/" +
		strconv.Itoa(sub.UserID) + "?" + env.apiKey

	req, err := http.NewRequest(http.MethodDelete, fullURL, nil)
	if err != nil {
		t.Logf("e2e cleanup: запрос на удаление %s: %v", username, err)

		return
	}
	req.Header.Set("Authorization", "Bearer "+env.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Logf("e2e cleanup: удалить %s: %v", username, err)

		return
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= http.StatusBadRequest {
		t.Logf("e2e cleanup: удалить %s -> %d", username, resp.StatusCode)
	}
}

// registerHWID получает подписку так же, как клиент: с заголовками устройства.
// Панель сама создаёт запись HWID, отдельный админский вызов не нужен.
func registerHWID(t *testing.T, subscriptionURL, hwid, model string) {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, subscriptionURL, nil)
	require.NoError(t, err)

	req.Header.Set("X-Hwid", hwid)
	req.Header.Set("X-Device-Os", "iOS")
	req.Header.Set("X-Ver-Os", "18.3")
	req.Header.Set("X-Device-Model", model)
	req.Header.Set("User-Agent", "Happ/2.0")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	_, _ = io.Copy(io.Discard, resp.Body)

	require.Less(
		t,
		resp.StatusCode,
		http.StatusBadRequest,
		"подписка не выдалась, значит панель не увидела HWID",
	)
}

func panelSub(t *testing.T, env *e2eEnv, username string) device.PanelSubscription {
	t.Helper()

	sub, err := env.panel.Subscription(context.Background(), username)
	require.NoError(t, err)

	return sub
}

func findKind(t *testing.T, seats []device.Device, kind string) device.Device {
	t.Helper()

	for _, s := range seats {
		if s.Kind == kind {
			return s
		}
	}

	t.Fatalf("места вида %q нет среди %+v", kind, seats)

	return device.Device{}
}

func findSeatByID(t *testing.T, seats []device.Device, id int64) device.Device {
	t.Helper()

	for _, s := range seats {
		if s.ID == id {
			return s
		}
	}

	t.Fatalf("места id=%d нет среди %+v", id, seats)

	return device.Device{}
}

func findHWID(t *testing.T, seats []device.Device, hwid string) device.Device {
	t.Helper()

	for _, s := range seats {
		if s.HWID == hwid {
			return s
		}
	}

	t.Fatalf("места с hwid=%q нет среди %+v", hwid, seats)

	return device.Device{}
}

// Базовые места входят в подписку: их ровно DEVICE_LIMIT, срок равен сроку
// подписки, лимит панели тоже равен базовому, и продлить их отдельно нельзя.
func TestE2E_BaseSeatsIncludedInSubscription(t *testing.T) {
	env := setup(t)

	ctx := context.Background()
	username := createUser(t, env, 7)

	seats, err := env.svc.ListDevices(ctx, username)
	require.NoError(t, err)
	require.Len(t, seats, env.base)

	sub := panelSub(t, env, username)

	for _, s := range seats {
		require.Equal(t, device.KindBase, s.Kind)
		require.Empty(t, s.HWID, "новое место не должно быть занято")
		require.WithinDuration(
			t,
			sub.ExpireAt,
			s.ExpiresAt,
			2*time.Second,
			"срок базового места равен сроку подписки",
		)
	}

	require.Equal(t, env.base, sub.DeviceLimit, "лимит панели равен базовому")

	err = env.svc.ExtendSeat(ctx, username, seats[0].ID, "e2e-pay-base-"+username, 1)
	require.ErrorIs(
		t,
		err,
		device.ErrSeatNotRenewable,
		"базовое место не продлевается отдельно от подписки",
	)
}

// Покупка доп. места поднимает лимит панели на 1 и не трогает срок подписки.
// Повторный платёж с тем же paymentID не создаёт второе место, продление
// добавляет месяцы к сроку самого места.
func TestE2E_BuyAndExtendExtraSeat(t *testing.T) {
	env := setup(t)

	ctx := context.Background()
	username := createUser(t, env, 30)

	_, err := env.svc.ListDevices(ctx, username)
	require.NoError(t, err)

	before := panelSub(t, env, username)
	require.Equal(t, env.base, before.DeviceLimit)

	payID := "e2e-pay-buy-" + username
	start := time.Now()

	require.NoError(t, env.svc.BuyExtraSeat(ctx, username, payID, 2))

	seats, err := env.svc.ListDevices(ctx, username)
	require.NoError(t, err)
	require.Len(t, seats, env.base+1)

	extra := findKind(t, seats, device.KindExtra)
	require.WithinDuration(
		t,
		start.AddDate(0, 2, 0),
		extra.ExpiresAt,
		time.Minute,
		"доп. место куплено на 2 месяца",
	)

	afterBuy := panelSub(t, env, username)
	require.Equal(t, env.base+1, afterBuy.DeviceLimit, "покупка поднимает лимит панели на 1")
	require.Equal(
		t,
		before.ExpireAt.Unix(),
		afterBuy.ExpireAt.Unix(),
		"срок подписки в панели не меняется",
	)

	require.NoError(
		t,
		env.svc.BuyExtraSeat(ctx, username, payID, 2),
		"повторный платёж не должен быть ошибкой",
	)

	seats, err = env.svc.ListDevices(ctx, username)
	require.NoError(t, err)
	require.Len(t, seats, env.base+1, "повторный платёж не создаёт второе место")

	extra = findKind(t, seats, device.KindExtra)
	oldExpire := extra.ExpiresAt

	require.NoError(t, env.svc.ExtendSeat(ctx, username, extra.ID, "e2e-pay-ext-"+username, 1))

	seats, err = env.svc.ListDevices(ctx, username)
	require.NoError(t, err)

	extended := findSeatByID(t, seats, extra.ID)
	require.WithinDuration(
		t,
		oldExpire.AddDate(0, 1, 0),
		extended.ExpiresAt,
		time.Second,
		"продление добавляет месяц к текущему сроку места",
	)

	afterExtend := panelSub(t, env, username)
	require.Equal(t, env.base+1, afterExtend.DeviceLimit, "продление не меняет лимит")
	require.Equal(
		t,
		before.ExpireAt.Unix(),
		afterExtend.ExpireAt.Unix(),
		"продление места не двигает срок подписки",
	)
}

// Истёкшее доп. место отзывается, лимит панели падает на 1, срок подписки
// остаётся прежним: подписка и место живут разными сроками.
func TestE2E_ExpiredExtraSeatReleasesSeatAndLimit(t *testing.T) {
	env := setup(t)

	ctx := context.Background()
	username := createUser(t, env, 30)

	_, err := env.svc.ListDevices(ctx, username)
	require.NoError(t, err)

	require.NoError(t, env.svc.BuyExtraSeat(ctx, username, "e2e-pay-exp-"+username, 1))

	seats, err := env.svc.ListDevices(ctx, username)
	require.NoError(t, err)

	extra := findKind(t, seats, device.KindExtra)

	before := panelSub(t, env, username)
	require.Equal(t, env.base+1, before.DeviceLimit)

	// Время вперёд не подвинуть, поэтому состариваем место прямо в базе.
	_, err = env.db.ExecContext(
		ctx,
		`UPDATE devices SET expires_at = now() - interval '1 minute' WHERE id = $1`,
		extra.ID,
	)
	require.NoError(t, err)

	require.NoError(t, env.svc.ExpireSeats(ctx))

	var revoked sql.NullTime
	require.NoError(t, env.db.QueryRowContext(
		ctx,
		`SELECT revoked_at FROM devices WHERE id = $1`,
		extra.ID,
	).Scan(&revoked))
	require.True(t, revoked.Valid, "истёкшее доп. место отзывается")

	after := panelSub(t, env, username)
	require.Equal(t, env.base, after.DeviceLimit, "лимит панели возвращается к базовому")
	require.Equal(
		t,
		before.ExpireAt.Unix(),
		after.ExpireAt.Unix(),
		"отзыв места не трогает срок подписки",
	)

	seats, err = env.svc.ListDevices(ctx, username)
	require.NoError(t, err)
	require.Len(t, seats, env.base, "в списке остались только базовые места")

	for _, s := range seats {
		require.Equal(t, device.KindBase, s.Kind)
	}
}

// Имя устройства берётся из панели, а «удалить» только отвязывает устройство:
// место остаётся оплаченным, лимит панели не меняется.
func TestE2E_DeviceNameFromPanelAndDeleteUnbinds(t *testing.T) {
	env := setup(t)

	ctx := context.Background()
	username := createUser(t, env, 30)

	seats, err := env.svc.ListDevices(ctx, username)
	require.NoError(t, err)
	require.Len(t, seats, env.base)

	sub := panelSub(t, env, username)
	require.Equal(t, env.base, sub.DeviceLimit)

	created, err := env.users.GetByUsername(ctx, username)
	require.NoError(t, err)
	require.NotEmpty(t, created.SubscriptionURL)

	hwid := fmt.Sprintf("e2e-hwid-%d", time.Now().UnixNano())
	const model = "iPhone 14 Pro"

	registerHWID(t, created.SubscriptionURL, hwid, model)

	seats, err = env.svc.ListDevices(ctx, username)
	require.NoError(t, err)

	bound := findHWID(t, seats, hwid)
	require.Equal(t, model, bound.Label, "имя устройства приходит из панели")

	panelDevices, err := env.panel.ListDevices(ctx, sub.UserID)
	require.NoError(t, err)
	require.Len(t, panelDevices, 1)
	require.Equal(t, hwid, panelDevices[0].HWID)
	require.Equal(t, model, panelDevices[0].Name)

	expireBefore := bound.ExpiresAt

	require.NoError(t, env.svc.RemoveDevice(ctx, username, bound.ID))

	seats, err = env.svc.ListDevices(ctx, username)
	require.NoError(t, err)

	after := findSeatByID(t, seats, bound.ID)
	require.Empty(t, after.HWID)
	require.Empty(t, after.Label)
	require.False(t, after.Revoked(), "место не отзывается, оно остаётся оплаченным")
	require.WithinDuration(
		t,
		expireBefore,
		after.ExpiresAt,
		time.Second,
		"срок места не меняется при отвязке",
	)

	panelDevices, err = env.panel.ListDevices(ctx, sub.UserID)
	require.NoError(t, err)
	require.Empty(t, panelDevices, "HWID удалён из панели")

	afterSub := panelSub(t, env, username)
	require.Equal(t, env.base, afterSub.DeviceLimit)
	require.Equal(t, sub.ExpireAt.Unix(), afterSub.ExpireAt.Unix())
}

// Без активной подписки доп. место не продаётся: трафик всё равно остановлен.
func TestE2E_ExtraSeatRequiresActiveSubscription(t *testing.T) {
	env := setup(t)

	ctx := context.Background()
	username := createUser(t, env, 30)

	_, err := env.svc.ListDevices(ctx, username)
	require.NoError(t, err)

	// Панель не принимает срок в прошлом, поэтому подписку надо пережить.
	sub := panelSub(t, env, username)
	expire := time.Now().Add(3 * time.Second)
	require.NoError(t, env.panel.UpdateUser(ctx, sub.UserID, expire, env.base))

	time.Sleep(time.Until(expire) + 200*time.Millisecond)

	err = env.svc.BuyExtraSeat(ctx, username, "e2e-pay-closed-"+username, 1)
	require.ErrorIs(
		t,
		err,
		device.ErrSubscriptionClosed,
		"без активной подписки доп. место не продаётся",
	)

	seats, err := env.svc.ListDevices(ctx, username)
	require.NoError(t, err)

	for _, s := range seats {
		require.NotEqual(t, device.KindExtra, s.Kind, "место не должно появиться")
	}
}

// Лимит панели не превышает MAX_DEVICE_LIMIT, каждая покупка добавляет ровно 1.
func TestE2E_ExtraSeatsRespectMaxLimit(t *testing.T) {
	env := setup(t)

	if env.max <= env.base {
		t.Skipf("MAX_DEVICE_LIMIT=%d не больше DEVICE_LIMIT=%d, проверять нечего", env.max, env.base)
	}

	ctx := context.Background()
	username := createUser(t, env, 30)

	_, err := env.svc.ListDevices(ctx, username)
	require.NoError(t, err)

	limit := env.base
	for limit < env.max {
		payID := fmt.Sprintf("e2e-pay-%s-%d", username, limit)

		require.NoError(t, env.svc.BuyExtraSeat(ctx, username, payID, 1))
		limit++

		sub := panelSub(t, env, username)
		require.Equal(t, limit, sub.DeviceLimit, "каждая покупка добавляет ровно одно место")
	}

	err = env.svc.BuyExtraSeat(ctx, username, "e2e-pay-over-"+username, 1)
	require.ErrorIs(t, err, device.ErrDeviceLimitReached, "выше MAX_DEVICE_LIMIT не продаём")

	sub := panelSub(t, env, username)
	require.Equal(t, env.max, sub.DeviceLimit, "лимит панели не превысил максимум")

	seats, err := env.svc.ListDevices(ctx, username)
	require.NoError(t, err)
	require.Len(t, seats, env.max)
}
