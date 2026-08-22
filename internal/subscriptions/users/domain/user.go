package subdomain

import (
	"errors"
	"time"
)

var ErrNoFindUser = errors.New("пользователь не найден")

type User struct {
	Name     string
	UUID     string
	Days     int
	Device   int
	URL      string
	ExpireAt time.Time
}

/*
Вот так прошел успешный запрос на создание
{
  "username": "test12",
  "status": "ACTIVE",
  "shortUuid": "",
  "trojanPassword": "Kx9mP2nQwR7t",
  "vlessUuid": "a3f7c2d1-84e6-4b19-9f05-2c3d7e8a1b4f",
  "ssPassword": "Tz5vL8jNhY3s",
  "trafficLimitBytes": 0,
  "trafficLimitStrategy": "NO_RESET",
  "expireAt": "2026-07-26T12:54:44.236Z",
  "createdAt": "2026-07-25T12:54:44.236Z",
  "lastTrafficResetAt": "2026-07-25T12:54:44.236Z",
  "description": "",
  "tag": null,
  "telegramId": null,
  "email": null,
  "hwidDeviceLimit": 0,
  "activeInternalSquads": [],
  "uuid": "e1b4d9c6-3f72-4a08-b5e1-7d2f9c0a6e83",
  "externalSquadUuid": null
}
*/

type CreateUserRequest struct {
	Username             string    `json:"username"`
	Status               string    `json:"status"`
	ShortUUID            string    `json:"shortUuid"`
	TrojanPassword       string    `json:"trojanPassword"`
	VLESSUUID            string    `json:"vlessUuid"`
	SSPassword           string    `json:"ssPassword"`
	TrafficLimitBytes    int64     `json:"trafficLimitBytes"`
	TrafficLimitStrategy string    `json:"trafficLimitStrategy"`
	ExpireAt             time.Time `json:"expireAt"`
	CreatedAt            time.Time `json:"createdAt"`
	LastTrafficResetAt   string    `json:"lastTrafficResetAt"`
	Description          string    `json:"description"`
	Tag                  *string   `json:"tag"`
	TelegramID           *int64    `json:"telegramId"`
	Email                *string   `json:"email"`
	HWIDDeviceLimit      int       `json:"hwidDeviceLimit"`
	ActiveInternalSquads []string  `json:"activeInternalSquads"`
	UUID                 string    `json:"uuid"`
	ExternalSquadUUID    *string   `json:"externalSquadUuid"`
}

// --- get uuid ---.

// APIResponse запрос на получение информации о пользователе по UUID.
type APIResponse struct {
	UserResponse UserResponse `json:"response"`
}

type UserResponse struct {
	UUID                   string      `json:"uuid"`
	ID                     int         `json:"id"`
	ShortUUID              string      `json:"shortUuid"`
	Username               string      `json:"username"`
	Status                 string      `json:"status"`
	TrafficLimitBytes      int64       `json:"trafficLimitBytes"`
	TrafficLimitStrategy   string      `json:"trafficLimitStrategy"`
	ExpireAt               string      `json:"expireAt"`
	TelegramID             *int64      `json:"telegramId"`
	Email                  *string     `json:"email"`
	Description            string      `json:"description"`
	Tag                    *string     `json:"tag"`
	HWIDDeviceLimit        int         `json:"hwidDeviceLimit"`
	ExternalSquadUUID      *string     `json:"externalSquadUuid"`
	TrojanPassword         string      `json:"trojanPassword"`
	VLESSUUID              string      `json:"vlessUuid"`
	SSPassword             string      `json:"ssPassword"`
	LastTriggeredThreshold int         `json:"lastTriggeredThreshold"`
	SubRevokedAt           *string     `json:"subRevokedAt"`
	LastTrafficResetAt     string      `json:"lastTrafficResetAt"`
	CreatedAt              string      `json:"createdAt"`
	UpdatedAt              string      `json:"updatedAt"`
	SubscriptionURL        string      `json:"subscriptionUrl"`
	ActiveInternalSquads   []any       `json:"activeInternalSquads"`
	UserTraffic            UserTraffic `json:"userTraffic"`
}

type UserTraffic struct {
	UsedTrafficBytes         int64   `json:"usedTrafficBytes"`
	LifetimeUsedTrafficBytes int64   `json:"lifetimeUsedTrafficBytes"`
	OnlineAt                 *string `json:"onlineAt"`
	LastConnectedNodeUUID    *string `json:"lastConnectedNodeUuid"`
	FirstConnectedAt         *string `json:"firstConnectedAt"`
}
