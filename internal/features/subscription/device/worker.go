package device

import (
	"context"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/integrations/remnawave"
	"go.uber.org/zap"
)

// // DeviceAddonRepository определяет операции работы с device_addons в БД.
type DeviceAddonRepository interface {
	AddDeviceAddonAtomic(
		userID string,
		baseLimit, maxLimit, priceRUB int,
		chargePeriod time.Duration,
	) (newCount int, err error)
	DeactivateAllDeviceAddons(userID string) error
	PrepayDeviceAddonsAtomic(
		userID string,
		priceRUB int,
		chargePeriod time.Duration,
	) (count int, err error)
	ProcessDueDeviceAddonsBilling(
		now time.Time,
		limit int,
		priceRUB int,
		chargePeriod time.Duration,
	) (map[string]int, error)
	CountActiveDeviceAddons(userID string) (int, error)
}

// DeviceWorker занимается фоновым биллингом доп. устройств.
type DeviceWorker struct {
	remna           remnawave.RemnawaveClient
	addonRepo       DeviceAddonRepository
	logger          *zap.Logger
	baseDeviceLimit int
}

func NewDeviceBillingService(
	remna remnawave.RemnawaveClient,
	addonRepo DeviceAddonRepository,
	l *zap.Logger,
	baseDeviceLimit int,
) *DeviceWorker {
	return &DeviceWorker{
		remna:           remna,
		addonRepo:       addonRepo,
		logger:          l,
		baseDeviceLimit: baseDeviceLimit,
	}
}

// RunBillingLoop запускает фоновую проверку раз в 24 часа.
func (s *DeviceWorker) RunBillingLoop(ctx context.Context) {
	s.processDueBilling(ctx, time.Now())

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processDueBilling(ctx, time.Now())
		}
	}
}

// processDueBilling обрабатывает просроченные доп. устройства и синхронизирует RemnaWave.
func (s *DeviceWorker) processDueBilling(ctx context.Context, now time.Time) {
	activeCounts, err := s.addonRepo.ProcessDueDeviceAddonsBilling(
		now, 200, extraDevicePriceRUB, 30*24*time.Hour,
	)
	if err != nil {
		s.logger.Error("device billing failed", zap.Error(err))

		return
	}

	for userID, activeAddons := range activeCounts {
		devices := uint8(s.baseDeviceLimit + activeAddons)
		if err := s.remna.SetDevices(ctx, userID, &devices); err != nil {
			s.logger.Error(
				"failed to sync device limit",
				zap.String("user_id", userID),
				zap.Error(err),
			)
		}
	}
}
