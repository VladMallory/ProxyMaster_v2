package models

import "time"

// структура для запроса в апи remanwave
type BulkExtendRequest struct {
	UUIDs []string `json:"uuids"`
	Days  int      `json:"extendDays"`
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

type CreateRequestUserDTO struct {
	Username             string `json:"username"`
	Status               string `json:"status"`
	ShortUUID            string `json:"shortUuid"`
	TrojanPassword       string `json:"trojanPassword"`
	VLessUUID            string `json:"vlessUuid"`
	SsPassword           string `json:"ssPassword"`
	TrafficLimitBytes    int    `json:"trafficLimitBytes"`
	TrafficLimitStrategy string `json:"trafficLimitStrategy"`
	ExpireAt             string `json:"expireAt"`
	CreatedAt            string `json:"createdAt"`
	LastTrafficResetAt   string `json:"lastTrafficResetAt"`
	Description          string `json:"description"`
	// Tag                  *string  `json:"tag"`
	// TelegramID           *string  `json:"telegramId"`
	// Email                *string  `json:"email"`
	// HWIDDeviceLimit      int      `json:"hwidDeviceLimit"`
	// ActiveInternalSquads []string `json:"activeInternalSquads"`
	// UUID                 string   `json:"uuid"`
	// ExternalSquadUUID    *string  `json:"externalSquadUuid"` доп поля, хз нужны будут или нет
}

type GetUUIDByUsernameResponse struct {
	UUID                   string      `json:"uuid"`
	ID                     int         `json:"id"`
	ShortUUID              string      `json:"shortUuid"`
	Username               string      `json:"username"`
	Status                 string      `json:"status"`
	TrafficLimitBytes      int         `json:"trafficLimitBytes"`
	TrafficLimitStrategy   string      `json:"trafficLimitStrategy"`
	ExpireAt               time.Time   `json:"expireAt"`
	TelegramID             interface{} `json:"telegramId"`
	Email                  interface{} `json:"email"`
	Description            interface{} `json:"description"`
	Tag                    interface{} `json:"tag"`
	HwidDeviceLimit        interface{} `json:"hwidDeviceLimit"`
	ExternalSquadUUID      interface{} `json:"externalSquadUuid"`
	TrojanPassword         string      `json:"trojanPassword"`
	VlessUUID              string      `json:"vlessUuid"`
	SsPassword             string      `json:"ssPassword"`
	LastTriggeredThreshold int         `json:"lastTriggeredThreshold"`
	SubRevokedAt           interface{} `json:"subRevokedAt"`
	SubLastUserAgent       interface{} `json:"subLastUserAgent"`
	SubLastOpenedAt        interface{} `json:"subLastOpenedAt"`
	LastTrafficResetAt     interface{} `json:"lastTrafficResetAt"`
	CreatedAt              time.Time   `json:"createdAt"`
	UpdatedAt              time.Time   `json:"updatedAt"`
	SubscriptionURL        string      `json:"subscriptionUrl"`
	ActiveInternalSquads   []struct {
		UUID string `json:"uuid"`
		Name string `json:"name"`
	} `json:"activeInternalSquads"`
	UserTraffic struct {
		UsedTrafficBytes         int         `json:"usedTrafficBytes"`
		LifetimeUsedTrafficBytes int         `json:"lifetimeUsedTrafficBytes"`
		OnlineAt                 interface{} `json:"onlineAt"`
		FirstConnectedAt         interface{} `json:"firstConnectedAt"`
		LastConnectedNodeUUID    interface{} `json:"lastConnectedNodeUuid"`
	} `json:"userTraffic"`
}

//хз нужно ли будет ответ сохранять
// type CreateResponseUserDTO struct {
// 	UUID                   string      `json:"uuid"`
// 	ID                     int         `json:"id"`
// 	ShortUUID              string      `json:"shortUuid"`
// 	Username               string      `json:"username"`
// 	Status                 string      `json:"status"`
// 	TrafficLimitBytes      int         `json:"trafficLimitBytes"`
// 	TrafficLimitStrategy   string      `json:"trafficLimitStrategy"`
// 	ExpireAt               time.Time   `json:"expireAt"`
// 	TelegramID             interface{} `json:"telegramId"`
// 	Email                  interface{} `json:"email"`
// 	Description            interface{} `json:"description"`
// 	Tag                    interface{} `json:"tag"`
// 	HwidDeviceLimit        interface{} `json:"hwidDeviceLimit"`
// 	ExternalSquadUUID      interface{} `json:"externalSquadUuid"`
// 	TrojanPassword         string      `json:"trojanPassword"`
// 	VlessUUID              string      `json:"vlessUuid"`
// 	SsPassword             string      `json:"ssPassword"`
// 	LastTriggeredThreshold int         `json:"lastTriggeredThreshold"`
// 	SubRevokedAt           interface{} `json:"subRevokedAt"`
// 	SubLastUserAgent       interface{} `json:"subLastUserAgent"`
// 	SubLastOpenedAt        interface{} `json:"subLastOpenedAt"`
// 	LastTrafficResetAt     interface{} `json:"lastTrafficResetAt"`
// 	CreatedAt              time.Time   `json:"createdAt"`
// 	UpdatedAt              time.Time   `json:"updatedAt"`
// 	SubscriptionURL        string      `json:"subscriptionUrl"`
// 	ActiveInternalSquads   []struct {
// 		UUID string `json:"uuid"`
// 		Name string `json:"name"`
// 	} `json:"activeInternalSquads"`
// 	UserTraffic struct {
// 		UsedTrafficBytes         int         `json:"usedTrafficBytes"`
// 		LifetimeUsedTrafficBytes int         `json:"lifetimeUsedTrafficBytes"`
// 		OnlineAt                 interface{} `json:"onlineAt"`
// 		FirstConnectedAt         interface{} `json:"firstConnectedAt"`
// 		LastConnectedNodeUUID    interface{} `json:"lastConnectedNodeUuid"`
// 	} `json:"userTraffic"`
// }
