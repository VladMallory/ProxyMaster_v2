package remnawave

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	subdomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/domain"
)

func (r *RemnawaveClient) CreateUser(
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

	resp, err := doRequest[subdomain.APIResponse](
		ctx,
		r.client,
		r.baseURL,
		r.token,
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

func (r RemnawaveClient) GetByUsername(
	ctx context.Context,
	username string,
) (subdomain.UserResponse, error) {
	path := "/api/users/by-username/" + username + "?" + r.apiKey

	resp, err := doRequest[subdomain.APIResponse](
		ctx,
		r.client,
		r.baseURL,
		r.token,
		http.MethodGet,
		path,
		nil,
	)
	if err != nil {
		if errors.Is(err, subdomain.ErrNoFindUser) {
			return subdomain.UserResponse{}, subdomain.ErrNoFindUser
		}

		return subdomain.UserResponse{}, err
	}

	return resp.UserResponse, nil
}

// GetUUIDByUsername — возвращает идентификатор пользователя по имени.
// Старый API отдавал uuid, новый — только числовой id.
// Возвращаем строку: на старой панели это uuid, на новой — id строкой.
// Вызывать только там, где идентификатор реально нужен.
func (r RemnawaveClient) GetUUIDByUsername(
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

func (r RemnawaveClient) GetByUUID(
	ctx context.Context,
	uuid string,
) (subdomain.UserResponse, error) {
	path := "/api/users/" + uuid + "?" + r.apiKey

	resp, err := doRequest[subdomain.APIResponse](
		ctx,
		r.client,
		r.baseURL,
		r.token,
		http.MethodGet,
		path,
		nil,
	)
	if err != nil {
		if errors.Is(err, subdomain.ErrNoFindUser) {
			return subdomain.UserResponse{}, subdomain.ErrNoFindUser
		}

		return subdomain.UserResponse{}, err
	}

	return resp.UserResponse, nil
}

// UpdateExpireAt двигает expireAt у существующего юзера.
// modify-user в Remnawave (PUT /api/users) требует полную сущность,
// поэтому сначала вычитываем текущего юзера и правим только дату.
func (r RemnawaveClient) UpdateExpireAt(
	ctx context.Context,
	username string,
	expireAt time.Time,
) error {
	// Берём текущее состояние, иначе панель не примет запрос.
	cur, err := r.GetByUsername(ctx, username)
	if err != nil {
		return err
	}

	// Тот же DTO, что при создании: контракт у панели единый.
	update := subdomain.CreateUserRequest{
		Username:             cur.Username,
		Status:               cur.Status,
		UUID:                 cur.UUID,
		VLESSUUID:            cur.VLESSUUID,
		TrojanPassword:       cur.TrojanPassword,
		SSPassword:           cur.SSPassword,
		TrafficLimitBytes:    cur.TrafficLimitBytes,
		TrafficLimitStrategy: cur.TrafficLimitStrategy,
		ExpireAt:             expireAt,
		CreatedAt:            time.Now(),
		LastTrafficResetAt:   cur.LastTrafficResetAt,
		Description:          cur.Description,
		Tag:                  cur.Tag,
		TelegramID:           cur.TelegramID,
		Email:                cur.Email,
		HWIDDeviceLimit:      cur.HWIDDeviceLimit,
		ActiveInternalSquads: activeSquads(cur.ActiveInternalSquads),
		ExternalSquadUUID:    cur.ExternalSquadUUID,
	}

	path := "/api/users?" + r.apiKey

	_, err = doRequest[subdomain.APIResponse](
		ctx,
		r.client,
		r.baseURL,
		r.token,
		http.MethodPut,
		path,
		update,
	)

	return err
}

// activeSquads приводит ответ панели ([]any) к типу запроса ([]string),
// т.к. панель может вернуть как список строк, так и null.
func activeSquads(raw []any) []string {
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}

	return out
}
