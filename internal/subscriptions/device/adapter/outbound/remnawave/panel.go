package remnawavedevice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	platformremnawave "github.com/VladMallory/ProxyMaster_v2/internal/platform/remnawave"
	devicedomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device"
)

// Проверяем на этапе компиляции.
var _ devicedomain.Panel = (*PanelAdapter)(nil)

type PanelAdapter struct {
	client *platformremnawave.Client
	apiKey string
}

func New(client *platformremnawave.Client, apiKey string) *PanelAdapter {
	return &PanelAdapter{client: client, apiKey: apiKey}
}

// userRefResponse то, что возвращает GET /api/users/by-username/{username}.
type userRefResponse struct {
	Response struct {
		ID              int    `json:"id"`
		Username        string `json:"username"`
		HWIDDeviceLimit int    `json:"hwidDeviceLimit"`
		ExpireAt        string `json:"expireAt"`
	} `json:"response"`
}

type hwidDeviceDTO struct {
	HWID        string  `json:"hwid"`
	Platform    *string `json:"platform"`
	OSVersion   *string `json:"osVersion"`
	DeviceModel *string `json:"deviceModel"`
}

// hwidListResponse GET /api/hwid/devices/{userUuid}.
type hwidListResponse struct {
	Response json.RawMessage `json:"response"`
}

// resolveUser username -> числовой userID через панель.
// В панели v3 uuid удалён, все пути принимают именно id.
func (a *PanelAdapter) resolveUser(
	ctx context.Context,
	username string,
) (userID int, err error) {
	path := "/api/users/by-username/" + url.PathEscape(username) + "?" + a.apiKey

	resp, err := platformremnawave.Do[userRefResponse](
		ctx,
		a.client,
		http.MethodGet,
		path,
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("remnawave resolve %q: %w", username, err)
	}

	if resp.Response.ID == 0 {
		return 0, fmt.Errorf("remnawave resolve %q: пустой user id", username)
	}

	return resp.Response.ID, nil
}

func parseDevices(raw json.RawMessage) ([]hwidDeviceDTO, error) {
	var obj struct {
		Devices []hwidDeviceDTO `json:"devices"`
		Device  []hwidDeviceDTO `json:"device"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		if obj.Devices != nil {
			return obj.Devices, nil
		}

		if obj.Device != nil {
			return obj.Device, nil
		}
	}

	var arr []hwidDeviceDTO
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("неизвестный формат hwid devices: %s", string(raw))
	}

	return arr, nil
}

func strVal(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

func (a *PanelAdapter) ListDevices(
	ctx context.Context,
	username string,
) ([]devicedomain.PanelDevice, error) {
	userID, err := a.resolveUser(ctx, username)
	if err != nil {
		return nil, err
	}
	path := "/api/hwid/devices/" + strconv.Itoa(userID) + "?" + a.apiKey

	resp, err := platformremnawave.Do[hwidListResponse](
		ctx,
		a.client,
		http.MethodGet,
		path,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("remnawave ListDevices %q: %w", username, err)
	}

	dtos, err := parseDevices(resp.Response)
	if err != nil {
		return nil, fmt.Errorf("remnawave ListDevices %q: %w", username, err)
	}

	out := make([]devicedomain.PanelDevice, 0, len(dtos))
	for _, d := range dtos {
		if d.HWID == "" {
			continue
		}

		out = append(out, devicedomain.PanelDevice{
			HWID:        d.HWID,
			Platform:    strVal(d.Platform),
			DeviceModel: strVal(d.DeviceModel),
		})
	}

	return out, nil
}

// Subscription читает текущий лимит устройств и срок подписки из панели.
// Нужно для ленивого импорта мест у пользователей, заведённых до перехода на
// модель «каждое устройство отдельно».
func (a *PanelAdapter) Subscription(
	ctx context.Context,
	username string,
) (devicedomain.PanelSubscription, error) {
	path := "/api/users/by-username/" + url.PathEscape(username) + "?" + a.apiKey

	resp, err := platformremnawave.Do[userRefResponse](
		ctx,
		a.client,
		http.MethodGet,
		path,
		nil,
	)
	if err != nil {
		return devicedomain.PanelSubscription{}, fmt.Errorf(
			"remnawave Subscription %q: %w",
			username,
			err,
		)
	}

	// Панель отдаёт expireAt в RFC3339 (time.Parse принимает и дробные секунды).
	var expireAt time.Time
	if resp.Response.ExpireAt != "" {
		expireAt, err = time.Parse(time.RFC3339, resp.Response.ExpireAt)
		if err != nil {
			return devicedomain.PanelSubscription{}, fmt.Errorf(
				"remnawave Subscription %q expireAt: %w",
				username,
				err,
			)
		}
	}

	return devicedomain.PanelSubscription{
		Limit:    resp.Response.HWIDDeviceLimit,
		ExpireAt: expireAt,
	}, nil
}

// DeleteDevice удаляет устройство hwid у username.
// POST /api/hwid/devices/delete {userId, hwid}.
func (a *PanelAdapter) DeleteDevice(ctx context.Context, username, hwid string) error {
	userID, err := a.resolveUser(ctx, username)
	if err != nil {
		return err
	}

	path := "/api/hwid/devices/delete?" + a.apiKey

	body := map[string]any{
		"userId": userID,
		"hwid":   hwid,
	}

	if _, err := platformremnawave.Do[hwidListResponse](
		ctx,
		a.client,
		http.MethodPost,
		path,
		body,
	); err != nil {
		return fmt.Errorf("remnawave DeleteDevice %q: %w", username, err)
	}

	return nil
}

// UpdateSubscription приводит панель к БД: лимит устройств + срок подписки.
// PATCH /api/users/ {id, hwidDeviceLimit?, expireAt}.
// limit == 0 значит подписка истекла лимит не трогаем, шлём только expireAt.
func (a *PanelAdapter) UpdateSubscription(
	ctx context.Context,
	username string,
	limit int,
	expireAt time.Time,
) error {
	userID, err := a.resolveUser(ctx, username)
	if err != nil {
		return err
	}

	if userID == 0 {
		return fmt.Errorf("remnawave UpdateSubscription %q: пустой user id", username)
	}

	body := map[string]any{
		"id":       userID,
		"expireAt": expireAt.UTC().Format(time.RFC3339),
	}

	if limit > 0 {
		body["hwidDeviceLimit"] = limit
	}

	path := "/api/users/?" + a.apiKey

	if _, err := platformremnawave.Do[userRefResponse](
		ctx,
		a.client,
		http.MethodPatch,
		path,
		body,
	); err != nil {
		return fmt.Errorf("remnawave UpdateSubscription %q: %w", username, err)
	}

	return nil
}
