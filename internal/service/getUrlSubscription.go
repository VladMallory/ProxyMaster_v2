// Package service содержит сервис по получению URL подписки,
// взаимодействующий с remnawave.
package service

import (
	"fmt"

	"ProxyMaster_v2/internal/domain"
)

// GetURLSubscription получает url подписки пользователя через username (Telegram ID).
func GetURLSubscription(remnawaveClient domain.RemnawaveClient, username string) (string, error) {
	// Получаем UUID пользователя по username (Telegram ID)
	uuid, err := remnawaveClient.GetUUIDByUsername(username)
	if err != nil {
		return "", fmt.Errorf("не удалось получить UUID пользователя: %w", err)
	}

	// Получаем информацию о пользователе по UUID
	userInfo, err := remnawaveClient.GetUserInfo(uuid)
	if err != nil {
		return "", fmt.Errorf("не удалось получить информацию о пользователе: %w", err)
	}

	return userInfo.Response.SubscriptionURL, nil
}
