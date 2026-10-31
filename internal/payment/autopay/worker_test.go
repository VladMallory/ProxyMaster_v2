package autopay

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// testNow — 2023-11-14T22:13:20Z, фиксированное «сейчас» для воркера.
var testNow = time.Unix(1_700_000_000, 0).UTC()

type chargeCall struct {
	methodID string
	amount   int
	key      string
}

type fakePayments struct {
	chargeID    string
	chargeErr   error
	status      string
	statusErr   error
	chargeCalls []chargeCall
	statusCalls []string
}

func (f *fakePayments) Charge(
	_ context.Context,
	methodID string,
	amount int,
	key string,
) (string, error) {
	f.chargeCalls = append(f.chargeCalls, chargeCall{
		methodID: methodID,
		amount:   amount,
		key:      key,
	})

	if f.chargeErr != nil {
		return "", f.chargeErr
	}

	return f.chargeID, nil
}

func (f *fakePayments) Status(_ context.Context, paymentID string) (string, error) {
	f.statusCalls = append(f.statusCalls, paymentID)

	if f.statusErr != nil {
		return "", f.statusErr
	}

	return f.status, nil
}

type extendCall struct {
	username string
	months   int
}

type fakePanel struct {
	expireAt     time.Time
	expireErr    error
	expireErrFor map[string]error
	extendErr    error
	extendCalls  []extendCall
}

func (f *fakePanel) ExpireAt(_ context.Context, username string) (time.Time, error) {
	if err := f.expireErrFor[username]; err != nil {
		return time.Time{}, err
	}

	if f.expireErr != nil {
		return time.Time{}, f.expireErr
	}

	return f.expireAt, nil
}

func (f *fakePanel) ExtendMonths(_ context.Context, username string, months int) error {
	f.extendCalls = append(f.extendCalls, extendCall{
		username: username,
		months:   months,
	})

	return f.extendErr
}

type setPendingCall struct {
	username  string
	paymentID string
	target    time.Time
}

type fakeStore struct {
	records         []record
	listErr         error
	setPendingErr   error
	clearErr        error
	deleteErr       error
	setPendingCalls []setPendingCall
	clearCalls      []string
	deleteCalls     []string
}

func (f *fakeStore) List(context.Context) ([]record, error) {
	return f.records, f.listErr
}

func (f *fakeStore) SetPending(
	_ context.Context,
	username, paymentID string,
	target time.Time,
) error {
	f.setPendingCalls = append(f.setPendingCalls, setPendingCall{
		username:  username,
		paymentID: paymentID,
		target:    target,
	})

	return f.setPendingErr
}

func (f *fakeStore) ClearPending(_ context.Context, username string) error {
	f.clearCalls = append(f.clearCalls, username)

	return f.clearErr
}

func (f *fakeStore) Delete(_ context.Context, username string) error {
	f.deleteCalls = append(f.deleteCalls, username)

	return f.deleteErr
}

type paidCall struct {
	userID string
	months int
	amount int
}

type fakeNotifier struct {
	paid   []paidCall
	failed []string
}

func (f *fakeNotifier) Paid(userID string, months, amountRub int) {
	f.paid = append(f.paid, paidCall{
		userID: userID,
		months: months,
		amount: amountRub,
	})
}

func (f *fakeNotifier) Failed(userID string) {
	f.failed = append(f.failed, userID)
}

func newTestWorker(
	st *fakeStore,
	p *fakePayments,
	pl *fakePanel,
	n *fakeNotifier,
) *worker {
	w := newWorker(st, p, pl, n)
	w.now = func() time.Time { return testNow }

	return w
}

func TestProcess_DoesNotChargeBeforeLeadTime(t *testing.T) {
	t.Parallel()

	st := &fakeStore{records: []record{{
		Username:      "u",
		MethodID:      "pm",
		Months:        1,
		AmountKopecks: 10000,
	}}}
	p := &fakePayments{}
	pl := &fakePanel{expireAt: testNow.Add(48 * time.Hour)}
	n := &fakeNotifier{}
	w := newTestWorker(st, p, pl, n)

	require.NoError(t, w.process(context.Background(), st.records[0]))
	require.Empty(t, p.chargeCalls)
	require.Empty(t, st.setPendingCalls)
	require.Empty(t, pl.extendCalls)
}

func TestProcess_ChargesWithinLeadTimeAndExtends(t *testing.T) {
	t.Parallel()

	expireAt := time.Unix(1_700_043_200, 0).UTC() // testNow + 12h
	st := &fakeStore{records: []record{{
		Username:      "u",
		MethodID:      "pm",
		Months:        1,
		AmountKopecks: 10000,
	}}}
	p := &fakePayments{chargeID: "pay-1", status: "succeeded"}
	pl := &fakePanel{expireAt: expireAt}
	n := &fakeNotifier{}
	w := newTestWorker(st, p, pl, n)

	require.NoError(t, w.process(context.Background(), st.records[0]))

	require.Equal(t, []chargeCall{{
		methodID: "pm",
		amount:   10000,
		key:      "autopay-u-1700043200",
	}}, p.chargeCalls)

	require.Equal(t, []setPendingCall{{
		username:  "u",
		paymentID: "pay-1",
		target:    time.Date(2023, 12, 15, 10, 13, 20, 0, time.UTC),
	}}, st.setPendingCalls)

	require.Equal(t, []extendCall{{username: "u", months: 1}}, pl.extendCalls)
	require.Equal(t, []string{"u"}, st.clearCalls)
	require.Equal(t, []paidCall{{userID: "u", months: 1, amount: 100}}, n.paid)
}

func TestProcess_SettlesPendingWithoutCharging(t *testing.T) {
	t.Parallel()

	st := &fakeStore{records: []record{{
		Username:         "u",
		MethodID:         "pm",
		Months:           1,
		AmountKopecks:    10000,
		PendingPaymentID: "pay-9",
	}}}
	p := &fakePayments{status: "pending"}
	pl := &fakePanel{}
	n := &fakeNotifier{}
	w := newTestWorker(st, p, pl, n)

	require.NoError(t, w.process(context.Background(), st.records[0]))
	require.Empty(t, p.chargeCalls)
	require.Equal(t, []string{"pay-9"}, p.statusCalls)
	require.Empty(t, st.deleteCalls)
}

func TestProcess_CanceledChargeDropsAutopay(t *testing.T) {
	t.Parallel()

	st := &fakeStore{records: []record{{
		Username:         "u",
		MethodID:         "pm",
		Months:           1,
		AmountKopecks:    10000,
		PendingPaymentID: "pay-9",
	}}}
	p := &fakePayments{status: "canceled"}
	pl := &fakePanel{}
	n := &fakeNotifier{}
	w := newTestWorker(st, p, pl, n)

	require.NoError(t, w.process(context.Background(), st.records[0]))
	require.Equal(t, []string{"u"}, st.deleteCalls)
	require.Equal(t, []string{"u"}, n.failed)
	require.Empty(t, pl.extendCalls)
}

func TestProcess_MissingPanelUserDropsAutopay(t *testing.T) {
	t.Parallel()

	st := &fakeStore{records: []record{{
		Username:      "u",
		MethodID:      "pm",
		Months:        1,
		AmountKopecks: 10000,
	}}}
	p := &fakePayments{}
	pl := &fakePanel{expireErr: errUserNotFound}
	n := &fakeNotifier{}
	w := newTestWorker(st, p, pl, n)

	require.NoError(t, w.process(context.Background(), st.records[0]))
	require.Equal(t, []string{"u"}, st.deleteCalls)
	require.Equal(t, []string{"u"}, n.failed)
	require.Empty(t, p.chargeCalls)
}

func TestProcess_ExpiredSubscriptionDropsAutopay(t *testing.T) {
	t.Parallel()

	st := &fakeStore{records: []record{{
		Username:      "u",
		MethodID:      "pm",
		Months:        1,
		AmountKopecks: 10000,
	}}}
	p := &fakePayments{}
	pl := &fakePanel{expireAt: testNow.Add(-time.Hour)}
	n := &fakeNotifier{}
	w := newTestWorker(st, p, pl, n)

	require.NoError(t, w.process(context.Background(), st.records[0]))
	require.Equal(t, []string{"u"}, st.deleteCalls)
	require.Equal(t, []string{"u"}, n.failed)
	require.Empty(t, p.chargeCalls)
}

func TestProcess_StatusErrorKeepsState(t *testing.T) {
	t.Parallel()

	st := &fakeStore{records: []record{{
		Username:         "u",
		MethodID:         "pm",
		Months:           1,
		AmountKopecks:    10000,
		PendingPaymentID: "pay-9",
	}}}
	p := &fakePayments{statusErr: errors.New("boom")}
	pl := &fakePanel{}
	n := &fakeNotifier{}
	w := newTestWorker(st, p, pl, n)

	require.Error(t, w.process(context.Background(), st.records[0]))
	require.Empty(t, st.deleteCalls)
	require.Empty(t, st.clearCalls)
	require.Empty(t, pl.extendCalls)
}

func TestProcess_ChargeErrorDoesNotCreatePending(t *testing.T) {
	t.Parallel()

	st := &fakeStore{records: []record{{
		Username:      "u",
		MethodID:      "pm",
		Months:        1,
		AmountKopecks: 10000,
	}}}
	p := &fakePayments{chargeErr: errors.New("boom")}
	pl := &fakePanel{expireAt: testNow.Add(12 * time.Hour)}
	n := &fakeNotifier{}
	w := newTestWorker(st, p, pl, n)

	require.Error(t, w.process(context.Background(), st.records[0]))
	require.Empty(t, st.setPendingCalls)
	require.Empty(t, pl.extendCalls)
}

func TestProcess_DoesNotExtendAlreadyExtendedSubscription(t *testing.T) {
	t.Parallel()

	expireAt := time.Unix(1_700_043_200, 0).UTC() // testNow + 12h
	st := &fakeStore{records: []record{{
		Username:         "u",
		MethodID:         "pm",
		Months:           1,
		AmountKopecks:    10000,
		PendingPaymentID: "pay-9",
		PendingTarget:    expireAt.Add(-time.Hour),
	}}}
	p := &fakePayments{status: "succeeded"}
	pl := &fakePanel{expireAt: expireAt}
	n := &fakeNotifier{}
	w := newTestWorker(st, p, pl, n)

	require.NoError(t, w.process(context.Background(), st.records[0]))
	require.Empty(t, pl.extendCalls)
	require.Equal(t, []string{"u"}, st.clearCalls)
	require.Equal(t, []paidCall{{userID: "u", months: 1, amount: 100}}, n.paid)
}

func TestRunOnce_ContinuesAfterRecordError(t *testing.T) {
	t.Parallel()

	st := &fakeStore{records: []record{
		{Username: "bad", MethodID: "pm", Months: 1, AmountKopecks: 10000},
		{Username: "good", MethodID: "pm", Months: 1, AmountKopecks: 10000},
	}}
	p := &fakePayments{chargeID: "pay-1", status: "succeeded"}
	pl := &fakePanel{
		expireAt:     testNow.Add(12 * time.Hour),
		expireErrFor: map[string]error{"bad": errors.New("panel down")},
	}
	n := &fakeNotifier{}
	w := newTestWorker(st, p, pl, n)

	w.runOnce(context.Background())

	require.Len(t, p.chargeCalls, 1)
	require.Equal(t, "pm", p.chargeCalls[0].methodID)
}

func TestRunOnce_ListErrorDoesNothing(t *testing.T) {
	t.Parallel()

	st := &fakeStore{listErr: errors.New("db down")}
	p := &fakePayments{}
	pl := &fakePanel{}
	n := &fakeNotifier{}
	w := newTestWorker(st, p, pl, n)

	w.runOnce(context.Background())

	require.Empty(t, p.chargeCalls)
	require.Empty(t, st.setPendingCalls)
}
