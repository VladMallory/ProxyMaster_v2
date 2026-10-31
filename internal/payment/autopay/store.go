package autopay

import (
	"context"
	"database/sql"
	"time"
)

type record struct {
	Username         string
	MethodID         string
	Months           int
	AmountKopecks    int
	PendingPaymentID string
	PendingTarget    time.Time
}

type store struct {
	db *sql.DB
}

func newStore(db *sql.DB) *store {
	return &store{db: db}
}

func (s *store) Migrate(ctx context.Context) error {
	const q = `
	CREATE TABLE IF NOT EXISTS autopayments (
    username           TEXT PRIMARY KEY,
    method_id          TEXT        NOT NULL,
    months             INTEGER     NOT NULL,
    amount_kopecks     BIGINT      NOT NULL,
    pending_payment_id TEXT        NOT NULL DEFAULT '',
    pending_target     TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
)`

	_, err := s.db.ExecContext(ctx, q)

	return err
}

func (s *store) Save(ctx context.Context, payment record) error {
	const q = `
	INSERT INTO autopayments (username, method_id, months, amount_kopecks)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (username) DO UPDATE
	SET method_id = EXCLUDED.method_id,
		months = EXCLUDED.months,
		amount_kopecks = EXCLUDED.amount_kopecks,
		updated_at = now()`

	_, err := s.db.ExecContext(
		ctx,
		q,
		payment.Username,
		payment.MethodID,
		payment.Months,
		payment.AmountKopecks,
	)

	return err
}

func (s *store) List(ctx context.Context) (out []record, err error) {
	const q = `
	SELECT username, method_id, months, amount_kopecks,
		pending_payment_id, pending_target
	FROM autopayments
	ORDER BY username`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	for rows.Next() {
		var (
			payment record
			target  sql.NullTime
		)

		if err := rows.Scan(
			&payment.Username,
			&payment.MethodID,
			&payment.Months,
			&payment.AmountKopecks,
			&payment.PendingPaymentID,
			&target,
		); err != nil {
			return nil, err
		}

		if target.Valid {
			payment.PendingTarget = target.Time
		}

		out = append(out, payment)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *store) SetPending(
	ctx context.Context,
	username string,
	paymentID string,
	target time.Time,
) error {
	const q = `
	UPDATE autopayments
	SET pending_payment_id = $2, pending_target = $3, updated_at = now()
	WHERE username = $1`

	_, err := s.db.ExecContext(ctx, q, username, paymentID, target)

	return err
}

func (s *store) ClearPending(ctx context.Context, username string) error {
	const q = `
	UPDATE autopayments
	SET pending_payment_id = '', pending_target = NULL, updated_at = now()
	WHERE username = $1`

	_, err := s.db.ExecContext(ctx, q, username)

	return err
}

func (s *store) Delete(ctx context.Context, username string) error {
	const q = `DELETE FROM autopayments WHERE username = $1`

	_, err := s.db.ExecContext(ctx, q, username)

	return err
}
