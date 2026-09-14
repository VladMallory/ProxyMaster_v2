package telegram

import (
	"context"

	platformtg "github.com/VladMallory/ProxyMaster_v2/internal/platform/telegram"
	userscase "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/service"
)

type PlatformUserAdapter struct {
	uc userscase.UserUseCase
}

func NewPlatformUserAdapter(uc userscase.UserUseCase) PlatformUserAdapter {
	return PlatformUserAdapter{uc: uc}
}

func (a PlatformUserAdapter) GetOrCreateSub(
	ctx context.Context,
	username string,
	trialDays int,
) (platformtg.UserView, error) {
	u, err := a.uc.GetOrCreateSub(ctx, username, trialDays)
	if err != nil {
		return platformtg.UserView{}, err
	}

	return platformtg.UserView{
		Name:     u.Name,
		URL:      u.URL,
		Device:   u.Device,
		ExpireAt: u.ExpireAt,
	}, nil
}
