package devicepostgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/VladMallory/ProxyMaster_v2/internal/platform/closer"
	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device"
)

type Repository struct {
	db *sql.DB
}

func NewDeviceDB(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListByUser(
	ctx context.Context,
	username string,
) (out []device.Device, err error) {
	const q = `
	SELECT id, username, COALESCE(label, ''), COALESCE(hwid, ''), kind,
		expires_at, revoked_at, created_at
	FROM devices
	WHERE username = $1 AND revoked_at IS NULL
	ORDER BY created_at, id`

	rows, err := r.db.QueryContext(ctx, q, username)
	if err != nil {
		return nil, err
	}
	defer closer.HelpClose(rows, &err)

	for rows.Next() {
		var (
			d         device.Device
			revokedAt sql.NullTime
		)
		if err := rows.Scan(
			&d.ID,
			&d.Username,
			&d.Label,
			&d.HWID,
			&d.Kind,
			&d.ExpiresAt,
			&revokedAt,
			&d.CreatedAt,
		); err != nil {
			return nil, err
		}

		if revokedAt.Valid {
			d.RevokedAt = revokedAt.Time
		}

		out = append(out, d)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (r *Repository) GetByID(
	ctx context.Context,
	username string,
	id int64,
) (device.Device, error) {
	const q = `
	SELECT id, username, COALESCE(label, ''), COALESCE(hwid, ''), kind,
		expires_at, revoked_at, created_at
	FROM devices
	WHERE id = $1 AND username = $2 AND revoked_at IS NULL`

	var (
		d         device.Device
		revokedAt sql.NullTime
	)

	err := r.db.QueryRowContext(ctx, q, id, username).Scan(
		&d.ID,
		&d.Username,
		&d.Label,
		&d.HWID,
		&d.Kind,
		&d.ExpiresAt,
		&revokedAt,
		&d.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return device.Device{}, device.ErrSeatNotFound
	}
	if err != nil {
		return device.Device{}, err
	}

	if revokedAt.Valid {
		d.RevokedAt = revokedAt.Time
	}

	return d, nil
}

// EnsureBaseSeats держит у пользователя ровно count базовых мест и выравнивает
// их срок по сроку подписки. Идемпотентно: лишние не удаляет, недостающие создаёт.
func (r *Repository) EnsureBaseSeats(
	ctx context.Context,
	username string,
	expiresAt time.Time,
	count int,
) error {
	if count <= 0 {
		return nil
	}

	return r.withLockedTx(ctx, username, func(tx *sql.Tx) error {
		var base int
		if err := tx.QueryRowContext(
			ctx,
			`SELECT count(*) FROM devices WHERE username = $1 AND kind = 'base' AND revoked_at IS NULL`,
			username,
		).Scan(&base); err != nil {
			return err
		}

		if missing := count - base; missing > 0 {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO devices (username, expires_at, kind)
				SELECT $1, $2, 'base'
				FROM generate_series(1, $3)`,
				username, expiresAt, missing,
			); err != nil {
				return err
			}
		}

		_, err := tx.ExecContext(ctx, `
			UPDATE devices
			SET expires_at = $2
			WHERE username = $1 AND kind = 'base' AND revoked_at IS NULL`,
			username, expiresAt,
		)

		return err
	})
}

// withLockedTx выполняет fn в транзакции под advisory-локом пользователя:
// два параллельных платежа не должны создать место дважды.
func (r *Repository) withLockedTx(
	ctx context.Context,
	username string,
	fn func(tx *sql.Tx) error,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	committed := false

	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		username,
	); err != nil {
		return err
	}

	if err = fn(tx); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	committed = true

	return nil
}

// claimPayment помечает платёж обработанным. false значит, что этот paymentID
// уже применяли раньше и операцию повторять не нужно.
func claimPayment(ctx context.Context, tx *sql.Tx, paymentID string) (bool, error) {
	var inserted string

	err := tx.QueryRowContext(ctx, `
		INSERT INTO processed_payments (payment_id)
		VALUES ($1)
		ON CONFLICT (payment_id) DO NOTHING
		RETURNING payment_id`,
		paymentID,
	).Scan(&inserted)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

// AddExtraSeat создаёт оплаченное дополнительное место. Повторный вызов с тем же
// paymentID ничего не делает, это защита от двойного вебхука.
func (r *Repository) AddExtraSeat(
	ctx context.Context,
	username string,
	paymentID string,
	expiresAt time.Time,
) error {
	return r.withLockedTx(ctx, username, func(tx *sql.Tx) error {
		claimed, err := claimPayment(ctx, tx, paymentID)
		if err != nil || !claimed {
			return err
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO devices (username, expires_at, kind)
			VALUES ($1, $2, 'extra')`,
			username, expiresAt,
		)

		return err
	})
}

// ExtendSeat продлевает одно оплаченное место. Продлевать можно только активное
// дополнительное место; повторный платёж с тем же paymentID не продлевает дважды.
func (r *Repository) ExtendSeat(
	ctx context.Context,
	username string,
	id int64,
	paymentID string,
	months int,
	now time.Time,
) error {
	return r.withLockedTx(ctx, username, func(tx *sql.Tx) error {
		claimed, err := claimPayment(ctx, tx, paymentID)
		if err != nil || !claimed {
			return err
		}

		var (
			kind      string
			expiresAt time.Time
			revokedAt sql.NullTime
		)

		err = tx.QueryRowContext(ctx, `
			SELECT kind, expires_at, revoked_at
			FROM devices
			WHERE id = $1 AND username = $2
			FOR UPDATE`,
			id, username,
		).Scan(&kind, &expiresAt, &revokedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return device.ErrSeatNotFound
		}
		if err != nil {
			return err
		}

		if kind != device.KindExtra || revokedAt.Valid || !expiresAt.After(now) {
			return device.ErrSeatNotRenewable
		}

		_, err = tx.ExecContext(ctx, `
			UPDATE devices SET expires_at = $3
			WHERE id = $1 AND username = $2`,
			id, username, device.ExtendFrom(expiresAt, now, months),
		)

		return err
	})
}

func (r *Repository) Bind(
	ctx context.Context,
	username string,
	id int64,
	hwid string,
	label string,
) error {
	const q = `
	UPDATE devices
	SET hwid = $3, label = $4
	WHERE id = $1
	  AND username = $2
	  AND hwid IS NULL
	  AND revoked_at IS NULL
	  AND expires_at > now()
	RETURNING id`

	var updated int64

	err := r.db.QueryRowContext(ctx, q, id, username, hwid, label).Scan(&updated)
	if errors.Is(err, sql.ErrNoRows) {
		return device.ErrSeatNotFree
	}
	if err != nil {

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return device.ErrSeatNotFree
		}

		return err
	}

	return nil
}

// UpdateLabel обновляет читаемое имя уже привязанного устройства.
func (r *Repository) UpdateLabel(
	ctx context.Context,
	username string,
	id int64,
	label string,
) error {
	const q = `
	UPDATE devices
	SET label = $3
	WHERE id = $1
	  AND username = $2
	  AND hwid IS NOT NULL
	  AND revoked_at IS NULL`

	if _, err := r.db.ExecContext(ctx, q, id, username, label); err != nil {
		return err
	}

	return nil
}

func (r *Repository) Unbind(
	ctx context.Context,
	username string,
	id int64,
) error {
	const q = `
	UPDATE devices
	SET hwid = NULL, label = NULL
	WHERE id = $1
	  AND username = $2
	  AND hwid IS NOT NULL
	  AND revoked_at IS NULL`

	if _, err := r.db.ExecContext(ctx, q, id, username); err != nil {
		return err
	}

	return nil
}

func (r *Repository) ListExpiredUsernames(
	ctx context.Context,
	now time.Time,
	limit int,
) (out []string, err error) {
	if limit <= 0 {
		return nil, nil
	}

	const q = `
	SELECT username
	FROM devices
	WHERE revoked_at IS NULL AND expires_at <= $1
	GROUP BY username
	ORDER BY MIN(expires_at)
	LIMIT $2`

	rows, err := r.db.QueryContext(ctx, q, now, limit)
	if err != nil {
		return nil, err
	}
	defer closer.HelpClose(rows, &err)

	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, err
		}

		out = append(out, username)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

// RevokeExpiredExtra отзывает истёкшие дополнительные места. Базовые места
// отзывом не трогаются, их срок подтягивается к подписке отдельно.
func (r *Repository) RevokeExpiredExtra(
	ctx context.Context,
	username string,
	now time.Time,
) (int64, error) {
	const q = `
	UPDATE devices
	SET revoked_at = $2, hwid = NULL, label = NULL
	WHERE username = $1
	  AND kind = 'extra'
	  AND revoked_at IS NULL
	  AND expires_at <= $2`

	res, err := r.db.ExecContext(ctx, q, username, now)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

var _ device.Repository = (*Repository)(nil)
