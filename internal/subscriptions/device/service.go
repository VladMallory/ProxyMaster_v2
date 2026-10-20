package device

import "context"

type Repository interface {
	ListByUser(ctx context.Context, username string) ([]Device, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListDevices(ctx context.Context, username string) ([]Device, error) {
	if username == "" {
		return nil, errEmptyUsername
	}

	devices, err := s.repo.ListByUser(ctx, username)
	if err != nil {
		return nil, err
	}

	return devices, nil
}
