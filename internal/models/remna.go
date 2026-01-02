package models

import "time"

// BulkExtendRequest структура для запроса в апи remnawave
type BulkExtendRequest struct {
	UUIDs []string `json:"uuids"`
	Days  int      `json:"extendDays"`
}

type UpdateUserRequest struct {
	Username             *string    `json:"username,omitempty"`
	Uuid                 *string    `json:"uuid,omitempty"`
	Status               *string    `json:"status,omitempty"`                     // ACTIVE, INACTIVE и т.д.
	TrafficLimitBytes    *uint64    `json:"trafficLimitBytes,omitempty"`          // Лимит трафика в байтах
	TrafficLimitStrategy *string    `json:"trafficLimitStrategy,omitempty"`       // NO_RESET, MONTHLY_RESET и т.д.
	ExpireAt             *time.Time `json:"expireAt,omitempty"`                   // Дата истечения срока действия
	Description          *string    `json:"description,omitempty"`                // Описание пользователя
	Tag                  *string    `json:"tag,omitempty"`                        // Метка или категория пользователя
	TelegramId           *string    `json:"telegramId,omitempty"`                 // ID Телеграм-аккаунта
	Email                *string    `json:"email,omitempty"`                      // Email пользователя
	HwidDeviceLimit      *uint8     `json:"hwidDeviceLimit,omitempty"`            // Ограничение устройств по HWID
	ActiveInternalSquads []string   `json:"activeInternalSquads,omitempty"`       // Активные внутренние группы/секции
	ExternalSquadUuid    *string    `json:"externalSquadUuid,omitempty"`          // Внешняя группа пользователей
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
		Users []user `json:"users"`
	} `json:"response"`
}

// User описывает данные одного пользователя.
type user struct {
	ID          int         `json:"id"`
	Username    string      `json:"username"`
	Status      string      `json:"status"`
	UserTraffic userTraffic `json:"userTraffic"`
	// Можно добавить остальные поля по необходимости
}

// ==========================================
// Configuration (Слой конфигурации)
// ==========================================

// Config хранит настройки приложения.
// Используется для передачи зависимостей (Dependency Injection).

// todo мб переименовать в AppToken или конфиг панели /internal/config/config.go
// todo переименовать в PanelConfig т.к можно запутаться

//legacy структура конфига
// type Config struct {
// 	BaseURL        string
// 	Login          string
// 	Pass           string
// 	SecretURLToken string
// 	APIToken       string // Токен для API запросов (из REMNA_TOKEN)
// }

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
	ActiveInternalSquads []string `json:"activeInternalSquads"`
	// UUID                 string   `json:"uuid"`
	// ExternalSquadUUID    *string  `json:"externalSquadUuid"` доп поля, хз нужны будут или нет
}

//
//
//
//

// ActiveInternalSquad Представляет активную внутреннюю группу
type ActiveInternalSquad struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

// UserTraffic Информация о трафике пользователя
type userTraffic struct {
	UsedTrafficBytes         uint64    `json:"usedTrafficBytes"`
	LifetimeUsedTrafficBytes uint64    `json:"lifetimeUsedTrafficBytes"`
	OnlineAt                 time.Time `json:"onlineAt"`
	LastConnectedNodeUUID    string    `json:"lastConnectedNodeUuid"`
	FirstConnectedAt         time.Time `json:"firstConnectedAt"`
}

// GetUUIDByUsernameResponse описывает ответ сервера с информацией о пользователе по его UUID.
type GetUUIDByUsernameResponse struct {
	Response struct {
		UUID                   string                `json:"uuid"`
		ID                     int                   `json:"id"`
		ShortUUID              string                `json:"shortUuid"`
		Username               string                `json:"username"`
		Status                 string                `json:"status"`
		TrafficLimitBytes      uint64                `json:"trafficLimitBytes"`
		TrafficLimitStrategy   string                `json:"trafficLimitStrategy"`
		ExpireAt               *time.Time            `json:"expireAt"`
		TelegramID             *string               `json:"telegramId,omitempty"`
		Email                  *string               `json:"email,omitempty"`
		Description            string                `json:"description"`
		Tag                    *string               `json:"tag,omitempty"`
		HWIDDeviceLimit        int                   `json:"hwidDeviceLimit"`
		ExternalSquadUUID      *string               `json:"externalSquadUuid,omitempty"`
		TrojanPassword         string                `json:"trojanPassword"`
		VLESSUUID              string                `json:"vlessUuid"`
		SSPassword             string                `json:"ssPassword"`
		LastTriggeredThreshold int                   `json:"lastTriggeredThreshold"`
		SubRevokedAt           *time.Time            `json:"subRevokedAt,omitempty"`
		SubLastUserAgent       string                `json:"subLastUserAgent"`
		SubLastOpenedAt        time.Time             `json:"subLastOpenedAt"`
		LastTrafficResetAt     *time.Time            `json:"lastTrafficResetAt,omitempty"`
		CreatedAt              time.Time             `json:"createdAt"`
		UpdatedAt              time.Time             `json:"updatedAt"`
		SubscriptionURL        string                `json:"subscriptionUrl"`
		ActiveInternalSquads   []ActiveInternalSquad `json:"activeInternalSquads"`
		UserTraffic            userTraffic           `json:"userTraffic"`
	} `json:"response"`
}

// GetUserInfoResponse Ответ
type GetUserInfoResponse struct {
	Response struct {
		UUID                   string                `json:"uuid"`
		ID                     int                   `json:"id"`
		ShortUUID              string                `json:"shortUuid"`
		Username               string                `json:"username"`
		Status                 string                `json:"status"`
		TrafficLimitBytes      int                   `json:"trafficLimitBytes"`
		TrafficLimitStrategy   string                `json:"trafficLimitStrategy"`
		ExpireAt               time.Time             `json:"expireAt"`
		TelegramID             *string               `json:"telegramId,omitempty"` // nullable field
		Email                  *string               `json:"email,omitempty"`      // nullable field
		Description            string                `json:"description"`
		Tag                    *string               `json:"tag,omitempty"` // nullable field
		HWIDDeviceLimit        int                   `json:"hwidDeviceLimit"`
		ExternalSquadUUID      *string               `json:"externalSquadUuid,omitempty"` // nullable field
		TrojanPassword         string                `json:"trojanPassword"`
		VlessUUID              string                `json:"vlessUuid"`
		SsPassword             string                `json:"ssPassword"`
		LastTriggeredThreshold int                   `json:"lastTriggeredThreshold"`
		SubRevokedAt           time.Time             `json:"subRevokedAt"`
		SubLastUserAgent       string                `json:"subLastUserAgent"`
		SubLastOpenedAt        time.Time             `json:"subLastOpenedAt"`
		LastTrafficResetAt     *time.Time            `json:"lastTrafficResetAt,omitempty"` // nullable field
		CreatedAt              time.Time             `json:"createdAt"`
		UpdatedAt              time.Time             `json:"updatedAt"`
		SubscriptionURL        string                `json:"subscriptionUrl"`
		ActiveInternalSquads   []ActiveInternalSquad `json:"activeInternalSquads"`
		UserTraffic            userTraffic           `json:"userTraffic"`
	} `json:"response"`
}

//
//
//
/*
хз нужно ли будет ответ сохранять
 	type CreateResponseUserDTO struct {
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
*/
