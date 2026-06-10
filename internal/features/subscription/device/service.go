package device

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/domain"
	billingDomain "github.com/VladMallory/ProxyMaster_v2/internal/features/billing/domain"
	"github.com/VladMallory/ProxyMaster_v2/internal/integrations/remnawave"
	"go.uber.org/zap"
)

const extraDevicePriceRUB = 50

// DeviceService управляет устройствами пользователя (бесплатными и платными).
type DeviceService struct {
	remna           remnawave.RemnawaveClient
	userRepo        domain.UserRepository
	addonRepo       DeviceAddonRepository
	logger          *zap.Logger
	baseDeviceLimit int
	maxDeviceLimit  int
}

func NewDeviceService(
	remna remnawave.RemnawaveClient,
	userRepo domain.UserRepository,
	addonRepo DeviceAddonRepository,
	l *zap.Logger,
	baseDeviceLimit int,
	maxDeviceLimit int,
) *DeviceService {
	return &DeviceService{
		remna:           remna,
		userRepo:        userRepo,
		addonRepo:       addonRepo,
		logger:          l,
		baseDeviceLimit: baseDeviceLimit,
		maxDeviceLimit:  maxDeviceLimit,
	}
}

// AddPaidDevice покупает 1 доп. устройство за 50₽/мес атомарно.
func (s *DeviceService) AddPaidDevice(userID string) error {
	defer s.logDuration("AddPaidDevice")()

	_, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return s.logError("user not found", err, zap.String("user_id", userID))
	}

	_, err = s.remna.GetUUIDByUsername(context.Background(), userID)
	if err != nil {
		if errors.Is(err, remnawave.ErrNotFound) {
			dto := remnawave.CreateUserDTO{Username: userID, Days: 30}
			if err := s.remna.CreateUser(dto); err != nil {
				return s.logError("failed to create user", err)
			}
		} else {
			return s.logError("failed to find user", err,
				zap.String("user_id", userID),
			)
		}
	}

	newExtraDevicesCount, err := s.addonRepo.AddDeviceAddonAtomic(
		userID, s.baseDeviceLimit, s.maxDeviceLimit, extraDevicePriceRUB, 30*24*time.Hour,
	)
	if err != nil {
		if errors.Is(err, ErrMaxDevices) || errors.Is(err, billingDomain.ErrInsufficientFunds) {
			return err
		}

		return s.logError("failed to buy device", err, zap.String("user_id", userID))
	}

	devices := uint8(s.baseDeviceLimit + newExtraDevicesCount)
	if err := s.remna.SetDevices(context.Background(), userID, &devices); err != nil {
		return s.logError("failed to set device limit", err, zap.String("user_id", userID))
	}

	return nil
}

// ResetPaidDevices сбрасывает платные устройства до базового лимита.
func (s *DeviceService) ResetPaidDevices(userID string) error {
	defer s.logDuration("ResetPaidDevices")()

	if err := s.addonRepo.DeactivateAllDeviceAddons(userID); err != nil {
		return s.logError("failed to deactivate addons", err)
	}

	var zero int
	_, err := s.userRepo.UpdateUser(userID, domain.UpdateUserTGDTO{ExtraDevicesCount: &zero})
	if err != nil {
		return s.logError("failed to reset count", err)
	}

	devices := uint8(s.baseDeviceLimit)
	if err := s.remna.SetDevices(context.Background(), userID, &devices); err != nil {
		return s.logError("failed to set device limit", err)
	}

	return nil
}

// PrepayPaidDevices предоплачивает все активные доп. устройства на месяц.
func (s *DeviceService) PrepayPaidDevices(userID string) (int, error) {
	defer s.logDuration("PrepayPaidDevices")()

	count, err := s.addonRepo.PrepayDeviceAddonsAtomic(userID, extraDevicePriceRUB, 30*24*time.Hour)
	if err != nil {
		if errors.Is(err, billingDomain.ErrInsufficientFunds) || errors.Is(err, ErrNoActiveDeviceAddons) {
			return 0, err
		}

		return 0, s.logError("failed to prepay devices", err)
	}

	return count, nil
}

func (s *DeviceService) logDuration(method string) func() {
	start := time.Now()

	return func() {
		s.logger.Info("вызов метода завершен",
			zap.String("method", method),
			zap.Duration("duration", time.Since(start)),
		)
	}
}

func (s *DeviceService) logError(msg string, err error, fields ...zap.Field) error {
	allFields := append([]zap.Field{zap.Error(err)}, fields...)
	s.logger.Error(msg, allFields...)

	return fmt.Errorf("%s: %w", msg, err)
}
