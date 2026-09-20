package remnawave

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	platformremnawave "github.com/VladMallory/ProxyMaster_v2/internal/platform/remnawave"
	subdomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/domain"
)

func (r *RemnawaveAdapter) CreateUser(
	ctx context.Context,
	username string,
	days int,
) (subdomain.User, error) {
	now := time.Now()

	user := subdomain.CreateUserRequest{
		Username:             username,
		Status:               "ACTIVE",
		UUID:                 uuid.NewString(),
		VLESSUUID:            uuid.NewString(),
		TrojanPassword:       strings.ReplaceAll(uuid.NewString(), "-", "")[:12],
		SSPassword:           strings.ReplaceAll(uuid.NewString(), "-", "")[:12],
		TrafficLimitBytes:    0,
		TrafficLimitStrategy: "MONTH",
		ExpireAt:             now.AddDate(0, 0, days),
		CreatedAt:            now,
		LastTrafficResetAt:   now.Format(time.RFC3339),
		ActiveInternalSquads: []string{},
	}

	path := "/api/users?" + r.apiKey

	resp, err := platformremnawave.Do[subdomain.APIResponse](
		ctx,
		r.client,
		http.MethodPost,
		path,
		user,
	)
	if err != nil {
		return subdomain.User{}, err
	}

	return subdomain.User{
		Name:     resp.UserResponse.Username,
		UUID:     resp.UserResponse.UUID,
		Days:     days,
		Device:   resp.UserResponse.HWIDDeviceLimit,
		URL:      resp.UserResponse.SubscriptionURL,
		ExpireAt: now,
	}, nil
}

func (r RemnawaveAdapter) GetByUsername(
	ctx context.Context,
	username string,
) (subdomain.UserResponse, error) {
	path := "/api/users/by-username/" + username + "?" + r.apiKey

	resp, err := platformremnawave.Do[subdomain.APIResponse](
		ctx,
		r.client,
		http.MethodGet,
		path,
		nil,
	)
	if err != nil {
		return subdomain.UserResponse{}, err
	}

	return resp.UserResponse, nil
}

// GetUUIDByUsername — возвращает идентификатор пользователя по имени.
// Старый API отдавал uuid, новый — только числовой id.
// Возвращаем строку: на старой панели это uuid, на новой — id строкой.
// Вызывать только там, где идентификатор реально нужен.
func (r RemnawaveAdapter) GetUUIDByUsername(
	ctx context.Context,
	username string,
) (string, error) {
	resp, err := r.GetByUsername(ctx, username)
	if err != nil {
		return "", err
	}

	if resp.UUID != "" {
		return resp.UUID, nil
	}

	return strconv.Itoa(resp.ID), nil
}

func (r RemnawaveAdapter) GetByUUID(
	ctx context.Context,
	uuid string,
) (subdomain.UserResponse, error) {
	path := "/api/users/" + uuid + "?" + r.apiKey

	resp, err := platformremnawave.Do[subdomain.APIResponse](
		ctx,
		r.client,
		http.MethodGet,
		path,
		nil,
	)
	if err != nil {
		return subdomain.UserResponse{}, err
	}

	return resp.UserResponse, nil
}

// ExtendExpire продлевает существующего клиента в Remnawave по uuid.
func (r *RemnawaveAdapter) ExtendExpire(
	ctx context.Context,
	uuid string,
	expireAt time.Time,
) error {
	id, err := strconv.Atoi(uuid)
	if err != nil {
		return err
	}

	path := "/api/users/?" + r.apiKey

	body := map[string]any{
		"id":       id,
		"expireAt": expireAt.Format(time.RFC3339),
	}

	_, err = platformremnawave.Do[subdomain.APIResponse](
		ctx,
		r.client,
		http.MethodPatch,
		path,
		body,
	)
	if err != nil {
		return err
	}

	return err
}
