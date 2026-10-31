//go:build e2e

// E2E автоплатежа: настоящие Postgres и Remnawave, платёжный шлюз подменён.
// Реальные деньги не списываются. Каждый тест создаёт своего пользователя
// в панели и удаляет его после себя, база живёт в отдельной схеме.
//
//nolint:cyclop,funlen,golines,lll,nlreturn,paralleltest,staticcheck
package autopay

import (
	"context"
	"database/sql"
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

	_ "github.com/jackc/pgx/v5/stdlib"

	platformremnawave "github.com/VladMallory/ProxyMaster_v2/internal/platform/remnawave"
	usersremnawave "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/adapter/outbound/remnawave"
)

const defaultE2EDSN = "postgres://app:change_me@localhost:5432/app?sslmode=disable"

type noopNotifier struct{}

func (noopNotifier) Notify(context.Context, error, usersremnawave.ErrorMeta) {}

type e2eEnv struct {
	db      *sql.DB
	baseURL string
	token   string
	apiKey  string
	squad   string
	price   int
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

func setupE2E(t *testing.T) *e2eEnv {
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
	require.NotEmpty(t, u.RawQuery, "в REMNA_PANEL нет query с ключом")

	price, err := strconv.Atoi(os.Getenv("PRICE_PER_MONTH"))
	require.NoError(t, err, "e2e: PRICE_PER_MONTH должен быть числом")

	return &e2eEnv{
		db:      openE2EDB(t),
		baseURL: (&url.URL{Scheme: u.Scheme, Host: u.Host}).String(),
		token:   token,
		apiKey:  u.RawQuery,
		squad:   squad,
		price:   price,
	}
}

// openE2EDB поднимает отдельную схему, чтобы тест не видел боевые таблицы.
func openE2EDB(t *testing.T) *sql.DB {
	t.Helper()

	ctx := context.Background()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = defaultE2EDSN
	}

	admin, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	require.NoError(t, admin.PingContext(ctx), "e2e: Postgres недоступен")

	schema := fmt.Sprintf("autopay_e2e_%d", time.Now().UnixNano())

	_, err = admin.ExecContext(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err, "e2e: создать схему")

	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		_ = admin.Close()
	})

	u, err := url.Parse(dsn)
	require.NoError(t, err)

	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()

	db, err := sql.Open("pgx", u.String())
	require.NoError(t, err)
	require.NoError(t, db.PingContext(ctx))

	t.Cleanup(func() { _ = db.Close() })

	return db
}

func createPanelUser(t *testing.T, env *e2eEnv, days int) string {
	t.Helper()

	client := platformremnawave.New(env.baseURL, env.token, zap.NewNop())
	users := usersremnawave.NewRemnawaveClient(
		client,
		env.apiKey,
		env.squad,
		zap.NewNop(),
		noopNotifier{},
	)

	username := fmt.Sprintf("e2e-autopay-%d", time.Now().UnixNano())

	_, err := users.CreateUser(context.Background(), username, days, 0)
	require.NoError(t, err, "контракт POST /api/users мог поменяться")

	t.Cleanup(func() { deletePanelUser(t, env, username) })

	return username
}

func deletePanelUser(t *testing.T, env *e2eEnv, username string) {
	t.Helper()

	ctx := context.Background()
	remna := newRemnaClient(env.baseURL, env.apiKey, env.token)

	user, err := remna.user(ctx, username)
	if err != nil {
		t.Logf("e2e cleanup: пользователь %s уже недоступен: %v", username, err)

		return
	}

	fullURL := strings.TrimRight(env.baseURL, "/") + "/api/users/" +
		strconv.Itoa(user.Response.ID) + "?" + env.apiKey

	req, err := http.NewRequest(http.MethodDelete, fullURL, nil)
	if err != nil {
		t.Logf("e2e cleanup: запрос удаления: %v", err)

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

// e2ePayments подменяет ЮKassa: списание всегда успешно.
type e2ePayments struct{}

func (e2ePayments) Charge(context.Context, string, int, string) (string, error) {
	return "e2e-payment", nil
}

func (e2ePayments) Status(context.Context, string) (string, error) {
	return "succeeded", nil
}

type recordingNotifier struct {
	paid   []string
	failed []string
}

func (n *recordingNotifier) Paid(userID string, _, _ int) {
	n.paid = append(n.paid, userID)
}

func (n *recordingNotifier) Failed(userID string) {
	n.failed = append(n.failed, userID)
}

func TestE2E_WorkerChargesAndExtendsRealPanel(t *testing.T) {
	env := setupE2E(t)

	ctx := context.Background()
	username := createPanelUser(t, env, 1)

	remna := newRemnaClient(env.baseURL, env.apiKey, env.token)

	before, err := remna.ExpireAt(ctx, username)
	require.NoError(t, err, "контракт GET /api/users/by-username мог поменяться")

	st := newStore(env.db)
	require.NoError(t, st.Migrate(ctx))

	rec := record{
		Username:      username,
		MethodID:      "e2e-method",
		Months:        1,
		AmountKopecks: env.price * 100,
	}
	require.NoError(t, st.Save(ctx, rec))

	notifier := &recordingNotifier{}
	w := newWorker(st, e2ePayments{}, remna, notifier)

	require.NoError(t, w.process(ctx, rec))
	require.Equal(t, []string{username}, notifier.paid)
	require.Empty(t, notifier.failed)

	after, err := remna.ExpireAt(ctx, username)
	require.NoError(t, err)

	require.WithinDuration(
		t,
		before.AddDate(0, 1, 0),
		after,
		2*time.Second,
		"подписка должна продлиться ровно на месяц",
	)

	rows, err := st.List(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Empty(t, rows[0].PendingPaymentID)
	require.True(t, rows[0].PendingTarget.IsZero())
}

func TestE2E_WorkerSkipsWhenSubscriptionIsFarAway(t *testing.T) {
	env := setupE2E(t)

	ctx := context.Background()
	username := createPanelUser(t, env, 10)

	remna := newRemnaClient(env.baseURL, env.apiKey, env.token)

	before, err := remna.ExpireAt(ctx, username)
	require.NoError(t, err)

	st := newStore(env.db)
	require.NoError(t, st.Migrate(ctx))

	rec := record{
		Username:      username,
		MethodID:      "e2e-method",
		Months:        1,
		AmountKopecks: env.price * 100,
	}
	require.NoError(t, st.Save(ctx, rec))

	notifier := &recordingNotifier{}
	w := newWorker(st, e2ePayments{}, remna, notifier)

	require.NoError(t, w.process(ctx, rec))
	require.Empty(t, notifier.paid)

	after, err := remna.ExpireAt(ctx, username)
	require.NoError(t, err)
	require.WithinDuration(t, before, after, 2*time.Second)
}

// TestE2E_YooKassaCredentials проверяет только авторизацию и сеть: список
// платежей, без создания платежа и без списания.
func TestE2E_YooKassaCredentials(t *testing.T) {
	root := repoRoot(t)
	require.NoError(t, godotenv.Load(filepath.Join(root, ".env")))

	shop := os.Getenv("YOUKASSA_SHOP_ID")
	secret := os.Getenv("YOUKASSA_SECRET_KEY")

	if shop == "" || secret == "" {
		t.Skip("e2e: не заданы YOUKASSA_SHOP_ID / YOUKASSA_SECRET_KEY")
	}

	req, err := http.NewRequest(http.MethodGet, yookassaBaseURL+"/v3/payments?limit=1", nil)
	require.NoError(t, err)
	req.SetBasicAuth(shop, secret)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	require.Less(
		t,
		resp.StatusCode,
		http.StatusMultipleChoices,
		"ЮKassa отклонила ключи: %s",
		body,
	)
}
