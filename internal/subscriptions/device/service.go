package device

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
)

var (
	ErrInvalidArgs    = errors.New("неверные аргументы операции с устройствами")
	ErrDeviceNotFound = errors.New("оплаченное место не найдено")
)

type Repository interface {
	Grant(
		ctx context.Context,
		paymentID, username string,
		count, months int,
		now time.Time,
	) (applied bool, err error)
	Extend(
		ctx context.Context,
		paymentID, username string,
		deviceID int64,
		months int,
		now time.Time,
	) (applied bool, err error)

	// ListByUser все места username, ОБЯЗАТЕЛЬНО ORDER BY created_at ASC, id ASC
	// (от этого зависит выбор «самого раннего свободного» места при привязке).
	ListByUser(ctx context.Context, username string) ([]Device, error)
	// Bind пишет hwid и label в конкретное свободное место.
	Bind(ctx context.Context, deviceID int64, hwid, label string) error
	// Unbind отвязывает устройство от места, оставляя место оплаченным (свободным).
	Unbind(ctx context.Context, username string, deviceID int64) error
	// Import заливает места, прочитанные из панели, пользователю, у которого в БД
	// ещё нет строк. Идемпотентен: блокировка по username + проверка count.
	Import(ctx context.Context, username string, rows []Device) error
	// ListToRevoke места с expires_at <= now и revoked_at IS NULL.
	ListToRevoke(ctx context.Context, now time.Time) ([]Device, error)
	// MarkRevoked закрывает место после удаления устройства из панели.
	MarkRevoked(ctx context.Context, deviceID int64, now time.Time) error
	// UsersToSync username, у которых срок/создание попало в (since, until].
	UsersToSync(ctx context.Context, since, until time.Time) ([]string, error)
}

// PanelDevice устройство из панели.
type PanelDevice struct {
	HWID        string
	Platform    string
	DeviceModel string
}

// PanelSubscription снимок подписки в панели: сколько мест разрешено и докуда.
// Нужен для ленивого импорта мест у пользователей, заведённых до перехода на
// модель «каждое устройство отдельно».
type PanelSubscription struct {
	Limit    int
	ExpireAt time.Time
}

type Panel interface {
	ListDevices(ctx context.Context, username string) ([]PanelDevice, error)
	Subscription(ctx context.Context, username string) (PanelSubscription, error)
	DeleteDevice(ctx context.Context, username, hwid string) error
	UpdateSubscription(ctx context.Context, username string, limit int, expireAt time.Time) error
}

// Service бизнес-логика устройств: оплата мест, привязка hwid, отзыв по сроку.
type Service struct {
	repo  Repository
	panel Panel
	log   *zap.Logger
	// now инъекция времени. Вынесена в поле ради тестируемости сроков:
	// в тестах подменяем счётчик, в бою это time.Now.
	now func() time.Time
}

func New(repo Repository, panel Panel, log *zap.Logger) *Service {
	return &Service{repo: repo, panel: panel, log: log, now: time.Now}
}

// Grant создаёт count новых оплаченных мест по months месяцев каждое.
func (s *Service) Grant(
	ctx context.Context,
	paymentID, username string,
	count, months int,
) error {
	if paymentID == "" || username == "" || count <= 0 || months <= 0 {
		return ErrInvalidArgs
	}

	// Гарантия базы: клиент мог купить устройство, ни разу не зайдя на экран
	// «Ваши устройства», и тогда строк в БД ещё нет. Импортируем состояние
	// панели ДО вставки новых мест. Иначе syncPanel посчитает лимит только по
	// новым строкам и «съест» базовые устройства подписки.
	if ds, err := s.repo.ListByUser(ctx, username); err != nil {
		s.log.Warn("grant: list before import failed",
			zap.String("username", username), zap.Error(err))
	} else if len(ds) == 0 {
		if _, ierr := s.importFromPanel(ctx, username); ierr != nil {
			s.log.Warn("grant: import before grant failed",
				zap.String("username", username), zap.Error(ierr))
		}
	}

	applied, err := s.repo.Grant(ctx, paymentID, username, count, months, s.now())
	if err != nil {
		return fmt.Errorf("grant devices: %w", err)
	}
	// Повтор вебхука: строки уже созданы прошлым вызовом, менять нечего.
	if !applied {
		s.log.Info("grant skipped: платёж уже обработан",
			zap.String("payment_id", paymentID),
			zap.String("username", username),
		)

		return nil
	}

	// Оплата уже в БД, поэтому сбой панели её НЕ откатывает: Reconcile доведёт
	// лимит и срок на следующем проходе.
	s.syncPanel(ctx, username)

	return nil
}

// Extend продлевает конкретное место deviceID на months месяцев.
func (s *Service) Extend(
	ctx context.Context,
	paymentID, username string,
	deviceID int64,
	months int,
) error {
	if paymentID == "" || username == "" || deviceID <= 0 || months <= 0 {
		return ErrInvalidArgs
	}

	applied, err := s.repo.Extend(ctx, paymentID, username, deviceID, months, s.now())
	if err != nil {
		return fmt.Errorf("extend device: %w", err)
	}
	if !applied {
		s.log.Info("extend skipped: платёж уже обработан",
			zap.String("payment_id", paymentID),
			zap.Int64("device_id", deviceID),
		)

		return nil
	}

	s.syncPanel(ctx, username)

	return nil
}

// Overview данные для экрана «Ваши устройства».
type Overview struct {
	Devices []Device
	Limit   int
}

// ListDevices собирает экран: сначала привязываем свежие hwid из панели.
func (s *Service) ListDevices(ctx context.Context, username string) (Overview, error) {
	if username == "" {
		return Overview{}, ErrInvalidArgs
	}

	ds, err := s.bind(ctx, username)
	if err != nil {
		return Overview{}, err
	}

	return Overview{
		Devices: withoutRevoked(ds),
		Limit:   CountActive(ds, s.now()),
	}, nil
}

// RemoveDevice отключает устройство от места: удаляет hwid из панели и делает
// место свободным, но НЕ снимает оплату, срок места сохраняется, и клиент
// может подключить другое устройство. Это «отвязать», а не «вернуть деньги».
func (s *Service) RemoveDevice(ctx context.Context, username string, deviceID int64) error {
	if username == "" || deviceID <= 0 {
		return ErrInvalidArgs
	}

	ds, err := s.repo.ListByUser(ctx, username)
	if err != nil {
		return fmt.Errorf("list devices: %w", err)
	}

	var target *Device
	for i := range ds {
		if ds[i].ID == deviceID {
			target = &ds[i]

			break
		}
	}
	if target == nil {
		return ErrDeviceNotFound
	}

	// Панель может уже не знать это устройство (клиент удалил сам) это ок.
	if target.HWID != "" {
		if err := s.panel.DeleteDevice(ctx, username, target.HWID); err != nil {
			return fmt.Errorf("panel delete device: %w", err)
		}
	}

	if err := s.repo.Unbind(ctx, username, deviceID); err != nil {
		return fmt.Errorf("unbind device: %w", err)
	}

	s.syncPanel(ctx, username)

	return nil
}

// bind привязывает подключённые в панели hwid к самым ранним свободным местам.
//
//nolint:cyclop
func (s *Service) bind(ctx context.Context, username string) ([]Device, error) {
	ds, err := s.repo.ListByUser(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}

	if len(ds) == 0 {
		imported, ierr := s.importFromPanel(ctx, username)
		if ierr != nil {
			// Импорт не критичен: показываем то, что есть (пусто), панель
			// попробуем прочитать при следующем заходе.
			s.log.Warn("import from panel failed",
				zap.String("username", username), zap.Error(ierr))
		} else {
			ds = imported
		}
	}

	panelDevices, err := s.panel.ListDevices(ctx, username)
	if err != nil {
		// Панель вспомогательный источник: экран должен открыться и без неё
		s.log.Warn("bind: panel list devices failed",
			zap.String("username", username), zap.Error(err))

		return ds, nil
	}

	// Множество уже привязанных hwid чтобы не перетирать label и не плодить дубли
	bound := make(map[string]struct{}, len(ds))
	for _, d := range ds {
		if d.HWID != "" {
			bound[d.HWID] = struct{}{}
		}
	}

	for _, pd := range panelDevices {
		if _, ok := bound[pd.HWID]; ok {
			continue
		}

		// Свободных мест нет, по лимиту панель не должна была пустить лишнее
		// Если всё же пустила не сочиняем место, просто ждём изменения лимита
		i := FirstFreeIndex(ds)
		if i < 0 {
			break
		}

		label := prettyName(pd)
		if err := s.repo.Bind(ctx, ds[i].ID, pd.HWID, label); err != nil {
			s.log.Warn("bind hwid failed",
				zap.String("username", username),
				zap.String("hwid", pd.HWID), zap.Error(err))

			continue
		}

		// Обновляем копию в памяти, следующий FirstFreeIndex увидит место занятым.
		ds[i].HWID = pd.HWID
		ds[i].Label = label
		bound[pd.HWID] = struct{}{}
	}

	return ds, nil
}

// importFromPanel разово переносит состояние панели в БД: сколько мест (лимит),
// какие hwid заняты и до какого срока. Нужно для пользователей, заведённых до
// перехода на модель «каждое устройство отдельно».
func (s *Service) importFromPanel(ctx context.Context, username string) ([]Device, error) {
	sub, err := s.panel.Subscription(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("panel subscription: %w", err)
	}

	// Просроченную подписку не импортируем: показывать нечего, а строки-зомби
	// только запутают экран (воркер всё равно закрыл бы их на первом проходе).
	if !sub.ExpireAt.After(s.now()) {
		return []Device{}, nil
	}

	panelDevices, err := s.panel.ListDevices(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("panel list devices: %w", err)
	}

	// Создаём столько мест, сколько разрешает панель, но не меньше числа уже
	// подключённых устройств (иначе занятым местам не хватит строк).
	total := max(sub.Limit, len(panelDevices), 0)
	if total <= 0 {
		return []Device{}, nil
	}

	rows := make([]Device, 0, total)
	for _, pd := range panelDevices {
		rows = append(rows, Device{
			Username:  username,
			HWID:      pd.HWID,
			Label:     prettyName(pd),
			ExpiresAt: sub.ExpireAt,
		})
	}
	for len(rows) < total {
		rows = append(rows, Device{Username: username, ExpiresAt: sub.ExpireAt})
	}

	if err := s.repo.Import(ctx, username, rows); err != nil {
		return nil, fmt.Errorf("import devices: %w", err)
	}

	return s.repo.ListByUser(ctx, username)
}

// syncPanel приводит подписку в панели к БД.
func (s *Service) syncPanel(ctx context.Context, username string) {
	ds, err := s.repo.ListByUser(ctx, username)
	if err != nil {
		s.log.Warn("sync panel: list devices failed",
			zap.String("username", username), zap.Error(err))

		return
	}

	now := s.now()
	limit := CountActive(ds, now)
	expireAt := MaxExpiresAt(ds, now)
	if expireAt.IsZero() {
		expireAt = now
	}

	if err := s.panel.UpdateSubscription(ctx, username, limit, expireAt); err != nil {
		// Оплата уже в БД, поэтому сбой панели не критичен: Reconcile повторит.
		s.log.Warn("sync panel: update subscription failed",
			zap.String("username", username), zap.Error(err))
	}
}

// Reconcile периодический проход воркера:
//  1. пересинхронизирует панель тем, у кого с since менялись сроки/места;
//  2. отзывает просроченные места (удаляет hwid из панели, ставит revoked_at).
func (s *Service) Reconcile(ctx context.Context, since time.Time) error {
	users, err := s.repo.UsersToSync(ctx, since, s.now())
	if err != nil {
		return fmt.Errorf("list users to sync: %w", err)
	}

	for _, username := range users {
		if err := ctx.Err(); err != nil {
			return err
		}

		s.syncPanel(ctx, username)
	}

	return s.RevokeExpired(ctx)
}

// RevokeExpired отзывает все места с истёкшим сроком.
func (s *Service) RevokeExpired(ctx context.Context) error {
	now := s.now()

	due, err := s.repo.ListToRevoke(ctx, now)
	if err != nil {
		return fmt.Errorf("list devices to revoke: %w", err)
	}

	for _, d := range due {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Пустое место (оплачено, но никто не подключился) удалять из панели
		// нечего — просто закрываем его.
		if d.HWID != "" {
			if err := s.panel.DeleteDevice(ctx, d.Username, d.HWID); err != nil {
				s.log.Error("revoke: panel delete failed, повторим позже",
					zap.String("username", d.Username),
					zap.String("hwid", d.HWID), zap.Error(err))

				continue
			}
		}

		if err := s.repo.MarkRevoked(ctx, d.ID, now); err != nil {
			s.log.Error("revoke: mark failed, повторим позже",
				zap.Int64("device_id", d.ID), zap.Error(err))

			continue
		}

		s.log.Info("device revoked",
			zap.String("username", d.Username),
			zap.String("hwid", d.HWID),
			zap.Int64("device_id", d.ID),
		)

		// Место закрылось уменьшаем лимит/срок подписки в панели.
		s.syncPanel(ctx, d.Username)
	}

	return nil
}

// withoutRevoked убирает закрытые места: их нет в панели и продлевать нечего.
func withoutRevoked(ds []Device) []Device {
	out := make([]Device, 0, len(ds))
	for _, d := range ds {
		if !d.Revoked() {
			out = append(out, d)
		}
	}

	return out
}

// prettyName собирает «Platform - Model» для Label с fallback на заглушку.
func prettyName(d PanelDevice) string {
	switch {
	case d.Platform != "" && d.DeviceModel != "":
		return d.Platform + " - " + d.DeviceModel
	case d.Platform != "":
		return d.Platform
	case d.DeviceModel != "":
		return d.DeviceModel
	default:
		return "Устройство"
	}
}
