package userscase

import (
	"context"
	"errors"
	"time"

	subdomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/domain"
)

type UserRepository interface {
	GetByUsername(ctx context.Context, username string) (subdomain.UserResponse, error)
	GetByUUID(ctx context.Context, uuid string) (subdomain.UserResponse, error)
	CreateUser(ctx context.Context, username string, days int) (subdomain.User, error)
}

type UserUseCase struct {
	repo               UserRepository
	defaultDeviceLimit int
}

func NewUserUseCase(repo UserRepository, defaultDeface int) UserUseCase {
	return UserUseCase{
		repo:               repo,
		defaultDeviceLimit: defaultDeface,
	}
}

// deviceCheck если панель возвращает 0 device, то задаем значение из env
// чтобы пользователю не писать "у вас устройств 0".
func (u UserUseCase) deviceCheck(defaultDeviceLimit int) int {
	if defaultDeviceLimit == 0 {
		return u.defaultDeviceLimit
	}

	return defaultDeviceLimit
}

// GetOrCreateSub получает или создает подписку пользователя.
func (u UserUseCase) GetOrCreateSub(
	ctx context.Context,
	username string,
	trialDays int,
) (subdomain.User, error) {
	resp, err := u.repo.GetByUsername(ctx, username)
	if errors.Is(err, subdomain.ErrNoFindUser) {
		createtUser, err := u.repo.CreateUser(ctx, username, trialDays)
		if err != nil {
			return subdomain.User{}, err
		}

		resp.HWIDDeviceLimit = u.deviceCheck(resp.HWIDDeviceLimit)

		return createtUser, nil
	}
	if err != nil {
		return subdomain.User{}, err
	}

	expireAt, err := time.Parse(time.RFC3339, resp.ExpireAt)
	if err != nil {
		return subdomain.User{}, err
	}

	return subdomain.User{
		Name:     resp.Username,
		UUID:     resp.UUID,
		Device:   u.deviceCheck(resp.HWIDDeviceLimit),
		URL:      resp.SubscriptionURL,
		Days:     remainingDays(resp.ExpireAt),
		ExpireAt: expireAt,
	}, nil
}

// GetURL получает URL подписки пользователя.
func (u UserUseCase) GetURL(ctx context.Context, username string) (subdomain.User, error) {
	resp, err := u.repo.GetByUsername(ctx, username)
	if err != nil {
		return subdomain.User{}, err
	}

	return subdomain.User{
		Name:   resp.Username,
		UUID:   resp.UUID,
		Device: u.deviceCheck(resp.HWIDDeviceLimit),
		URL:    resp.SubscriptionURL,
	}, nil
}

// remainingDays считает сколько дней подписки осталось.
func remainingDays(expireAt string) int {
	t, err := time.Parse(time.RFC3339, expireAt)
	if err != nil {
		return 0
	}

	return int(time.Until(t).Hours() / 24)
}
