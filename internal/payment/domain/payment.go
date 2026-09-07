package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type TransactionStatus string

const (
	StatusPending TransactionStatus = "pending"
	StatusSuccess TransactionStatus = "success"
	StatusFailed  TransactionStatus = "failed"
)

var (
	ErrInvalidAmount    = errors.New("сумма должна быть положительной")
	ErrAlreadyProcessed = errors.New("транзакция уже обработана")
	ErrNotFound         = errors.New("транзакция не найдена")
)

type Transaction struct {
	UUID       string
	UserID     string
	Amount     int
	Status     TransactionStatus
	Provider   string
	ExternalID string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewTransaction создаёт транзакцию в статусе pending.
// Единственная проверка, которая универсальна для ЛЮБОГО платежа: сумма положительная.
func NewTransaction(userID string, amount int, provider string) (Transaction, error) {
	if amount <= 0 {
		return Transaction{}, ErrInvalidAmount
	}

	now := time.Now()

	return Transaction{
		UUID:      uuid.NewString(),
		UserID:    userID,
		Amount:    amount,
		Status:    StatusPending,
		Provider:  provider,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
