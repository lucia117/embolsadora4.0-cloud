package edge_devices

// CreateDeviceCommand represents a request to create a new edge device.
type CreateDeviceCommand struct {
	Name             string
	MachineID        string
	EdgeType         string
	RaspberryBaseURL string
	Description      *string
	PLCAddress       *string
}

// UpdateDeviceCommand represents a request to update an edge device.
// All fields are optional (nil pointer = "leave unchanged").
// machineId and edgeType are immutable and not part of the command.
type UpdateDeviceCommand struct {
	Name             *string
	Description      *string
	RaspberryBaseURL *string
	PLCAddress       *string
}
