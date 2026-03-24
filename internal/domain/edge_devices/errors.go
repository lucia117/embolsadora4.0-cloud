package edge_devices

import "errors"

// ErrDeviceNotFound is returned when a device is not found.
// HTTP status: 404
var ErrDeviceNotFound = errors.New("device not found")

// ErrMachineIDConflict is returned when machineId already exists for the tenant.
// HTTP status: 409 with message "CONFLICT: machineId ya existe en el tenant"
var ErrMachineIDConflict = errors.New("machine_id already exists for this tenant")

// ErrDeviceDisabled is returned when an operation is attempted on a disabled device.
// HTTP status: 400 with code "EDGE_DEVICE_DISABLED"
var ErrDeviceDisabled = errors.New("device is disabled")

// ErrDeviceValidation is returned for domain validation errors.
// HTTP status: 400
var ErrDeviceValidation = errors.New("device validation error")
