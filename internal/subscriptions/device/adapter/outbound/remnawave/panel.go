package deviceremnawave

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	platformremnawave "github.com/VladMallory/ProxyMaster_v2/internal/platform/remnawave"
	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device"
)

type Panel struct {
	client *platformremnawave.Client
	apiKey string
}

func NewPanel(client *platformremnawave.Client, apiKey string) *Panel {
	return &Panel{
		client: client,
		apiKey: apiKey,
	}
}

var _ device.Panel = (*Panel)(nil)

func (p *Panel) path(raw string) string {
	if p.apiKey == "" {
		return raw
	}

	return raw + "?" + p.apiKey
}

type panelUser struct {
	Response struct {
		ID              int    `json:"id"`
		ExpireAt        string `json:"expireAt"`
		HWIDDeviceLimit int    `json:"hwidDeviceLimit"`
	} `json:"response"`
}

func (p *Panel) Subscription(
	ctx context.Context,
	username string,
) (device.PanelSubscription, error) {
	path := p.path("/api/users/by-username/" + url.PathEscape(username))

	resp, err := platformremnawave.Do[panelUser](
		ctx,
		p.client,
		http.MethodGet,
		path,
		nil,
	)
	if errors.Is(err, platformremnawave.ErrNotFound) {
		return device.PanelSubscription{}, fmt.Errorf(
			"remnawave subscription %q: %w",
			username,
			device.ErrPanelNotFound,
		)
	}
	if err != nil {
		return device.PanelSubscription{}, fmt.Errorf(
			"remnawave subscription %q: %w",
			username,
			err,
		)
	}

	expireAt, err := time.Parse(time.RFC3339Nano, resp.Response.ExpireAt)
	if err != nil {
		return device.PanelSubscription{}, fmt.Errorf(
			"remnawave subscription %q: expireAt: %w",
			username,
			err,
		)
	}

	return device.PanelSubscription{
		UserID:      resp.Response.ID,
		ExpireAt:    expireAt,
		DeviceLimit: resp.Response.HWIDDeviceLimit,
	}, nil
}

type panelHWIDs struct {
	Response struct {
		Devices []struct {
			HWID        string `json:"hwid"`
			Platform    string `json:"platform"`
			OSVersion   string `json:"osVersion"`
			DeviceModel string `json:"deviceModel"`
		} `json:"devices"`
	} `json:"response"`
}

// ListDevices читает HWID-устройства пользователя и собирает читаемое имя
// из панели: модель устройства или платформа с версией ОС.
func (p *Panel) ListDevices(ctx context.Context, userID int) ([]device.PanelDevice, error) {
	path := p.path("/api/hwid/devices/" + strconv.Itoa(userID))

	resp, err := platformremnawave.Do[panelHWIDs](
		ctx,
		p.client,
		http.MethodGet,
		path,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("remnawave hwid user %d: %w", userID, err)
	}

	devices := make([]device.PanelDevice, 0, len(resp.Response.Devices))
	seen := make(map[string]struct{}, len(resp.Response.Devices))

	for _, d := range resp.Response.Devices {
		if d.HWID == "" {
			continue
		}
		if _, ok := seen[d.HWID]; ok {
			continue
		}

		seen[d.HWID] = struct{}{}
		devices = append(devices, device.PanelDevice{
			HWID: d.HWID,
			Name: deviceName(d.DeviceModel, d.Platform, d.OSVersion),
		})
	}

	return devices, nil
}

// deviceName собирает читаемое имя из необязательных полей панели.
func deviceName(model, platform, osVersion string) string {
	if model != "" {
		return model
	}

	name := strings.TrimSpace(platform + " " + osVersion)
	if name != "" {
		return name
	}

	return ""
}

type deleteHWIDBody struct {
	UserID int    `json:"userId"`
	HWID   string `json:"hwid"`
}

func (p *Panel) DeleteHWID(
	ctx context.Context,
	userID int,
	hwid string,
) error {
	_, err := platformremnawave.Do[struct{}](
		ctx,
		p.client,
		http.MethodPost,
		p.path("/api/hwid/devices/delete"),
		deleteHWIDBody{
			UserID: userID,
			HWID:   hwid,
		},
	)
	if errors.Is(err, platformremnawave.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("remnawave delete hwid user %d: %w", userID, err)
	}

	return nil
}

type updateUserBody struct {
	ID              int    `json:"id"`
	ExpireAt        string `json:"expireAt"`
	HWIDDeviceLimit int    `json:"hwidDeviceLimit"`
}

func (p *Panel) UpdateUser(
	ctx context.Context,
	userID int,
	expireAt time.Time,
	deviceLimit int,
) error {
	body := updateUserBody{
		ID:              userID,
		ExpireAt:        expireAt.UTC().Format(time.RFC3339),
		HWIDDeviceLimit: deviceLimit,
	}

	_, err := platformremnawave.Do[struct{}](
		ctx,
		p.client,
		http.MethodPatch,
		p.path("/api/users"),
		body,
	)
	if err != nil {
		return fmt.Errorf("remnawave update user %d: %w", userID, err)
	}

	return nil
}
