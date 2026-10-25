//go:build integration

//nolint:cyclop,funlen,golines,intrange,lll,nlreturn,paralleltest,staticcheck
package devicepostgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	platform "github.com/VladMallory/ProxyMaster_v2/internal/platform/postgres"
	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device"
)

const defaultTestDSN = "postgres://app:change_me@localhost:5432/app?sslmode=disable"

func openTestRepository(t *testing.T) (*Repository, *sql.DB) {
	t.Helper()

	ctx := context.Background()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = defaultTestDSN
	}

	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open admin db: %v", err)
	}
	if err := admin.PingContext(ctx); err != nil {
		_ = admin.Close()
		t.Fatalf("ping admin db: %v", err)
	}

	schema := fmt.Sprintf("device_test_%d", time.Now().UnixNano())

	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		_ = admin.Close()
		t.Fatalf("create schema: %v", err)
	}

	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		_ = admin.Close()
	})

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}

	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()

	db, err := platform.Open(ctx, u.String())
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := platform.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return NewDeviceDB(db), db
}

func TestEnsureBaseSeatsCreatesAndAligns(t *testing.T) {
	ctx := context.Background()
	repo, _ := openTestRepository(t)

	expires := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

	if err := repo.EnsureBaseSeats(ctx, "42", expires, 2); err != nil {
		t.Fatalf("EnsureBaseSeats: %v", err)
	}

	seats, err := repo.ListByUser(ctx, "42")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(seats) != 2 {
		t.Fatalf("seats = %d, want 2", len(seats))
	}
	for _, seat := range seats {
		if seat.Kind != device.KindBase || !seat.ExpiresAt.Equal(expires) {
			t.Fatalf("seat = %+v, want base with expire %v", seat, expires)
		}
	}

	// Продление: срок двигается, лишние места не создаются.
	later := expires.Add(30 * 24 * time.Hour)
	if err := repo.EnsureBaseSeats(ctx, "42", later, 2); err != nil {
		t.Fatalf("EnsureBaseSeats renew: %v", err)
	}

	seats, err = repo.ListByUser(ctx, "42")
	if err != nil {
		t.Fatalf("ListByUser after renew: %v", err)
	}
	if len(seats) != 2 {
		t.Fatalf("seats after renew = %d, want 2", len(seats))
	}
	if !seats[0].ExpiresAt.Equal(later) {
		t.Fatalf("expire = %v, want %v", seats[0].ExpiresAt, later)
	}
}

func TestAddExtraSeatIdempotent(t *testing.T) {
	ctx := context.Background()
	repo, _ := openTestRepository(t)

	expires := time.Now().Add(3 * 24 * time.Hour).Truncate(time.Microsecond)

	if err := repo.AddExtraSeat(ctx, "42", "pay1", expires); err != nil {
		t.Fatalf("AddExtraSeat: %v", err)
	}
	if err := repo.AddExtraSeat(ctx, "42", "pay1", expires); err != nil {
		t.Fatalf("AddExtraSeat second: %v", err)
	}

	seats, err := repo.ListByUser(ctx, "42")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(seats) != 1 || seats[0].Kind != device.KindExtra {
		t.Fatalf("seats = %+v, want one extra seat", seats)
	}
}

func TestExtendSeatRules(t *testing.T) {
	ctx := context.Background()
	repo, _ := openTestRepository(t)

	now := time.Now().Truncate(time.Microsecond)
	expires := now.Add(10 * 24 * time.Hour)

	if err := repo.AddExtraSeat(ctx, "42", "pay-extra", expires); err != nil {
		t.Fatalf("AddExtraSeat: %v", err)
	}
	seats, err := repo.ListByUser(ctx, "42")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	seatID := seats[0].ID

	if err := repo.ExtendSeat(ctx, "42", seatID, "pay-extend", 1, now); err != nil {
		t.Fatalf("ExtendSeat: %v", err)
	}
	if err := repo.ExtendSeat(ctx, "42", seatID, "pay-extend", 1, now); err != nil {
		t.Fatalf("ExtendSeat duplicate: %v", err)
	}

	got, err := repo.GetByID(ctx, "42", seatID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !got.ExpiresAt.Equal(expires.AddDate(0, 1, 0)) {
		t.Fatalf("expire = %v, want %v", got.ExpiresAt, expires.AddDate(0, 1, 0))
	}

	// Базовое место продлить нельзя.
	if err := repo.EnsureBaseSeats(ctx, "42", expires, 1); err != nil {
		t.Fatalf("EnsureBaseSeats: %v", err)
	}
	base, err := repo.ListByUser(ctx, "42")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	var baseID int64
	for _, d := range base {
		if d.Kind == device.KindBase {
			baseID = d.ID
		}
	}
	if err := repo.ExtendSeat(
		ctx,
		"42",
		baseID,
		"pay-base",
		1,
		now,
	); !errors.Is(
		err,
		device.ErrSeatNotRenewable,
	) {
		t.Fatalf("ExtendSeat base error = %v, want ErrSeatNotRenewable", err)
	}
}

func TestBindSetsLabel(t *testing.T) {
	ctx := context.Background()
	repo, _ := openTestRepository(t)

	expires := time.Now().Add(24 * time.Hour).Truncate(time.Microsecond)

	if err := repo.EnsureBaseSeats(ctx, "42", expires, 1); err != nil {
		t.Fatalf("EnsureBaseSeats: %v", err)
	}
	seats, err := repo.ListByUser(ctx, "42")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}

	if err := repo.Bind(ctx, "42", seats[0].ID, "a", "iPhone"); err != nil {
		t.Fatalf("Bind: %v", err)
	}

	got, err := repo.GetByID(ctx, "42", seats[0].ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.HWID != "a" || got.Label != "iPhone" {
		t.Fatalf("seat = %+v, want hwid=a label=iPhone", got)
	}
}

func TestRevokeExpiredExtraOnlyExtra(t *testing.T) {
	ctx := context.Background()
	repo, _ := openTestRepository(t)

	now := time.Now()

	if err := repo.EnsureBaseSeats(ctx, "42", now.Add(-time.Hour), 1); err != nil {
		t.Fatalf("EnsureBaseSeats: %v", err)
	}
	if err := repo.AddExtraSeat(ctx, "42", "pay-expired", now.Add(-time.Hour)); err != nil {
		t.Fatalf("AddExtraSeat: %v", err)
	}

	revoked, err := repo.RevokeExpiredExtra(ctx, "42", now)
	if err != nil {
		t.Fatalf("RevokeExpiredExtra: %v", err)
	}
	if revoked != 1 {
		t.Fatalf("revoked = %d, want 1", revoked)
	}

	seats, err := repo.ListByUser(ctx, "42")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(seats) != 1 || seats[0].Kind != device.KindBase {
		t.Fatalf("seats = %+v, want only base seat kept", seats)
	}
}

func TestListExpiredUsernamesGroupsAndOrders(t *testing.T) {
	ctx := context.Background()
	repo, db := openTestRepository(t)

	now := time.Now()

	rows := []struct {
		username  string
		expiresAt time.Time
		revokedAt sql.NullTime
	}{
		{"old", now.Add(-2 * time.Hour), sql.NullTime{}},
		{"old", now.Add(-90 * time.Minute), sql.NullTime{}},
		{"new", now.Add(-time.Hour), sql.NullTime{}},
		{"active", now.Add(time.Hour), sql.NullTime{}},
		{"revoked", now.Add(-3 * time.Hour), sql.NullTime{Time: now, Valid: true}},
	}

	for _, row := range rows {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO devices (username, expires_at, revoked_at)
			VALUES ($1, $2, $3)`,
			row.username, row.expiresAt, row.revokedAt,
		); err != nil {
			t.Fatalf("insert %s: %v", row.username, err)
		}
	}

	got, err := repo.ListExpiredUsernames(ctx, now, 10)
	if err != nil {
		t.Fatalf("ListExpiredUsernames: %v", err)
	}

	want := []string{"old", "new"}
	if len(got) != len(want) {
		t.Fatalf("usernames = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("usernames = %v, want %v", got, want)
		}
	}
}

func TestAddExtraSeatConcurrent(t *testing.T) {
	ctx := context.Background()
	repo, _ := openTestRepository(t)

	expires := time.Now().Add(24 * time.Hour)

	var wg sync.WaitGroup
	errs := make(chan error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- repo.AddExtraSeat(ctx, "42", "pay-once", expires)
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("AddExtraSeat: %v", err)
		}
	}

	seats, err := repo.ListByUser(ctx, "42")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(seats) != 1 {
		t.Fatalf("seats = %d, want 1", len(seats))
	}
}
