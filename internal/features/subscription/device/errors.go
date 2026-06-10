package device

import "errors"

var (
	ErrMaxDevices           = errors.New("max devices")
	ErrNoActiveDeviceAddons = errors.New("no active device addons")
)
