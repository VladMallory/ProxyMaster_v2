package db

import "errors"

var (
	ErrDuplicateKey       = errors.New("duplicate key")
	ErrDatabaseConnection = errors.New("database connection error")
)
