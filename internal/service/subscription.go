package service

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/infrastructure/remnawave"
)

// SubscriptionService представляет собой сервис для управления подписками клиентов с помощью remnawave.
type SubscriptionService struct {
	remna domain.RemnawaveClient
}

// NewSubscriptionService конструктор сервиса.
func NewSubscriptionService(remna domain.RemnawaveClient) *SubscriptionService {
	return &SubscriptionService{
		remna: remna,
	}
}

// ActivateSubscription активирует подписку клиенту telegram на указанное количество месяцев.
// Если имеется подписка - продлить. Если подписки нет - создать.
func (s *SubscriptionService) ActivateSubscription(telegramID int64, months int) (string, error) {
	totalDays := months * 30
	username := strconv.FormatInt(telegramID, 10)

	// Проверяем есть ли пользователь в панели
	userUUID, err := s.remna.GetUUIDByUsername(username)
	if err != nil {
		if errors.Is(err, remnawave.ErrNotFound) {
			log.Println("СЕРВИС: пользователь не найден, создаем нового:", username)
			err = s.remna.CreateUser(username, totalDays)
			if err != nil {
				return "", fmt.Errorf("ошибка создания пользователя: %w", err)
			}

			return fmt.Sprintf("пользователь %s создан на %d дней", username, totalDays), nil
		}

		return "", fmt.Errorf("ошибка поиска пользователя: %w", err)
	}

	log.Println("СЕРВИС: пользователь найден")

	err = s.remna.ExtendClientSubscription(userUUID, username, totalDays)
	if err != nil {
		return "", errors.New("ошибка продления подписки пользователю: " + username + " " + err.Error())
	}

	return "подписка для пользователя " + username + " продлена на " + strconv.Itoa(totalDays) + " дней", nil
}
