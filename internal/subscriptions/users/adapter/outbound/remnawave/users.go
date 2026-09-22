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
		return subdomain.User{}, r.notifierAdmin.Map(ctx, err, ErrorMeta{
			Op:       "CreateUser",
			Username: username,
		})
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

// GetByUsername получение информации о пользователе через username.
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
		return subdomain.UserResponse{}, r.notifierAdmin.Map(
			ctx,
			err,
			ErrorMeta{
				Op:       "GetByUsername",
				Username: username,
			},
		)
	}

	return resp.UserResponse, nil
}

// GetByID получение информации о пользователе через ID.
func (r RemnawaveAdapter) GetByID(
	ctx context.Context,
	userID int,
) (subdomain.UserResponse, error) {
	path := "/api/users/" + strconv.Itoa(userID) + "?" + r.apiKey

	resp, err := platformremnawave.Do[subdomain.APIResponse](
		ctx,
		r.client,
		http.MethodGet,
		path,
		nil,
	)
	if err != nil {
		return subdomain.UserResponse{}, r.notifierAdmin.Map(
			ctx,
			err,
			ErrorMeta{Op: "GetByID"},
		)
	}

	return resp.UserResponse, nil
}

// ExtendExpire продлевает пользователя в Remnawave.
func (r *RemnawaveAdapter) ExtendExpire(
	ctx context.Context,
	userID int,
	expireAt time.Time,
) error {
	path := "/api/users/?" + r.apiKey

	body := map[string]any{
		"id":       userID,
		"expireAt": expireAt.Format(time.RFC3339),
	}

	_, err := platformremnawave.Do[subdomain.APIResponse](
		ctx,
		r.client,
		http.MethodPatch,
		path,
		body,
	)
	if err != nil {
		return r.notifierAdmin.Map(
			ctx,
			err,
			ErrorMeta{Op: "ExtendExpire"},
		)
	}

	return nil
}
