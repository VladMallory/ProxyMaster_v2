package app

import (
	"ProxyMaster_v2/internal/domain"
	"context"
)

type App struct {
	remnawaveClient domain.RemnawaveClient
}

func New() (*App, error) {
	// cfg, err := config.New()
	// if err != nil {
	// 	return nil, err
	// }

	// TODO: реализация с api remnawave

	//

	return &App{}, nil
}

func (a *App) Run() {
	ctx := context.Background()

	a.remnawaveClient.GetServiceInfo(ctx, "")
}
