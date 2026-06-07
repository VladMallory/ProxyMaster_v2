// Package domain содержит доменные типы для платежей.
package domain

// PaymentStatus статус платежа.
type PaymentStatus string

const (
	PaymentStatusPending PaymentStatus = "pending"
	PaymentStatusSuccess PaymentStatus = "success"
	PaymentStatusFailed  PaymentStatus = "failed"
)
