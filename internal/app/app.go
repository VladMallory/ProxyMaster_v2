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

	remnawaveAdapter := remnawave.New(cfg.RemnaPanelURL, cfg.RemnawaveKey)

	return &App{
		remnawaveClient: remnawaveAdapter,
	}, nil
}

func (a *App) Run() {
	fmt.Println("Запуск")

	ctx := context.Background()

	// Пробуем получить информацию о сервисе с ID "1" (тестовый запрос)
	fmt.Println("Проверка подключения к Remnawave...")
	info, err := a.remnawaveClient.GetServiceInfo(ctx, "1")
	if err != nil {
		fmt.Printf("Ошибка при запросе: %v\n", err)
	} else {
		fmt.Printf("Успех! Данные сервиса: %s\n", info)
	}
}
