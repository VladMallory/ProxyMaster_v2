package main

import (
	"ProxyMaster_v2/internal/app"
	"log"
	"log/slog"
	"os"
)

func main() {
	// настройка логирования
	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	})
	logger := slog.New(handler)

	// сборка приложения
	myApp, err := app.New()
	slog.SetDefault(logger)
	if err != nil {
		log.Fatal("ошибка сборки приложения: %w", err)
	}

	// запуск приложения
	myApp.Run()
}
