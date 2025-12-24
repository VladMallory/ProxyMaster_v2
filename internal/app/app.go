package app

import (
	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/infrastructure/remnawave"
	"ProxyMaster_v2/internal/models"
	"context"
)

type App struct {
	remnawaveClient domain.RemnawaveClient
}

func New() (*App, error) {
	cfg, err := config.New()
	if err != nil {
		return nil, err
	}

	clientConfig := &models.Config{
		BaseURL:        cfg.RemnaPanelURL,
		Login:          cfg.RemnaLogin,
		Pass:           cfg.RemnaPass,
		SecretURLToken: cfg.RemnasecretUrlToken,
		APIToken:       cfg.RemnawaveKey,
	}
	remnaClient := remnawave.NewRemnaClient(clientConfig)

	if err := remnaClient.CreateClient("873925524", 5); err != nil {
		return nil, err
	}

	return &App{
		remnawaveClient: remnaClient,
	}, nil
}

func (a *App) Run() {
	ctx := context.Background()

	a.remnawaveClient.GetServiceInfo(ctx, "")
}
