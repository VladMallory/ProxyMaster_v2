package app

import (
	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/infrastructure/remnawave"
	"context"
	"fmt"
	"net/url"
)

type App struct {
	remnawaveClient domain.RemnawaveClient
}

func New() (*App, error) {
	cfg, err := config.New()
	if err != nil {
		return nil, err
	}

	// Чистим URL от лишнего мусора, оставляем только схему и хост
	baseURL := cfg.RemnaPanelURL
	u, _ := url.Parse(cfg.RemnaPanelURL)
	if u != nil {
		baseURL = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	}

	remnawaveAdapter := remnawave.New(baseURL, cfg.RemnawaveKey)

	// Если есть логин и пароль, пробуем получить свежий токен
	if cfg.RemnaLogin != "" && cfg.RemnaPass != "" {
		fmt.Println("Пробуем авторизоваться через логин/пароль...")
		err := remnawaveAdapter.Login(context.Background(), cfg.RemnaLogin, cfg.RemnaPass)
		if err != nil {
			fmt.Printf("Ошибка входа (используем старый токен): %v\n", err)
		} else {
			fmt.Println("Успешная авторизация! Получен новый токен.")
		}
	}

	return &App{
		remnawaveClient: remnawaveAdapter,
	}, nil
}

func (a *App) Run() {
	fmt.Println("Запуск")

	ctx := context.Background()

	// Пробуем получить информацию о сервисе с ID "1" (тестовый запрос)
	fmt.Println("Проверка подключения к Remnawave...")
	info, err := a.remnawaveClient.GetServiceInfo(ctx, "")
	if err != nil {
		fmt.Printf("Ошибка при запросе: %v\n", err)
	} else {
		fmt.Printf("Успех! Данные сервиса: %s\n", info)
	}
}
