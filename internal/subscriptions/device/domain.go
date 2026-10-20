package device

import "errors"

var errEmptyUsername = errors.New("пустое поле username")

type Device struct {
	ID   int64
	Name string
	HWID string
}

// Bound сообщает привязано ли место к устройству.
func (d Device) Bound() bool { return d.HWID != "" }
