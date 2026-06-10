// Package domain содержит доменные типы для платежей.
package domain

import "errors"

// ErrInsufficientFunds недостаточно средств для операции.
var ErrInsufficientFunds = errors.New("insufficient funds")

// PaymentStatus статус платежа.
type PaymentStatus string

const (
	PaymentStatusPending PaymentStatus = "pending"
	PaymentStatusSuccess PaymentStatus = "success"
	PaymentStatusFailed  PaymentStatus = "failed"
)
