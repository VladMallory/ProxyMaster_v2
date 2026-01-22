package models

import "time"

// Title - название squad, например bs, yandexServer и тд
// UserID - FOREIGHT KEY id пользователя
// UUID - айди squad
// expiresAt - дата истечения squad
type Squad struct {
	Title     string
	UserID    string
	UUID      string
	ExpiresAt time.Time
}
