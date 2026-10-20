package devicepostgres

import (
	"context"
	"database/sql"

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
	SELECT id, COALESCE(label, ''), COALESCE(hwid, '')
	FROM devices
	WHERE username = $1 AND revoked_at IS NULL
	ORDER BY id`

	rows, err := r.db.QueryContext(ctx, q, username)
	if err != nil {
		return nil, err
	}
	defer closer.HelpClose(rows, &err)

	for rows.Next() {
		var d device.Device

		if err := rows.Scan(&d.ID, &d.Name, &d.HWID); err != nil {
			return nil, err
		}

		out = append(out, d)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

var _ device.Repository = (*Repository)(nil)
