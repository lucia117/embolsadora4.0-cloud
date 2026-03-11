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
// Both fields are optional (pointers indicate optionality).
type UpdateDeviceCommand struct {
	Name        *string
	Description *string
}
