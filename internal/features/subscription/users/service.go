package users

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/domain"
	"github.com/VladMallory/ProxyMaster_v2/internal/integrations/remnawave"
	"go.uber.org/zap"
)

// SubscriptionService управляет подписками (активация, продление, получение URL).
type SubscriptionService struct {
	remna  remnawave.RemnawaveClient
	dbRepo domain.UserRepository
	logger *zap.Logger
}

func NewSubscriptionService(
	remna remnawave.RemnawaveClient,
	dbRepo domain.UserRepository,
	l *zap.Logger,
) *SubscriptionService {
	return &SubscriptionService{
		remna:  remna,
		dbRepo: dbRepo,
		logger: l,
	}
}

// GetURLSubscription получает url подписки пользователя.
func (s *SubscriptionService) GetURLSubscription(userID string) (string, error) {
	uuid, err := s.remna.GetUUIDByUsername(context.Background(), userID)
	if err != nil {
		return "", fmt.Errorf(
			"не удалось получить UUID пользователя: %w",
			err,
		)
	}

	userInfo, err := s.remna.GetUserInfo(uuid)
	if err != nil {
		return "", fmt.Errorf("не удалось получить информацию о пользователе: %w", err)
	}

	return userInfo.Response.SubscriptionURL, nil
}

func (s *SubscriptionService) logDuration(method string) func() {
	start := time.Now()

	return func() {
		s.logger.Info(
			"вызов метода завершен",
			zap.String("method", method),
			zap.Duration("duration", time.Since(start)),
		)
	}
}

func (s *SubscriptionService) logError(msg string, err error, fields ...zap.Field) error {
	allFields := append([]zap.Field{zap.Error(err)}, fields...)
	s.logger.Error(msg, allFields...)

	return fmt.Errorf("%s: %w", msg, err)
}

// ActivateSubscription активирует/продлевает подписку на указанное количество месяцев.
// nolint:funlen
func (s *SubscriptionService) ActivateSubscription(userID string, months int) (string, error) {
	defer s.logDuration("ActivateSubscription")()

	_, err := s.dbRepo.GetUserByID(userID)
	if err != nil {
		return "", s.logError(
			"пользователь не найден в DB",
			err,
			zap.String("user_id", userID),
		)
	}

	s.logger.Info("пользователь найден", zap.String("user_id", userID))

	totalDays := months * 30

	userUUID, err := s.remna.GetUUIDByUsername(context.Background(), userID)
	if err != nil {
		if errors.Is(err, remnawave.ErrNotFound) {
			s.logger.Info(
				"пользователь не найден, создаем нового",
				zap.String("user_id", userID),
			)

			dto := remnawave.CreateUserDTO{
				Username: userID,
				Days:     totalDays,
			}

			err = s.remna.CreateUser(dto)
			if err != nil {
				return "", s.logError(
					"ошибка создания пользователя",
					err,
					zap.String("user_id", userID),
				)
			}

			return fmt.Sprintf("пользователь %s создан на %d дней", userID, totalDays), nil
		}

		return "", s.logError(
			"ошибка поиска пользователя",
			err,
			zap.String("user_id", userID),
		)
	}

	s.logger.Info("пользователь найден", zap.String("user_id", userID))

	userInfo, err := s.remna.GetUserInfo(userUUID)
	if err != nil {
		return "", s.logError(
			"ошибка получения информации о пользователе",
			err,
			zap.String("user_id", userID),
		)
	}

	if userInfo.Response.ExpireAt.Before(time.Now()) {
		daysSinceExpiry := int(time.Since(userInfo.Response.ExpireAt).Hours()/24) + 1
		totalDays += daysSinceExpiry
	}

	err = s.remna.ExtendClientSubscription(userUUID, userID, totalDays)
	if err != nil {
		return "", s.logError(
			"ошибка продления подписки",
			err,
			zap.String("user_id", userID),
		)
	}

	s.logger.Info(
		"подписка продлена",
		zap.String("user_id", userID),
		zap.Int("days", totalDays),
	)

	return "подписка для пользователя " + userID + " продлена на " + strconv.Itoa(
		totalDays,
	) + " дней", nil
}
