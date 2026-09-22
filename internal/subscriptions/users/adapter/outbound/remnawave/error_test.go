package remnawave

import (
	"context"
	"errors"
	"sync"
	"testing"

	platformremnawave "github.com/VladMallory/ProxyMaster_v2/internal/platform/remnawave"
	subdomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/domain"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type recordingNotifier struct {
	mu    sync.Mutex
	calls int
	errs  []error
	metas []ErrorMeta
}

func (r *recordingNotifier) Notify(_ context.Context, err error, meta ErrorMeta) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	r.errs = append(r.errs, err)
	r.metas = append(r.metas, meta)
}

func (r *recordingNotifier) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.calls
}

func (r *recordingNotifier) last() (ErrorMeta, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.errs) == 0 {
		return ErrorMeta{}, nil
	}

	return r.metas[len(r.metas)-1], r.errs[len(r.errs)-1]
}

//nolint:funlen
func TestErrorHandler_Map(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	boom := errors.New("boom")
	wrappedNotFound := errors.Join(errors.New("wrap"), platformremnawave.ErrNotFound)

	tests := []struct {
		name         string
		err          error
		meta         ErrorMeta
		nilNotifier  bool
		wantErr      error
		wantErrSubst string
		wantNotify   bool
	}{
		{
			name:       "nil ошибка возвращает nil и не зовет админа",
			err:        nil,
			meta:       ErrorMeta{Op: "GetByUsername", Username: "873925520"},
			wantErr:    nil,
			wantNotify: false,
		},
		{
			name:       "ErrNotFound маппится в ErrNoFindUser и зовет админа",
			err:        platformremnawave.ErrNotFound,
			meta:       ErrorMeta{Op: "GetByUsername", Username: "873925520"},
			wantErr:    subdomain.ErrNoFindUser,
			wantNotify: true,
		},
		{
			name:       "обернутый ErrNotFound тоже маппится и зовет админа",
			err:        wrappedNotFound,
			meta:       ErrorMeta{Op: "GetByUsername", Username: "873925520"},
			wantErr:    subdomain.ErrNoFindUser,
			wantNotify: true,
		},
		{
			name:         "обычная ошибка оборачивается с op и зовет админа",
			err:          boom,
			meta:         ErrorMeta{Op: "CreateUser", Username: "vlad"},
			wantErrSubst: "CreateUser: boom",
			wantNotify:   true,
		},
		{
			name:         "nil нотифаер не паникует и все равно маппит",
			err:          boom,
			meta:         ErrorMeta{Op: "CreateUser", Username: "vlad"},
			nilNotifier:  true,
			wantErrSubst: "CreateUser: boom",
			wantNotify:   false,
		},
		{
			name:        "nil нотифаер с ErrNotFound не паникует",
			err:         platformremnawave.ErrNotFound,
			meta:        ErrorMeta{Op: "GetByUsername", Username: "873925520"},
			nilNotifier: true,
			wantErr:     subdomain.ErrNoFindUser,
			wantNotify:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := &recordingNotifier{}
			var n Notifier
			n = rec
			if tt.nilNotifier {
				n = nil
			}
			h := NewErrorHandler(zap.NewNop(), n)

			var got error
			require.NotPanics(t, func() {
				got = h.Map(ctx, tt.err, tt.meta)
			})

			switch {
			case tt.wantErr != nil:
				require.ErrorIs(t, got, tt.wantErr)
			case tt.wantErrSubst != "":
				require.Error(t, got)
				require.ErrorContains(t, got, tt.wantErrSubst)
				require.ErrorIs(t, got, tt.err)
			default:
				require.NoError(t, got)
			}

			if tt.wantNotify {
				require.Equal(t, 1, rec.count())
				gotMeta, gotErr := rec.last()
				require.Equal(t, tt.meta, gotMeta)
				require.ErrorIs(t, gotErr, tt.err)
			} else if !tt.nilNotifier {
				require.Equal(t, 0, rec.count())
			}
		})
	}
}

func TestErrorHandler_Map_NotifyGetsOriginalError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rec := &recordingNotifier{}
	h := NewErrorHandler(zap.NewNop(), rec)
	sentinel := errors.New("db down")
	meta := ErrorMeta{Op: "ExtendExpire", Username: "vlad"}

	got := h.Map(ctx, sentinel, meta)

	require.Error(t, got)
	require.ErrorContains(t, got, "ExtendExpire")
	require.Equal(t, 1, rec.count())
	gotMeta, gotErr := rec.last()
	require.Equal(t, meta, gotMeta)
	require.Equal(t, sentinel, gotErr)
}
