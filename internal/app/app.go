package app

import (
	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/infrastructure/remnawave"
	"context"
	"fmt"
)

type App struct {
	remnawaveClient domain.RemnawaveClient
}

func New() (*App, error) {
	cfg, err := config.New()
	if err != nil {
		return nil, err
	}

	// baseURL := cfg.RemnaPanelURL

	remnawaveClient := remnawave.NewRemnaClient(cfg)
	// Если есть логин и пароль, пробуем получить свежий токен
	if err := remnawaveClient.Login(context.Background(), cfg.RemnaLogin, cfg.RemnaPass); err != nil {
		fmt.Printf("Ошибка входа (используем старый токен): %v\n", err)
	}

	return &App{
		remnawaveClient: remnawaveClient,
	}, nil
}

func (a *App) Run() {
	ctx := context.Background()

	a.remnawaveClient.GetServiceInfo(ctx, "")
}
