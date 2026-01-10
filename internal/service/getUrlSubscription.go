// Package service сервис по получению URL подписки,
// взаимодействует с remnawave.
package service

import "ProxyMaster_v2/internal/domain"

// GetURLSubscription получает url подписки пользователя через username (Telegram ID).
func GetURLSubscription(remnawaveClient domain.RemnawaveClient, username string) (string, error) {
	uuid, err := remnawaveClient.GetUUIDByUsername(username)
	if err != nil {
		return "", err
	}

	userInfo, err := remnawaveClient.GetUserInfo(uuid)
	if err != nil {
		return "", err
	}

	return userInfo.Response.SubscriptionURL, nil
}
