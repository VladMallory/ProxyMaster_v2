package main

// структура для запроса в апи remanwave
type BulkExtendRequest struct {
	UUIDs []string `json:"uuids"`
	Days  int      `json:"days"`
}

// LoginRequest описывает тело запроса для авторизации.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse описывает ответ сервера после успешной авторизации.
// Обновлено на основе реального ответа сервера: {"response": {"accessToken": "..."}}
type LoginResponse struct {
	Response struct {
		AccessToken string `json:"accessToken"`
	} `json:"response"`
}

// UsersResponse описывает структуру ответа API со списком пользователей.
type UsersResponse struct {
	Response struct {
		Total int    `json:"total"`
		Users []User `json:"users"`
	} `json:"response"`
}

// User описывает данные одного пользователя.
type User struct {
	ID          int         `json:"id"`
	Username    string      `json:"username"`
	Status      string      `json:"status"`
	UserTraffic UserTraffic `json:"userTraffic"`
	// Можно добавить остальные поля по необходимости
}

// UserTraffic описывает статистику трафика пользователя.
type UserTraffic struct {
	UsedTrafficBytes int64 `json:"usedTrafficBytes"`
}

// ==========================================
// Configuration (Слой конфигурации)
// ==========================================

// Config хранит настройки приложения.
// Используется для передачи зависимостей (Dependency Injection).

// мб переименовать в AppToken или конфиг панели /internal/config/config.go
// переименовать в PanelConfig т.к можно запутаться

type Config struct {
	BaseURL        string
	Login          string
	Pass           string
	SecretURLToken string
	APIToken       string // Токен для API запросов (из REMNA_TOKEN)
}
