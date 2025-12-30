package models

import "time"

type UserTG struct {
	ID        string    `db:"id"`
	Balance   float64   `db:"balance"`
	Trial     bool      `db:"trial"`
	CreatedAt time.Time `db:"created_at"`
}

type CreateUserTGDTO struct {
	ID      string  `db:"id"`
	Balance float64 `db:"balance"`
	Trial   bool    `db:"trial"`
}

type UpdateUserTGDTO struct {
	Balance *float64
	Trial   *bool
}
