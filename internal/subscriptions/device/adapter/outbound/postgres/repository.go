package postgresremnawave

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/platform/closer"
	devicedomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device"
)

// Проверяем на этапе компиляции, что адаптер реализует порт сервиса.
var _ devicedomain.Repository = (*Repository)(nil)

// deviceColumns единый список колонок: чтобы scanDevice и запросы не разъезжались.
const deviceColumns = `id, username, hwid, label, expires_at, revoked_at, created_at`

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// scanner общий для *sql.Row и *sql.Rows, чтобы не дублировать разбор колонок.
type scanner interface {
	Scan(dest ...any) error
}

// scanDevice собирает Device из строки, превращая NULL в пустое значение.
func scanDevice(sc scanner) (devicedomain.Device, error) {
	var (
		d       devicedomain.Device
		hwid    sql.NullString
		label   sql.NullString
		revoked sql.NullTime
	)

	if err := sc.Scan(
		&d.ID, &d.Username, &hwid, &label, &d.ExpiresAt, &revoked, &d.CreatedAt,
	); err != nil {
		return devicedomain.Device{}, err
	}

	d.HWID = hwid.String
	d.Label = label.String
	if revoked.Valid {
		d.RevokedAt = revoked.Time
	}

	return d, nil
}

// claimPayment помечает платёж обработанным в рамках транзакции tx.
// Возвращает false, если это повтор (строки в processed_payments уже была).
// Благодаря ON CONFLICT две параллельные транзакции не пройдут обе.
func claimPayment(ctx context.Context, tx *sql.Tx, paymentID string) (bool, error) {
	const q = `
INSERT INTO processed_payments (payment_id)
VALUES ($1)
ON CONFLICT (payment_id) DO NOTHING`

	res, err := tx.ExecContext(ctx, q, paymentID)
	if err != nil {
		return false, fmt.Errorf("postgres claim payment: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("postgres claim payment rows: %w", err)
	}

	return n > 0, nil
}

// Grant в одной транзакции помечает платёж и создаёт count мест.
// Если платёж уже обработан, applied=false, БД не меняется.
func (r *Repository) Grant(
	ctx context.Context,
	paymentID, username string,
	count, months int,
	now time.Time,
) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("postgres grant begin: %w", err)
	}
	// Rollback после успешного Commit вернёт ErrTxDone — это безопасно.
	defer func() { _ = tx.Rollback() }()

	applied, err := claimPayment(ctx, tx, paymentID)
	if err != nil {
		return false, err
	}
	if !applied {
		return false, nil
	}

	// generate_series размножает одну строку на count — одним запросом.
	const q = `
INSERT INTO devices (username, expires_at, created_at)
SELECT $1, $2, $3
FROM generate_series(1, $4)`

	expiresAt := devicedomain.NewExpiry(now, months)

	if _, err := tx.ExecContext(ctx, q, username, expiresAt, now, count); err != nil {
		return false, fmt.Errorf("postgres grant insert devices: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("postgres grant commit: %w", err)
	}

	return true, nil
}

// Extend в одной транзакции продлевает конкретное место.
//
// FOR UPDATE блокирует строку: два одновременных продления не затрут друг друга.
func (r *Repository) Extend(
	ctx context.Context,
	paymentID, username string,
	deviceID int64,
	months int,
	now time.Time,
) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("postgres extend begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	applied, err := claimPayment(ctx, tx, paymentID)
	if err != nil {
		return false, err
	}
	if !applied {
		return false, nil
	}

	// Берём текущий срок под блокировкой. Проверка username здесь же — чужое
	// место продлить нельзя.
	const sel = `
SELECT expires_at, revoked_at
FROM devices
WHERE id = $1 AND username = $2
FOR UPDATE`

	var (
		current time.Time
		revoked sql.NullTime
	)

	err = tx.QueryRowContext(ctx, sel, deviceID, username).Scan(&current, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		// Место не найдено/чужое: транзакция откатится, платёж останется
		// необработанным — клиент сможет повторить корректно.
		return false, devicedomain.ErrDeviceNotFound
	}
	if err != nil {
		return false, fmt.Errorf("postgres extend select: %w", err)
	}

	newExpiry := devicedomain.ExtendFrom(current, now, months)

	// Если место было отозвано — снимаем hwid/label: место станет свободным и
	// примет новое подключение. Для ещё активного места железо сохраняем.
	// В UPDATE правая часть CASE видит СТАРЫЕ значения строки.
	const upd = `
UPDATE devices
SET expires_at = $1,
    revoked_at = NULL,
    hwid       = CASE WHEN revoked_at IS NOT NULL THEN NULL ELSE hwid END,
    label      = CASE WHEN revoked_at IS NOT NULL THEN NULL ELSE label END
WHERE id = $2 AND username = $3`

	if _, err := tx.ExecContext(ctx, upd, newExpiry, deviceID, username); err != nil {
		return false, fmt.Errorf("postgres extend update: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("postgres extend commit: %w", err)
	}

	return true, nil
}

// ListByUser все места username. Порядок (created_at, id) критичен: сервис
// выбирает «самое раннее свободное» место, доверяя этому порядку.
func (r *Repository) ListByUser(
	ctx context.Context,
	username string,
) ([]devicedomain.Device, error) {
	q := `
SELECT ` + deviceColumns + `
FROM devices
WHERE username = $1
ORDER BY created_at ASC, id ASC`

	rows, err := r.db.QueryContext(ctx, q, username)
	if err != nil {
		return nil, fmt.Errorf("postgres device ListByUser: %w", err)
	}
	defer closer.HelpClose(rows, &err)

	out := []devicedomain.Device{}

	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres device scan: %w", err)
		}

		out = append(out, d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres device rows: %w", err)
	}

	return out, nil
}

// Bind пишет hwid и label в СВОБОДНОЕ место. Условие hwid IS NULL защищает от
// гонки двух воркеров: второй Bind просто не изменит ни одной строки.
func (r *Repository) Bind(ctx context.Context, deviceID int64, hwid, label string) error {
	const q = `
UPDATE devices
SET hwid = $1, label = $2
WHERE id = $3 AND hwid IS NULL AND revoked_at IS NULL`

	if _, err := r.db.ExecContext(ctx, q, hwid, label, deviceID); err != nil {
		return fmt.Errorf("postgres device Bind: %w", err)
	}

	return nil
}

// Unbind освобождает место, сохраняя оплаченный срок.
func (r *Repository) Unbind(ctx context.Context, username string, deviceID int64) error {
	const q = `
UPDATE devices
SET hwid = NULL, label = NULL
WHERE id = $1 AND username = $2`

	if _, err := r.db.ExecContext(ctx, q, deviceID, username); err != nil {
		return fmt.Errorf("postgres device Unbind: %w", err)
	}

	return nil
}

// Import заливает места, прочитанные из панели, пользователю, у которого в БД
// ещё нет строк.
//
// Advisory-lock по username держится до конца транзакции и сериализует
// параллельные заходы на экран: второй клик дождётся первого и увидит, что
// строки уже есть, — дублей не будет.
func (r *Repository) Import(
	ctx context.Context,
	username string,
	rows []devicedomain.Device,
) error {
	if len(rows) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("postgres import begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const lock = `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`
	if _, err := tx.ExecContext(ctx, lock, username); err != nil {
		return fmt.Errorf("postgres import lock: %w", err)
	}

	var existing int
	if err := tx.QueryRowContext(ctx,
		`SELECT count(*) FROM devices WHERE username = $1`, username,
	).Scan(&existing); err != nil {
		return fmt.Errorf("postgres import count: %w", err)
	}
	// Пока ждали блокировку, строки мог залить параллельный заход — импорт не нужен.
	if existing > 0 {
		return tx.Commit()
	}

	const q = `
INSERT INTO devices (username, hwid, label, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5)`
	now := time.Now().UTC()

	for _, d := range rows {
		// Пустое место храним как NULL, а не пустую строку: на hwid IS NULL
		// завязаны и частичный уникальный индекс, и выбор свободного места.
		var hwid, label any
		if d.HWID != "" {
			hwid, label = d.HWID, d.Label
		}

		if _, err := tx.ExecContext(ctx, q, username, hwid, label, d.ExpiresAt, now); err != nil {
			return fmt.Errorf("postgres import insert: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("postgres import commit: %w", err)
	}

	return nil
}

// ListToRevoke места, у которых истёк срок и которые ещё не отозваны.
// Партиальный индекс devices_to_revoke_idx делает этот запрос дешёвым.
func (r *Repository) ListToRevoke(
	ctx context.Context,
	now time.Time,
) ([]devicedomain.Device, error) {
	q := `
SELECT ` + deviceColumns + `
FROM devices
WHERE revoked_at IS NULL AND expires_at <= $1
ORDER BY expires_at ASC`

	rows, err := r.db.QueryContext(ctx, q, now)
	if err != nil {
		return nil, fmt.Errorf("postgres device ListToRevoke: %w", err)
	}
	defer closer.HelpClose(rows, &err)

	out := []devicedomain.Device{}

	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres device scan to revoke: %w", err)
		}

		out = append(out, d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres device rows to revoke: %w", err)
	}

	return out, nil
}

// MarkRevoked закрывает место. Условие revoked_at IS NULL делает вызов
// идемпотентным: повторный отзыв ничего не меняет.
func (r *Repository) MarkRevoked(ctx context.Context, deviceID int64, now time.Time) error {
	const q = `UPDATE devices SET revoked_at = $1 WHERE id = $2 AND revoked_at IS NULL`

	if _, err := r.db.ExecContext(ctx, q, now, deviceID); err != nil {
		return fmt.Errorf("postgres device MarkRevoked: %w", err)
	}

	return nil
}

// UsersToSync username, у которых с since менялись сроки или появлялись места.
func (r *Repository) UsersToSync(
	ctx context.Context,
	since, until time.Time,
) ([]string, error) {
	const q = `
SELECT DISTINCT username
FROM devices
WHERE (expires_at > $1 AND expires_at <= $2)
   OR (created_at > $1 AND created_at <= $2)
ORDER BY username ASC`

	rows, err := r.db.QueryContext(ctx, q, since, until)
	if err != nil {
		return nil, fmt.Errorf("postgres device UsersToSync: %w", err)
	}
	defer closer.HelpClose(rows, &err)

	users := []string{}

	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, fmt.Errorf("postgres device scan username: %w", err)
		}

		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres device rows usernames: %w", err)
	}

	return users, nil
}
