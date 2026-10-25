//nolint:cyclop,funlen,golines,intrange,lll,nlreturn,paralleltest,staticcheck
package device

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

type fakeRepo struct {
	mu     sync.Mutex
	seats  map[string][]Device
	nextID int64
	calls  *[]string
	clock  func() time.Time

	processed map[string]bool

	bindErr   error
	ensureErr error
	unbindErr error
	revokeErr error
	listErr   error
	addErr    error
	extendErr error
	labelErr  error
}

func newFakeRepo(calls *[]string) *fakeRepo {
	return &fakeRepo{
		seats:     make(map[string][]Device),
		calls:     calls,
		clock:     time.Now,
		processed: make(map[string]bool),
	}
}

func (r *fakeRepo) log(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	*r.calls = append(*r.calls, s)
}

func (r *fakeRepo) add(d Device) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if d.ID > r.nextID {
		r.nextID = d.ID
	}
	r.seats[d.Username] = append(r.seats[d.Username], d)
}

func (r *fakeRepo) ListByUser(_ context.Context, username string) ([]Device, error) {
	r.log("repo.list")

	if r.listErr != nil {
		return nil, r.listErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var out []Device
	for _, d := range r.seats[username] {
		if d.RevokedAt.IsZero() {
			out = append(out, d)
		}
	}

	return out, nil
}

func (r *fakeRepo) GetByID(_ context.Context, username string, id int64) (Device, error) {
	r.log("repo.get")

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, d := range r.seats[username] {
		if d.ID == id && d.RevokedAt.IsZero() {
			return d, nil
		}
	}

	return Device{}, ErrSeatNotFound
}

func (r *fakeRepo) EnsureBaseSeats(
	_ context.Context,
	username string,
	expiresAt time.Time,
	count int,
) error {
	r.log("repo.ensure_base")

	if r.ensureErr != nil {
		return r.ensureErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var base int
	for _, d := range r.seats[username] {
		if d.Base() && d.RevokedAt.IsZero() {
			base++
		}
	}

	for i := base; i < count; i++ {
		r.nextID++
		r.seats[username] = append(r.seats[username], Device{
			ID:        r.nextID,
			Username:  username,
			Kind:      KindBase,
			ExpiresAt: expiresAt,
			CreatedAt: r.clock(),
		})
	}

	for i := range r.seats[username] {
		d := &r.seats[username][i]
		if d.Base() && d.RevokedAt.IsZero() {
			d.ExpiresAt = expiresAt
		}
	}

	return nil
}

func (r *fakeRepo) AddExtraSeat(
	_ context.Context,
	username string,
	paymentID string,
	expiresAt time.Time,
) error {
	r.log("repo.add_extra")

	if r.addErr != nil {
		return r.addErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.processed[paymentID] {
		return nil
	}
	r.processed[paymentID] = true

	r.nextID++
	r.seats[username] = append(r.seats[username], Device{
		ID:        r.nextID,
		Username:  username,
		Kind:      KindExtra,
		ExpiresAt: expiresAt,
		CreatedAt: r.clock(),
	})

	return nil
}

func (r *fakeRepo) ExtendSeat(
	_ context.Context,
	username string,
	id int64,
	paymentID string,
	months int,
	now time.Time,
) error {
	r.log("repo.extend")

	if r.extendErr != nil {
		return r.extendErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.processed[paymentID] {
		return nil
	}

	for i := range r.seats[username] {
		d := &r.seats[username][i]
		if d.ID != id {
			continue
		}

		if !d.RevokedAt.IsZero() || !d.Extra() || !d.ExpiresAt.After(now) {
			return ErrSeatNotRenewable
		}

		d.ExpiresAt = ExtendFrom(d.ExpiresAt, now, months)
		r.processed[paymentID] = true

		return nil
	}

	return ErrSeatNotFound
}

func (r *fakeRepo) Bind(
	_ context.Context,
	username string,
	id int64,
	hwid string,
	label string,
) error {
	r.log("repo.bind")

	if r.bindErr != nil {
		return r.bindErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.seats[username] {
		d := &r.seats[username][i]
		if d.ID != id || !d.RevokedAt.IsZero() || d.HWID != "" || !d.ExpiresAt.After(r.clock()) {
			continue
		}

		for _, other := range r.seats[username] {
			if other.HWID == hwid {
				return ErrSeatNotFree
			}
		}

		d.HWID = hwid
		d.Label = label

		return nil
	}

	return ErrSeatNotFree
}

func (r *fakeRepo) UpdateLabel(
	_ context.Context,
	username string,
	id int64,
	label string,
) error {
	r.log("repo.update_label")

	if r.labelErr != nil {
		return r.labelErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.seats[username] {
		d := &r.seats[username][i]
		if d.ID == id && d.RevokedAt.IsZero() && d.HWID != "" {
			d.Label = label
		}
	}

	return nil
}

func (r *fakeRepo) Unbind(_ context.Context, username string, id int64) error {
	r.log("repo.unbind")

	if r.unbindErr != nil {
		return r.unbindErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.seats[username] {
		d := &r.seats[username][i]
		if d.ID == id && d.RevokedAt.IsZero() {
			d.HWID = ""
			d.Label = ""
		}
	}

	return nil
}

func (r *fakeRepo) ListExpiredUsernames(
	_ context.Context,
	now time.Time,
	limit int,
) ([]string, error) {
	r.log("repo.list_expired")

	if r.listErr != nil {
		return nil, r.listErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var out []string
	for username, seats := range r.seats {
		for _, d := range seats {
			if d.ExpiredAt(now) {
				out = append(out, username)
				break
			}
		}
		if len(out) == limit {
			break
		}
	}

	return out, nil
}

func (r *fakeRepo) RevokeExpiredExtra(
	_ context.Context,
	username string,
	now time.Time,
) (int64, error) {
	r.log("repo.revoke_extra")

	if r.revokeErr != nil {
		return 0, r.revokeErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var n int64
	for i := range r.seats[username] {
		d := &r.seats[username][i]
		if !d.Extra() || !d.RevokedAt.IsZero() || d.ExpiresAt.After(now) {
			continue
		}
		d.RevokedAt = now
		d.HWID = ""
		d.Label = ""
		n++
	}

	return n, nil
}

func (r *fakeRepo) seat(id int64) Device {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, seats := range r.seats {
		for _, d := range seats {
			if d.ID == id {
				return d
			}
		}
	}

	return Device{}
}

type updateCall struct {
	userID int
	expire time.Time
	limit  int
}

type fakePanel struct {
	sub          PanelSubscription
	subErr       error
	subErrByUser map[string]error
	devices      []PanelDevice
	devicesErr   error
	deleteErr    error
	updateErr    error
	calls        *[]string

	subUsernames []string
	deleted      []string
	updates      []updateCall
}

func (p *fakePanel) Subscription(
	_ context.Context,
	username string,
) (PanelSubscription, error) {
	p.log("panel.subscription")
	p.subUsernames = append(p.subUsernames, username)

	if err, ok := p.subErrByUser[username]; ok {
		return PanelSubscription{}, err
	}

	return p.sub, p.subErr
}

func (p *fakePanel) ListDevices(_ context.Context, _ int) ([]PanelDevice, error) {
	p.log("panel.devices")
	return p.devices, p.devicesErr
}

func (p *fakePanel) DeleteHWID(_ context.Context, _ int, hwid string) error {
	p.log("panel.delete")
	p.deleted = append(p.deleted, hwid)
	return p.deleteErr
}

func (p *fakePanel) UpdateUser(
	_ context.Context,
	userID int,
	expireAt time.Time,
	deviceLimit int,
) error {
	p.log("panel.update")
	p.updates = append(p.updates, updateCall{userID: userID, expire: expireAt, limit: deviceLimit})
	return p.updateErr
}

func (p *fakePanel) log(s string) {
	*p.calls = append(*p.calls, s)
}

func newServiceForTest(repo Repository, panel Panel, now time.Time) *Service {
	if r, ok := repo.(*fakeRepo); ok {
		r.clock = func() time.Time { return now }
	}

	s := NewService(repo, panel, 2, 5, zap.NewNop())
	s.now = func() time.Time { return now }
	return s
}

func TestListDevicesEnsuresBaseSeatsForNewUser(t *testing.T) {
	now := time.Now()
	expire := now.Add(48 * time.Hour)

	var calls []string
	repo := newFakeRepo(&calls)
	panel := &fakePanel{
		sub:   PanelSubscription{UserID: 7, ExpireAt: expire, DeviceLimit: 0},
		calls: &calls,
	}
	svc := newServiceForTest(repo, panel, now)

	got, err := svc.ListDevices(context.Background(), "42")
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("seats = %d, want 2 base seats", len(got))
	}
	for _, d := range got {
		if !d.Base() || !d.ExpiresAt.Equal(expire) {
			t.Fatalf("seat = %+v, want base with expire %v", d, expire)
		}
	}
}

func TestListDevicesExtendsBaseSeatsOnRenewal(t *testing.T) {
	now := time.Now()
	oldExpire := now.Add(time.Hour)
	newExpire := now.Add(30 * 24 * time.Hour)

	var calls []string
	repo := newFakeRepo(&calls)
	repo.add(Device{ID: 1, Username: "42", Kind: KindBase, ExpiresAt: oldExpire, CreatedAt: now})
	repo.add(Device{ID: 2, Username: "42", Kind: KindBase, ExpiresAt: oldExpire, CreatedAt: now})

	panel := &fakePanel{
		sub:   PanelSubscription{UserID: 7, ExpireAt: newExpire, DeviceLimit: 2},
		calls: &calls,
	}
	svc := newServiceForTest(repo, panel, now)

	got, err := svc.ListDevices(context.Background(), "42")
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}

	for _, d := range got {
		if !d.ExpiresAt.Equal(newExpire) {
			t.Fatalf("seat %d expire = %v, want %v", d.ID, d.ExpiresAt, newExpire)
		}
	}
}

func TestListDevicesBindsNewDeviceWithLabel(t *testing.T) {
	now := time.Now()
	expire := now.Add(24 * time.Hour)

	var calls []string
	repo := newFakeRepo(&calls)
	repo.add(Device{ID: 1, Username: "42", Kind: KindBase, ExpiresAt: expire, CreatedAt: now})
	repo.add(Device{ID: 2, Username: "42", Kind: KindBase, ExpiresAt: expire, CreatedAt: now})

	panel := &fakePanel{
		sub:     PanelSubscription{UserID: 7, ExpireAt: expire, DeviceLimit: 2},
		devices: []PanelDevice{{HWID: "a", Name: "iPhone"}},
		calls:   &calls,
	}
	svc := newServiceForTest(repo, panel, now)

	got, err := svc.ListDevices(context.Background(), "42")
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}

	if got[0].HWID != "a" || got[0].Label != "iPhone" {
		t.Fatalf("first seat = %+v, want bound with label iPhone", got[0])
	}
}

func TestListDevicesDoesNotCreateExtraSeatFromPanel(t *testing.T) {
	now := time.Now()
	expire := now.Add(24 * time.Hour)

	var calls []string
	repo := newFakeRepo(&calls)
	repo.add(Device{ID: 1, Username: "42", Kind: KindBase, ExpiresAt: expire, CreatedAt: now})
	repo.add(Device{ID: 2, Username: "42", Kind: KindBase, ExpiresAt: expire, CreatedAt: now})

	panel := &fakePanel{
		sub:     PanelSubscription{UserID: 7, ExpireAt: expire, DeviceLimit: 2},
		devices: []PanelDevice{{HWID: "a"}, {HWID: "b"}, {HWID: "c"}},
		calls:   &calls,
	}
	svc := newServiceForTest(repo, panel, now)

	got, err := svc.ListDevices(context.Background(), "42")
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("seats = %d, want 2", len(got))
	}
}

func TestListDevicesPanelFailureKeepsDBState(t *testing.T) {
	now := time.Now()
	expire := now.Add(24 * time.Hour)

	var calls []string
	repo := newFakeRepo(&calls)
	repo.add(Device{ID: 1, Username: "42", Kind: KindBase, HWID: "a", ExpiresAt: expire, CreatedAt: now})

	panel := &fakePanel{subErr: errors.New("panel down"), calls: &calls}
	svc := newServiceForTest(repo, panel, now)

	got, err := svc.ListDevices(context.Background(), "42")
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}

	if len(got) != 1 || got[0].HWID != "a" {
		t.Fatalf("seats = %+v, want bound seat a", got)
	}
}

func TestBuyExtraSeatAddsSeatAndPushesLimit(t *testing.T) {
	now := time.Now()
	expire := now.Add(30 * 24 * time.Hour)

	var calls []string
	repo := newFakeRepo(&calls)
	repo.add(Device{ID: 1, Username: "42", Kind: KindBase, ExpiresAt: expire, CreatedAt: now})
	repo.add(Device{ID: 2, Username: "42", Kind: KindBase, ExpiresAt: expire, CreatedAt: now})

	panel := &fakePanel{
		sub:   PanelSubscription{UserID: 7, ExpireAt: expire, DeviceLimit: 2},
		calls: &calls,
	}
	svc := newServiceForTest(repo, panel, now)

	if err := svc.BuyExtraSeat(context.Background(), "42", "pay1", 3); err != nil {
		t.Fatalf("BuyExtraSeat: %v", err)
	}

	devices, err := repo.ListByUser(context.Background(), "42")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(devices) != 3 {
		t.Fatalf("seats = %d, want 3", len(devices))
	}

	if len(panel.updates) != 1 {
		t.Fatalf("updates = %d, want 1", len(panel.updates))
	}
	upd := panel.updates[0]
	if upd.limit != 3 {
		t.Fatalf("limit = %d, want 3", upd.limit)
	}
	if !upd.expire.Equal(expire) {
		t.Fatalf("expire = %v, want subscription expiry %v", upd.expire, expire)
	}
}

func TestBuyExtraSeatRejectsClosedSubscription(t *testing.T) {
	now := time.Now()

	var calls []string
	repo := newFakeRepo(&calls)
	panel := &fakePanel{
		sub:   PanelSubscription{UserID: 7, ExpireAt: now.Add(-time.Hour), DeviceLimit: 2},
		calls: &calls,
	}
	svc := newServiceForTest(repo, panel, now)

	err := svc.BuyExtraSeat(context.Background(), "42", "pay1", 1)
	if !errors.Is(err, ErrSubscriptionClosed) {
		t.Fatalf("error = %v, want ErrSubscriptionClosed", err)
	}
}

func TestBuyExtraSeatRejectsLimitReached(t *testing.T) {
	now := time.Now()
	expire := now.Add(30 * 24 * time.Hour)

	var calls []string
	repo := newFakeRepo(&calls)
	repo.add(Device{ID: 1, Username: "42", Kind: KindBase, ExpiresAt: expire, CreatedAt: now})
	repo.add(Device{ID: 2, Username: "42", Kind: KindBase, ExpiresAt: expire, CreatedAt: now})
	repo.add(Device{ID: 3, Username: "42", Kind: KindExtra, ExpiresAt: expire, CreatedAt: now})
	repo.add(Device{ID: 4, Username: "42", Kind: KindExtra, ExpiresAt: expire, CreatedAt: now})
	repo.add(Device{ID: 5, Username: "42", Kind: KindExtra, ExpiresAt: expire, CreatedAt: now})

	panel := &fakePanel{
		sub:   PanelSubscription{UserID: 7, ExpireAt: expire, DeviceLimit: 5},
		calls: &calls,
	}
	svc := newServiceForTest(repo, panel, now)

	err := svc.BuyExtraSeat(context.Background(), "42", "pay1", 1)
	if !errors.Is(err, ErrDeviceLimitReached) {
		t.Fatalf("error = %v, want ErrDeviceLimitReached", err)
	}
}

func TestCanBuyExtraChecksSubscriptionAndLimit(t *testing.T) {
	now := time.Now()
	expire := now.Add(30 * 24 * time.Hour)

	var calls []string
	repo := newFakeRepo(&calls)
	repo.add(Device{ID: 1, Username: "42", Kind: KindBase, ExpiresAt: expire, CreatedAt: now})
	repo.add(Device{ID: 2, Username: "42", Kind: KindBase, ExpiresAt: expire, CreatedAt: now})

	closed := &fakePanel{
		sub:   PanelSubscription{UserID: 7, ExpireAt: now.Add(-time.Hour), DeviceLimit: 2},
		calls: &calls,
	}
	svc := newServiceForTest(repo, closed, now)
	if err := svc.CanBuyExtra(context.Background(), "42"); !errors.Is(err, ErrSubscriptionClosed) {
		t.Fatalf("closed error = %v, want ErrSubscriptionClosed", err)
	}

	repo.add(Device{ID: 3, Username: "42", Kind: KindExtra, ExpiresAt: expire, CreatedAt: now})
	repo.add(Device{ID: 4, Username: "42", Kind: KindExtra, ExpiresAt: expire, CreatedAt: now})
	repo.add(Device{ID: 5, Username: "42", Kind: KindExtra, ExpiresAt: expire, CreatedAt: now})

	full := &fakePanel{
		sub:   PanelSubscription{UserID: 7, ExpireAt: expire, DeviceLimit: 5},
		calls: &calls,
	}
	svc = newServiceForTest(repo, full, now)
	if err := svc.CanBuyExtra(context.Background(), "42"); !errors.Is(err, ErrDeviceLimitReached) {
		t.Fatalf("full error = %v, want ErrDeviceLimitReached", err)
	}
}

func TestBuyExtraSeatIdempotent(t *testing.T) {
	now := time.Now()
	expire := now.Add(30 * 24 * time.Hour)

	var calls []string
	repo := newFakeRepo(&calls)
	repo.add(Device{ID: 1, Username: "42", Kind: KindBase, ExpiresAt: expire, CreatedAt: now})
	repo.add(Device{ID: 2, Username: "42", Kind: KindBase, ExpiresAt: expire, CreatedAt: now})

	panel := &fakePanel{
		sub:   PanelSubscription{UserID: 7, ExpireAt: expire, DeviceLimit: 2},
		calls: &calls,
	}
	svc := newServiceForTest(repo, panel, now)

	if err := svc.BuyExtraSeat(context.Background(), "42", "pay1", 1); err != nil {
		t.Fatalf("BuyExtraSeat: %v", err)
	}
	if err := svc.BuyExtraSeat(context.Background(), "42", "pay1", 1); err != nil {
		t.Fatalf("BuyExtraSeat second: %v", err)
	}

	devices, _ := repo.ListByUser(context.Background(), "42")
	if len(devices) != 3 {
		t.Fatalf("seats = %d, want 3", len(devices))
	}
}

func TestExtendSeatExtendsExtraSeat(t *testing.T) {
	now := time.Now()
	expire := now.Add(30 * 24 * time.Hour)

	var calls []string
	repo := newFakeRepo(&calls)
	repo.add(Device{ID: 1, Username: "42", Kind: KindBase, ExpiresAt: expire, CreatedAt: now})
	repo.add(Device{ID: 2, Username: "42", Kind: KindBase, ExpiresAt: expire, CreatedAt: now})
	repo.add(Device{
		ID:        3,
		Username:  "42",
		Kind:      KindExtra,
		ExpiresAt: now.Add(10 * 24 * time.Hour),
		CreatedAt: now,
	},
	)

	panel := &fakePanel{calls: &calls}
	svc := newServiceForTest(repo, panel, now)

	if err := svc.ExtendSeat(context.Background(), "42", 3, "pay1", 1); err != nil {
		t.Fatalf("ExtendSeat: %v", err)
	}

	got := repo.seat(3)
	want := now.Add(10*24*time.Hour).AddDate(0, 1, 0)
	if !got.ExpiresAt.Equal(want) {
		t.Fatalf("expire = %v, want %v", got.ExpiresAt, want)
	}
}

func TestExtendSeatRejectsBaseSeat(t *testing.T) {
	now := time.Now()
	expire := now.Add(24 * time.Hour)

	var calls []string
	repo := newFakeRepo(&calls)
	repo.add(Device{ID: 1, Username: "42", Kind: KindBase, ExpiresAt: expire, CreatedAt: now})

	panel := &fakePanel{calls: &calls}
	svc := newServiceForTest(repo, panel, now)

	err := svc.ExtendSeat(context.Background(), "42", 1, "pay1", 1)
	if !errors.Is(err, ErrSeatNotRenewable) {
		t.Fatalf("error = %v, want ErrSeatNotRenewable", err)
	}
}

func TestExtendSeatRejectsExpiredSeat(t *testing.T) {
	now := time.Now()

	var calls []string
	repo := newFakeRepo(&calls)
	repo.add(Device{
		ID:        3,
		Username:  "42",
		Kind:      KindExtra,
		ExpiresAt: now.Add(-time.Hour),
		CreatedAt: now,
	},
	)

	panel := &fakePanel{calls: &calls}
	svc := newServiceForTest(repo, panel, now)

	err := svc.ExtendSeat(context.Background(), "42", 3, "pay1", 1)
	if !errors.Is(err, ErrSeatNotRenewable) {
		t.Fatalf("error = %v, want ErrSeatNotRenewable", err)
	}
}

func TestExtendSeatIdempotent(t *testing.T) {
	now := time.Now()
	expire := now.Add(30 * 24 * time.Hour)

	var calls []string
	repo := newFakeRepo(&calls)
	repo.add(Device{ID: 3, Username: "42", Kind: KindExtra, ExpiresAt: expire, CreatedAt: now})

	panel := &fakePanel{calls: &calls}
	svc := newServiceForTest(repo, panel, now)

	if err := svc.ExtendSeat(context.Background(), "42", 3, "pay1", 1); err != nil {
		t.Fatalf("ExtendSeat: %v", err)
	}
	if err := svc.ExtendSeat(context.Background(), "42", 3, "pay1", 1); err != nil {
		t.Fatalf("ExtendSeat second: %v", err)
	}

	got := repo.seat(3)
	want := expire.AddDate(0, 1, 0)
	if !got.ExpiresAt.Equal(want) {
		t.Fatalf("expire = %v, want %v (no double extension)", got.ExpiresAt, want)
	}
}

func TestRemoveDeviceUnbinds(t *testing.T) {
	now := time.Now()

	var calls []string
	repo := newFakeRepo(&calls)
	repo.add(Device{
		ID:        1,
		Username:  "42",
		Kind:      KindBase,
		HWID:      "a",
		ExpiresAt: now.Add(time.Hour),
		CreatedAt: now,
	},
	)

	panel := &fakePanel{sub: PanelSubscription{UserID: 7}, calls: &calls}
	svc := newServiceForTest(repo, panel, now)

	if err := svc.RemoveDevice(context.Background(), "42", 1); err != nil {
		t.Fatalf("RemoveDevice: %v", err)
	}

	if got := repo.seat(1).HWID; got != "" {
		t.Fatalf("hwid = %q, want empty", got)
	}
}

func TestExpireSeatsRevokesExpiredExtraAndDecrementsLimit(t *testing.T) {
	now := time.Now()
	expire := now.Add(30 * 24 * time.Hour)

	var calls []string
	repo := newFakeRepo(&calls)
	repo.add(Device{ID: 1, Username: "42", Kind: KindBase, ExpiresAt: expire, CreatedAt: now})
	repo.add(Device{ID: 2, Username: "42", Kind: KindBase, ExpiresAt: expire, CreatedAt: now})
	repo.add(Device{
		ID:        3,
		Username:  "42",
		Kind:      KindExtra,
		HWID:      "expired",
		ExpiresAt: now.Add(-time.Hour),
		CreatedAt: now,
	},
	)

	panel := &fakePanel{
		sub:     PanelSubscription{UserID: 7, ExpireAt: expire, DeviceLimit: 3},
		devices: []PanelDevice{{HWID: "expired"}},
		calls:   &calls,
	}
	svc := newServiceForTest(repo, panel, now)

	if err := svc.ExpireSeats(context.Background()); err != nil {
		t.Fatalf("ExpireSeats: %v", err)
	}

	if got := repo.seat(3).RevokedAt; got.IsZero() {
		t.Fatal("extra seat not revoked")
	}
	if len(panel.deleted) != 1 || panel.deleted[0] != "expired" {
		t.Fatalf("deleted = %v, want [expired]", panel.deleted)
	}
	if len(panel.updates) != 1 {
		t.Fatalf("updates = %d, want 1", len(panel.updates))
	}
	if panel.updates[0].limit != 2 {
		t.Fatalf("limit = %d, want 2", panel.updates[0].limit)
	}
}

func TestExpireSeatsKeepsBaseSeatsWhenSubInactive(t *testing.T) {
	now := time.Now()

	var calls []string
	repo := newFakeRepo(&calls)
	repo.add(Device{
		ID:        1,
		Username:  "42",
		Kind:      KindBase,
		HWID:      "a",
		ExpiresAt: now.Add(-time.Hour),
		CreatedAt: now,
	},
	)

	panel := &fakePanel{
		sub:     PanelSubscription{UserID: 7, ExpireAt: now.Add(-time.Hour), DeviceLimit: 2},
		devices: []PanelDevice{{HWID: "a"}},
		calls:   &calls,
	}
	svc := newServiceForTest(repo, panel, now)

	if err := svc.ExpireSeats(context.Background()); err != nil {
		t.Fatalf("ExpireSeats: %v", err)
	}

	seat := repo.seat(1)
	if !seat.RevokedAt.IsZero() {
		t.Fatalf("base seat revoked, want kept")
	}
	if seat.HWID != "a" {
		t.Fatalf("hwid = %q, want kept a", seat.HWID)
	}
}

func TestExpireSeatsPanelFailureDoesNotRevoke(t *testing.T) {
	now := time.Now()

	var calls []string
	repo := newFakeRepo(&calls)
	repo.add(Device{
		ID:        3,
		Username:  "42",
		Kind:      KindExtra,
		HWID:      "a",
		ExpiresAt: now.Add(-time.Hour),
		CreatedAt: now,
	},
	)

	panel := &fakePanel{subErr: errors.New("panel down"), calls: &calls}
	svc := newServiceForTest(repo, panel, now)

	if err := svc.ExpireSeats(context.Background()); err == nil {
		t.Fatal("ExpireSeats error = nil, want error")
	}

	if got := repo.seat(3).RevokedAt; !got.IsZero() {
		t.Fatalf("revoked_at = %v, want zero", got)
	}
}
