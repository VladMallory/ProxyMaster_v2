package database

import (
	"log/slog"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func Connect(databaseURL string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", databaseURL)
	if err != nil {
		slog.Warn(
			"Failed db connectio",
			"error_message", err,
		)
		return nil, err
	}
	db.SetMaxOpenConns(15)
	db.SetMaxIdleConns(5)
	return db, nil
}

// func Connect(databaseURL string) (*sqlx.DB, error) {
// 	db, err := sqlx.Connect("postgres", databaseURL)
// 	if err != nil {
// 		return nil, err
// 	}
// 	db.SetMaxOpenConns(15)
// 	db.SetMaxIdleConns(5)
// 	return db, nil
// }
