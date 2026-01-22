package database

import (
	"ProxyMaster_v2/internal/models"
	"ProxyMaster_v2/pkg/logger"
	"time"
)

func (s *UserStorage) AddSquad(squadData models.Squad) error {
	query := `
	INSERT INTO squads (title, user_id, uuid, expires_at)
	VALUES ($1, $2, $3, $4)
	`

	if _, err := s.db.Exec(
		query,
		squadData.Title,
		squadData.UserID,
		squadData.UUID,
		squadData.ExpiresAt,
	); err != nil {
		s.logger.Error(
			"failed to add squad in db",
			logger.Field{Key: "err_msg", Value: err},
		)

		return err
	}

	return nil

}

func (s *UserStorage) RemoveSquad(userID string) error {
	query := `
		DELETE FROM squads
		WHERE user_id = $1
	`

	result, err := s.db.Exec(query, userID)

	if err != nil {
		s.logger.Error(
			"failed to delete squad from db",
			logger.Field{Key: "err_msg", Value: err},
		)

		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		s.logger.Warn(
			"запись не была удалена",
			logger.Field{Key: "userID", Value: userID},
		)

		return ErrSquadNotDeleted
	}

	return nil
}

func (s *UserStorage) GetAllExpiredSquads() ([]models.Squad, error) {
	query := `
		SELECT (user_id, uuid)
		FROM squads
		WHERE expires_at < $1
	`

	var expiredSquads []models.Squad

	currentTime := time.Now()
	formatedTime := currentTime.Format("2006-01-02 15:04:05")

	if err := s.db.Select(&expiredSquads, query, formatedTime); err != nil {
		s.logger.Error(
			"failed to get expired squads",
			logger.Field{Key: "err_msg", Value: err},
		)

		return nil, err
	}

	s.logger.Info("истекшие squads успешно получены")

	if err := s.removeExpiredSquads(formatedTime); err != nil {
		return nil, err
	}

	s.logger.Debug("все четко")

	return expiredSquads, nil
}

func (s *UserStorage) removeExpiredSquads(formatedTime string) error {
	query := `
		DELETE FROM squads
		WHERE expires_at < $1
	`

	//formatedTime := date.Format("2006-01-02 15:04:05")

	res, err := s.db.Exec(query, formatedTime)

	if err != nil {
		s.logger.Error(
			"failed to remove expired squads from db",
			logger.Field{Key: "err_msg", Value: err},
		)

		return err
	}

	rowsAffected, _ := res.RowsAffected()

	if rowsAffected == 0 {
		s.logger.Warn("записи не были удалены")

		return ErrSquadNotDeleted
	}

	s.logger.Info(
		"истекшие squads успешно удалены",
		logger.Field{Key: "удалено", Value: rowsAffected},
	)

	return nil
}
