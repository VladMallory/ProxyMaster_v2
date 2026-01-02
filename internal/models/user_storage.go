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
	// Ставим * для надежности. Чтобы передавали значения через &
	// Иначе не указав явно Balance он бы при каждом запросе
	// ставился бы на 0. А так он остается таким же, как был.
	// Потому что ставится nil, если значение не указано.
	Balance *int
	Trial   *bool
}
