package userscase

import (
	"context"
	"errors"
	"testing"
	"time"

	subdomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/domain"
	"github.com/stretchr/testify/require"
)

// fakeUserRepository — ручная реализация UserRepository для тестов.
// Каждый метод — функция-поле: не задал функцию, но метод вызвался -> nil pointer panic,
// который сразу укажет "бизнес-логика дёрнула то, чего не должна была".
type fakeUserRepository struct {
	getByUsernameFunc func(ctx context.Context, username string) (subdomain.UserResponse, error)
	getByUUIDFunc     func(ctx context.Context, uuid string) (subdomain.UserResponse, error)
	createUserFunc    func(ctx context.Context, username string, days int) (subdomain.User, error)
}

func (f *fakeUserRepository) GetByUsername(
	ctx context.Context,
	username string,
) (subdomain.UserResponse, error) {
	return f.getByUsernameFunc(ctx, username)
}

func (f *fakeUserRepository) GetByUUID(
	ctx context.Context,
	uuid string,
) (subdomain.UserResponse, error) {
	return f.getByUUIDFunc(ctx, uuid)
}

func (f *fakeUserRepository) CreateUser(
	ctx context.Context,
	username string,
	days int,
) (subdomain.User, error) {
	return f.createUserFunc(ctx, username, days)
}

// nolint: funlen
func TestUserUseCase_GetOrCreateSub(t *testing.T) {
	t.Parallel()

	futureExpire := time.Now().Add(72 * time.Hour).Format(time.RFC3339)

	tests := []struct {
		name          string
		repo          *fakeUserRepository
		wantErr       bool
		wantErrSubstr string
		check         func(t *testing.T, got subdomain.User)
	}{
		{
			name: "юзер не найден -> создаём нового на 30 дней",
			repo: &fakeUserRepository{
				getByUsernameFunc: func(ctx context.Context, username string) (subdomain.UserResponse, error) {
					return subdomain.UserResponse{}, subdomain.ErrNoFindUser
				},
				createUserFunc: func(ctx context.Context, username string, days int) (subdomain.User, error) {
					require.Equal(t, "vlad", username)
					require.Equal(t, 30, days)

					return subdomain.User{Name: username, Days: days}, nil
				},
			},
			check: func(t *testing.T, got subdomain.User) {
				require.Equal(t, "vlad", got.Name)
				require.Equal(t, 30, got.Days)
			},
		},
		{
			name: "репозиторий вернул произвольную ошибку -> пробрасывается как есть",
			repo: &fakeUserRepository{
				getByUsernameFunc: func(ctx context.Context, username string) (subdomain.UserResponse, error) {
					return subdomain.UserResponse{}, errors.New("db unavailable")
				},
			},
			wantErr:       true,
			wantErrSubstr: "db unavailable",
		},
		{
			name: "существующий юзер с валидной датой -> корректный маппинг + remainingDays",
			repo: &fakeUserRepository{
				getByUsernameFunc: func(ctx context.Context, username string) (subdomain.UserResponse, error) {
					return subdomain.UserResponse{
						Username:        "people1",
						UUID:            "uuid-123",
						HWIDDeviceLimit: 3,
						SubscriptionURL: "https://sub.example.com/vlad",
						ExpireAt:        futureExpire,
					}, nil
				},
			},
			check: func(t *testing.T, got subdomain.User) {
				require.Equal(t, "people1", got.Name)
				require.Equal(t, "uuid-123", got.UUID)
				require.Equal(t, 3, got.Device)
				require.Equal(t, "https://sub.example.com/vlad", got.URL)
				require.InDelta(t, 3, got.Days, 1) // допуск на округление до суток
			},
		},
		{
			name: "битая дата ExpireAt -> ошибка парсинга",
			repo: &fakeUserRepository{
				getByUsernameFunc: func(ctx context.Context, username string) (subdomain.UserResponse, error) {
					return subdomain.UserResponse{Username: "people1", ExpireAt: "not-a-date"}, nil
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := NewUserUseCase(tt.repo)
			got, err := uc.GetOrCreateSub(context.Background(), "vlad", 30)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrSubstr != "" {
					require.ErrorContains(t, err, tt.wantErrSubstr)
				}

				return
			}
			require.NoError(t, err)
			tt.check(t, got)
		})
	}
}

func TestUserUseCase_GetURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		repo          *fakeUserRepository
		wantErr       bool
		wantErrSubstr string
		want          subdomain.User
	}{
		{
			name: "успех",
			repo: &fakeUserRepository{
				getByUsernameFunc: func(ctx context.Context, username string) (subdomain.UserResponse, error) {
					return subdomain.UserResponse{
						Username:        "vlad",
						UUID:            "uuid-123",
						HWIDDeviceLimit: 3,
						SubscriptionURL: "https://sub.example.com/vlad",
					}, nil
				},
			},
			want: subdomain.User{
				Name:   "vlad",
				UUID:   "uuid-123",
				Device: 3,
				URL:    "https://sub.example.com/vlad",
			},
		},
		{
			name: "ошибка репозитория пробрасывается",
			repo: &fakeUserRepository{
				getByUsernameFunc: func(ctx context.Context, username string) (subdomain.UserResponse, error) {
					return subdomain.UserResponse{}, errors.New("timeout")
				},
			},
			wantErr:       true,
			wantErrSubstr: "timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := NewUserUseCase(tt.repo)
			got, err := uc.GetURL(context.Background(), "vlad")

			if tt.wantErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErrSubstr)

				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
