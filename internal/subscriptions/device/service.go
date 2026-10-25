package device

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
)

const (
	DefaultExpiryInterval = 5 * time.Minute

	expiryBatchSize = 100
)

type Repository interface {
	ListByUser(ctx context.Context, username string) ([]Device, error)
	GetByID(ctx context.Context, username string, id int64) (Device, error)
	EnsureBaseSeats(ctx context.Context, username string, expiresAt time.Time, count int) error
	AddExtraSeat(ctx context.Context, username string, paymentID string, expiresAt time.Time) error
	ExtendSeat(
		ctx context.Context,
		username string,
		id int64,
		paymentID string,
		months int,
		now time.Time,
	) error
	Bind(ctx context.Context, username string, id int64, hwid string, label string) error
	UpdateLabel(ctx context.Context, username string, id int64, label string) error
	Unbind(ctx context.Context, username string, id int64) error
	ListExpiredUsernames(ctx context.Context, now time.Time, limit int) ([]string, error)
	RevokeExpiredExtra(ctx context.Context, username string, now time.Time) (int64, error)
}

type PanelSubscription struct {
	UserID      int
	ExpireAt    time.Time
	DeviceLimit int
}

// PanelDevice описывает устройство из панели: HWID и читаемое имя.
type PanelDevice struct {
	HWID string
	Name string
}

type Panel interface {
	Subscription(ctx context.Context, username string) (PanelSubscription, error)
	ListDevices(ctx context.Context, userID int) ([]PanelDevice, error)
	DeleteHWID(ctx context.Context, userID int, hwid string) error
	UpdateUser(ctx context.Context, userID int, expireAt time.Time, deviceLimit int) error
}

type Service struct {
	repo         Repository
	panel        Panel
	defaultLimit int
	maxLimit     int
	logger       *zap.Logger
	now          func() time.Time
}

func NewService(
	repo Repository,
	panel Panel,
	defaultLimit int,
	maxLimit int,
	logger *zap.Logger,
) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Service{
		repo:         repo,
		panel:        panel,
		defaultLimit: defaultLimit,
		maxLimit:     maxLimit,
		logger:       logger,
		now:          time.Now,
	}
}

// ListDevices возвращает места пользователя, сверенные с панелью.
// Экран рисуется из БД, панель обновляет привязки, имена и лимит.
func (s *Service) ListDevices(ctx context.Context, username string) ([]Device, error) {
	if username == "" {
		return nil, errEmptyUsername
	}

	devices, err := s.repo.ListByUser(ctx, username)
	if err != nil {
		return nil, err
	}

	sub, panelDevices, ok := s.panelState(ctx, username)
	if !ok {
		return devices, nil
	}

	// Базовые места подтягиваем к сроку подписки, пока она активна.
	if sub.ExpireAt.After(s.now()) {
		devices, err = s.ensureBaseSeats(ctx, username, devices, sub.ExpireAt)
		if err != nil {
			s.logger.Warn(
				"device sync: base seats failed; using db",
				zap.String("username", username),
				zap.Error(err),
			)

			return devices, nil
		}
	}

	devices, err = s.reconcile(ctx, username, devices, panelDevices)
	if err != nil {
		s.logger.Warn(
			"device sync: reconcile failed; using last known db state",
			zap.String("username", username),
			zap.Error(err),
		)

		return devices, nil
	}

	s.syncPanel(ctx, username, sub, devices)

	return devices, nil
}

// GetDevice возвращает одно место по ID и username из Telegram.
func (s *Service) GetDevice(ctx context.Context, username string, id int64) (Device, error) {
	if username == "" {
		return Device{}, errEmptyUsername
	}
	if id <= 0 {
		return Device{}, ErrSeatNotFound
	}

	return s.repo.GetByID(ctx, username, id)
}

// panelState читает подписку и устройства из панели.
// При любой ошибке возвращает false, вызывающий остаётся на данных БД.
func (s *Service) panelState(
	ctx context.Context,
	username string,
) (PanelSubscription, []PanelDevice, bool) {
	sub, err := s.panel.Subscription(ctx, username)
	if err != nil {
		s.logger.Warn(
			"device sync: panel subscription failed; using db",
			zap.String("username", username),
			zap.Error(err),
		)

		return PanelSubscription{}, nil, false
	}

	panelDevices, err := s.panel.ListDevices(ctx, sub.UserID)
	if err != nil {
		s.logger.Warn(
			"device sync: panel device list failed; using db",
			zap.String("username", username),
			zap.Error(err),
		)

		return PanelSubscription{}, nil, false
	}

	return sub, panelDevices, true
}

// ensureBaseSeats держит базовые места у подписки: создаёт недостающие
// и выравнивает срок. Возвращает свежий список.
func (s *Service) ensureBaseSeats(
	ctx context.Context,
	username string,
	devices []Device,
	expiresAt time.Time,
) ([]Device, error) {
	if err := s.repo.EnsureBaseSeats(ctx, username, expiresAt, s.defaultLimit); err != nil {
		return devices, fmt.Errorf("ensure base seats: %w", err)
	}

	fresh, err := s.repo.ListByUser(ctx, username)
	if err != nil {
		return devices, fmt.Errorf("reread after ensure: %w", err)
	}

	return fresh, nil
}

// CanBuyExtra проверяет, можно ли сейчас купить доп. место: подписка активна
// и лимит не достигнут. Нужно хендлеру, чтобы не создавать платёж впустую.
func (s *Service) CanBuyExtra(ctx context.Context, username string) error {
	now := s.now()

	sub, err := s.panel.Subscription(ctx, username)
	if errors.Is(err, ErrPanelNotFound) {
		return ErrSubscriptionClosed
	}
	if err != nil {
		return fmt.Errorf("can buy extra: %w", err)
	}
	if !sub.ExpireAt.After(now) {
		return ErrSubscriptionClosed
	}

	devices, err := s.repo.ListByUser(ctx, username)
	if err != nil {
		return fmt.Errorf("can buy extra: %w", err)
	}
	if s.defaultLimit+CountActiveExtra(devices, now) >= s.maxLimit {
		return ErrDeviceLimitReached
	}

	return nil
}

// BuyExtraSeat создаёт оплаченное дополнительное место и поднимает лимит
// панели. Панель догоняет лимит; сбой панели оплату не откатывает.
func (s *Service) BuyExtraSeat(
	ctx context.Context,
	username string,
	paymentID string,
	months int,
) error {
	if username == "" {
		return errEmptyUsername
	}
	if months <= 0 {
		return ErrSeatNotRenewable
	}

	now := s.now()

	sub, err := s.panel.Subscription(ctx, username)
	if errors.Is(err, ErrPanelNotFound) {
		return ErrSubscriptionClosed
	}
	if err != nil {
		return fmt.Errorf("buy extra seat: panel subscription: %w", err)
	}
	if !sub.ExpireAt.After(now) {
		return ErrSubscriptionClosed
	}

	devices, err := s.repo.ListByUser(ctx, username)
	if err != nil {
		return fmt.Errorf("buy extra seat: list seats: %w", err)
	}

	activeExtra := CountActiveExtra(devices, now)
	if s.defaultLimit+activeExtra >= s.maxLimit {
		return ErrDeviceLimitReached
	}

	if err := s.repo.AddExtraSeat(ctx, username, paymentID, NewExpire(now, months)); err != nil {
		return fmt.Errorf("buy extra seat: %w", err)
	}

	s.pushLimit(ctx, sub, s.defaultLimit+activeExtra+1)

	return nil
}

// ExtendSeat продлевает одно оплаченное место на месяцы. Продлевается только
// активное дополнительное место, базовые живут сроком подписки.
func (s *Service) ExtendSeat(
	ctx context.Context,
	username string,
	seatID int64,
	paymentID string,
	months int,
) error {
	if username == "" {
		return errEmptyUsername
	}
	if seatID <= 0 {
		return ErrSeatNotFound
	}
	if months <= 0 {
		return ErrSeatNotRenewable
	}

	if err := s.repo.ExtendSeat(ctx, username, seatID, paymentID, months, s.now()); err != nil {
		return fmt.Errorf("extend seat: %w", err)
	}

	return nil
}

// RemoveDevice отвязывает устройство от места.
// Сначала удаляет HWID из панели, затем освобождает место в БД.
func (s *Service) RemoveDevice(
	ctx context.Context,
	username string,
	deviceID int64,
) error {
	if username == "" {
		return errEmptyUsername
	}
	if deviceID <= 0 {
		return ErrSeatNotFound
	}

	seat, err := s.repo.GetByID(ctx, username, deviceID)
	if err != nil {
		return err
	}

	if !seat.Bound() {
		return nil
	}

	sub, err := s.panel.Subscription(ctx, username)
	if errors.Is(err, ErrPanelNotFound) {
		s.logger.Warn(
			"девайс удален: в панели user удален",
			zap.String("username", username),
			zap.Int64("device_id", seat.ID),
		)

		return s.repo.Unbind(ctx, username, seat.ID)
	}
	if err != nil {
		return fmt.Errorf("удаление девайса: %w", err)
	}

	if err := s.panel.DeleteHWID(ctx, sub.UserID, seat.HWID); err != nil {
		return fmt.Errorf("remove device: panel delete hwid: %w", err)
	}

	if err := s.repo.Unbind(ctx, username, seat.ID); err != nil {
		s.logger.Warn(
			"device remove: panel deleted hwid but db unbind failed",
			zap.String("username", username),
			zap.Int64("device_id", seat.ID),
			zap.Error(err),
		)

		return fmt.Errorf("remove device: db unbind: %w", err)
	}

	return nil
}

// RunExpiryWorker запускает фоновый проход отзыва истёкших мест.
func (s *Service) RunExpiryWorker(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = DefaultExpiryInterval
	}

	s.runExpiry(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runExpiry(ctx)
		}
	}
}

func (s *Service) runExpiry(ctx context.Context) {
	if err := s.ExpireSeats(ctx); err != nil {
		s.logger.Error("device expiry: worker failed", zap.Error(err))
	}
}

// ExpireSeats находит пользователей с истёкшими местами и обрабатывает их:
// продлевает базовые, если подписка ещё активна, отзывает истёкшие доп. места
// и пересчитывает лимит панели.
func (s *Service) ExpireSeats(ctx context.Context) error {
	now := s.now()

	usernames, err := s.repo.ListExpiredUsernames(ctx, now, expiryBatchSize)
	if err != nil {
		return fmt.Errorf("list expired usernames: %w", err)
	}

	var errs []error

	for _, username := range usernames {
		if err := s.expireUser(ctx, username, now); err != nil {
			s.logger.Warn(
				"device expiry: user failed",
				zap.String("username", username),
				zap.Error(err),
			)

			errs = append(errs, fmt.Errorf("%s: %w", username, err))
		}
	}

	return errors.Join(errs...)
}

// expireUser обрабатывает одного пользователя.
func (s *Service) expireUser(
	ctx context.Context,
	username string,
	now time.Time,
) error {
	devices, err := s.repo.ListByUser(ctx, username)
	if err != nil {
		return fmt.Errorf("list seats: %w", err)
	}

	sub, err := s.panel.Subscription(ctx, username)
	if errors.Is(err, ErrPanelNotFound) {
		return s.revokeExpiredExtra(ctx, username, now)
	}
	if err != nil {
		return fmt.Errorf("panel subscription: %w", err)
	}

	return s.expirePanelUser(ctx, username, sub, devices, now)
}

// expirePanelUser сверяет места пользователя, который ещё есть в панели.
func (s *Service) expirePanelUser(
	ctx context.Context,
	username string,
	sub PanelSubscription,
	devices []Device,
	now time.Time,
) error {
	var err error

	// Подписка активна, подтягиваем базовые места к её сроку.
	if sub.ExpireAt.After(now) {
		devices, err = s.ensureBaseSeats(ctx, username, devices, sub.ExpireAt)
		if err != nil {
			return err
		}
	}

	panelDevices, err := s.panel.ListDevices(ctx, sub.UserID)
	if err != nil {
		return fmt.Errorf("panel device list: %w", err)
	}

	devices, err = s.reconcileAt(ctx, username, devices, panelDevices, now)
	if err != nil {
		return fmt.Errorf("reconcile: %w", err)
	}

	if err = s.deleteInactiveHWIDs(ctx, sub.UserID, devices, panelDevices, now); err != nil {
		return err
	}

	revoked, err := s.repo.RevokeExpiredExtra(ctx, username, now)
	if err != nil {
		return fmt.Errorf("revoke expired extra seats: %w", err)
	}

	s.syncPanel(ctx, username, sub, devices)

	if revoked > 0 {
		s.logger.Info(
			"device expiry: extra seats revoked",
			zap.String("username", username),
			zap.Int64("revoked", revoked),
		)
	}

	return nil
}

// revokeExpiredExtra отзывает истёкшие доп. места, когда пользователя уже нет
// в панели: лимит там всё равно некому обновлять.
func (s *Service) revokeExpiredExtra(
	ctx context.Context,
	username string,
	now time.Time,
) error {
	if _, err := s.repo.RevokeExpiredExtra(ctx, username, now); err != nil {
		return fmt.Errorf("revoke expired extra seats: %w", err)
	}

	return nil
}

// deleteInactiveHWIDs удаляет из панели HWID, которым не соответствует
// активное занятое место. Базовые места не трогаются, чтобы устройство
// вернулось после возобновления подписки.
func (s *Service) deleteInactiveHWIDs(
	ctx context.Context,
	userID int,
	devices []Device,
	panelDevices []PanelDevice,
	now time.Time,
) error {
	keep := make(map[string]struct{})

	for _, d := range devices {
		if !d.Bound() {
			continue
		}
		if d.ActiveAt(now) || d.Base() {
			keep[d.HWID] = struct{}{}
		}
	}

	for _, pd := range panelDevices {
		if pd.HWID == "" {
			continue
		}
		if _, ok := keep[pd.HWID]; ok {
			continue
		}

		if err := s.panel.DeleteHWID(ctx, userID, pd.HWID); err != nil {
			return fmt.Errorf("panel delete hwid %q: %w", pd.HWID, err)
		}
	}

	return nil
}

// syncPanel проталкивает в панель лимит = базовое число + активные доп. места.
// Срок подписки при этом не меняется, его владелец модуль подписок.
func (s *Service) syncPanel(
	ctx context.Context,
	username string,
	sub PanelSubscription,
	devices []Device,
) {
	target := s.defaultLimit + CountActiveExtra(devices, s.now())
	if sub.DeviceLimit == target {
		return
	}

	s.pushLimit(ctx, sub, target)

	s.logger.Info(
		"device sync: panel limit updated",
		zap.String("username", username),
		zap.Int("limit", target),
		zap.Int("panel_limit", sub.DeviceLimit),
	)
}

// pushLimit обновляет лимит в панели, сохраняя её срок подписки.
func (s *Service) pushLimit(ctx context.Context, sub PanelSubscription, limit int) {
	if err := s.panel.UpdateUser(ctx, sub.UserID, sub.ExpireAt, limit); err != nil {
		s.logger.Warn(
			"device sync: panel limit update failed",
			zap.Int("user_id", sub.UserID),
			zap.Int("limit", limit),
			zap.Error(err),
		)
	}
}

// reconcile сверяет привязки БД с панелью и обновляет имена устройств.
func (s *Service) reconcile(
	ctx context.Context,
	username string,
	devices []Device,
	panelDevices []PanelDevice,
) ([]Device, error) {
	return s.reconcileAt(ctx, username, devices, panelDevices, s.now())
}

// reconcileAt то же, что reconcile, но с явным временем для тестов.
func (s *Service) reconcileAt(
	ctx context.Context,
	username string,
	devices []Device,
	panelDevices []PanelDevice,
	now time.Time,
) ([]Device, error) {
	devices, err := s.unbindMissing(ctx, username, devices, panelHWIDSet(panelDevices))
	if err != nil {
		return devices, err
	}

	devices, err = s.bindMissing(ctx, username, devices, panelDevices, now)
	if err != nil {
		return devices, err
	}

	devices, err = s.updateLabels(ctx, username, devices, panelDevices)
	if err != nil {
		return devices, err
	}

	fresh, err := s.repo.ListByUser(ctx, username)
	if err != nil {
		return devices, err
	}

	return fresh, nil
}

// panelHWIDSet собирает множество HWID из ответа панели, пустые пропускает.
func panelHWIDSet(panelDevices []PanelDevice) map[string]struct{} {
	set := make(map[string]struct{}, len(panelDevices))

	for _, pd := range panelDevices {
		if pd.HWID == "" {
			continue
		}

		set[pd.HWID] = struct{}{}
	}

	return set
}

// panelNames строит карту HWID -> читаемое имя.
func panelNames(panelDevices []PanelDevice) map[string]string {
	names := make(map[string]string, len(panelDevices))

	for _, pd := range panelDevices {
		if pd.HWID == "" {
			continue
		}

		names[pd.HWID] = pd.Name
	}

	return names
}

// unbindMissing освобождает места, чей HWID пропал из панели.
func (s *Service) unbindMissing(
	ctx context.Context,
	username string,
	devices []Device,
	panelSet map[string]struct{},
) ([]Device, error) {
	changed := false

	for _, d := range devices {
		if !d.Bound() {
			continue
		}

		if _, ok := panelSet[d.HWID]; ok {
			continue
		}

		if err := s.repo.Unbind(ctx, username, d.ID); err != nil {
			return devices, err
		}

		changed = true
	}

	if !changed {
		return devices, nil
	}

	return s.repo.ListByUser(ctx, username)
}

// bindMissing привязывает новые HWID из панели к свободным местам.
func (s *Service) bindMissing(
	ctx context.Context,
	username string,
	devices []Device,
	panelDevices []PanelDevice,
	now time.Time,
) ([]Device, error) {
	for _, pd := range panelDevices {
		if pd.HWID == "" {
			continue
		}

		if hasHWID(devices, pd.HWID) {
			continue
		}

		bound, err := s.bindOne(ctx, username, &devices, pd.HWID, pd.Name, now)
		if err != nil {
			return devices, err
		}

		if !bound {
			s.logger.Warn(
				"device sync: panel has more hwids than free seats",
				zap.String("username", username),
			)

			break
		}
	}

	return devices, nil
}

// updateLabels обновляет имена уже привязанных устройств из панели.
func (s *Service) updateLabels(
	ctx context.Context,
	username string,
	devices []Device,
	panelDevices []PanelDevice,
) ([]Device, error) {
	names := panelNames(panelDevices)

	changed := false

	for _, d := range devices {
		if !d.Bound() {
			continue
		}

		name := names[d.HWID]
		if name == "" || name == d.Label {
			continue
		}

		if err := s.repo.UpdateLabel(ctx, username, d.ID, name); err != nil {
			return devices, err
		}

		changed = true
	}

	if !changed {
		return devices, nil
	}

	return s.repo.ListByUser(ctx, username)
}

// bindOne привязывает один HWID к самому раннему свободному месту.
func (s *Service) bindOne(
	ctx context.Context,
	username string,
	devices *[]Device,
	hwid string,
	label string,
	now time.Time,
) (bool, error) {
	tried := make(map[int64]struct{})

	for {
		if hasHWID(*devices, hwid) {
			return true, nil
		}

		free := firstFreeIndexExcept(*devices, now, tried)
		if free < 0 {
			return false, nil
		}

		seatID := (*devices)[free].ID

		err := s.repo.Bind(ctx, username, seatID, hwid, label)
		if errors.Is(err, ErrSeatNotFree) {
			tried[seatID] = struct{}{}

			fresh, rerr := s.repo.ListByUser(ctx, username)
			if rerr != nil {
				return false, rerr
			}

			*devices = fresh

			continue
		}
		if err != nil {
			return false, err
		}

		(*devices)[free].HWID = hwid
		(*devices)[free].Label = label

		return true, nil
	}
}

// firstFreeIndexExcept ищет первое свободное место, кроме уже опробованных.
func firstFreeIndexExcept(
	devices []Device,
	now time.Time,
	except map[int64]struct{},
) int {
	for i := range devices {
		if _, ok := except[devices[i].ID]; ok {
			continue
		}
		if devices[i].FreeAt(now) {
			return i
		}
	}

	return -1
}

// hasHWID проверяет, есть ли данный HWID среди мест.
func hasHWID(devices []Device, hwid string) bool {
	for _, d := range devices {
		if d.HWID == hwid {
			return true
		}
	}

	return false
}
