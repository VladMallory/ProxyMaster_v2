package models

import "time"

type UserTG struct {
	ID        string    `db:"id"`
	Balance   int       `db:"balance"`
	Trial     bool      `db:"trial"`
	CreatedAt time.Time `db:"created_at"`
}

type CreateUserTGDTO struct {
	ID      string `db:"id"`
	Balance int    `db:"balance"`
	Trial   bool   `db:"trial"`
}

type UpdateUserTGDTO struct {
	Balance *int
	Trial   *bool
}
