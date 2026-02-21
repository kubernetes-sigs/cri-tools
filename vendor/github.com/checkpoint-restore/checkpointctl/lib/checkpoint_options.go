// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"fmt"
)

// OptionType represents the type of a checkpoint option value
type OptionType int

const (
	OptionTypeBool OptionType = iota
	OptionTypeInt
	OptionTypeString
)

// CheckpointOption defines a known checkpoint option with its expected type
type CheckpointOption struct {
	Type OptionType
}

// SupportedCheckpointOption defines a supported checkpoint option with help text
// for use by tools that include this package.
type SupportedCheckpointOption struct {
	Type OptionType
	Help string
}

// SupportedCheckpointOptions lists all checkpoint options that are currently
// supported. Tools can use this to display help text or list available options.
var SupportedCheckpointOptions = map[string]SupportedCheckpointOption{
	"leave-running": {
		Type: OptionTypeBool,
		Help: "leave container(s) in running state after checkpointing",
	},
}

// CheckpointOptions represents the options from a CheckpointPodRequest
type CheckpointOptions struct {
	// LeaveRunning leaves the processes running in the container after checkpointing
	LeaveRunning bool `json:"leaveRunning,omitempty"`
	// TCPEstablished enables support for established TCP connections in the checkpoint
	TCPEstablished bool `json:"tcpEstablished,omitempty"`
	// GhostLimit limits max size of deleted file contents inside image
	GhostLimit int64 `json:"ghostLimit,omitempty"`
	// NetworkLock specifies the network locking/unlocking method
	NetworkLock string `json:"networkLock,omitempty"`
}

// knownCheckpointOptions defines all known checkpoint options and their expected types.
// It is built from SupportedCheckpointOptions plus additional options that are not yet
// supported but are defined here to ensure the parsing logic for all option types
// (bool, int, string) is tested and works.
var knownCheckpointOptions map[string]CheckpointOption

func init() {
	knownCheckpointOptions = make(map[string]CheckpointOption)

	// Include all supported options
	for name, opt := range SupportedCheckpointOptions {
		knownCheckpointOptions[name] = CheckpointOption{Type: opt.Type}
	}

	// Additional options for testing different option types (not yet supported)
	knownCheckpointOptions["tcp-established"] = CheckpointOption{Type: OptionTypeBool}
	knownCheckpointOptions["ghost-limit"] = CheckpointOption{Type: OptionTypeInt}
	knownCheckpointOptions["network-lock"] = CheckpointOption{Type: OptionTypeString}
}

// parseBool parses a string value as a boolean, accepting:
// yes, no, true, false, on, off, 0, 1 (case-insensitive)
func parseBool(value string) (bool, error) {
	switch value {
	case "yes", "Yes", "YES", "true", "True", "TRUE", "on", "On", "ON", "1":
		return true, nil
	case "no", "No", "NO", "false", "False", "FALSE", "off", "Off", "OFF", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %q (accepted: yes, no, true, false, on, off, 0, 1)", value)
	}
}

// ParseCheckpointOptions validates and parses checkpoint options from a map[string]string.
// It checks if the options are known and if their values match the expected types.
// Returns a CheckpointOptions struct and any validation errors encountered.
func ParseCheckpointOptions(options map[string]string) (*CheckpointOptions, error) {
	result := &CheckpointOptions{}
	var errs []string

	for key, value := range options {
		opt, known := knownCheckpointOptions[key]
		if !known {
			errs = append(errs, fmt.Sprintf("unknown option: %q", key))
			continue
		}

		switch opt.Type {
		case OptionTypeBool:
			boolVal, err := parseBool(value)
			if err != nil {
				errs = append(errs, fmt.Sprintf("option %q: %v", key, err))
				continue
			}
			switch key {
			case "leave-running":
				result.LeaveRunning = boolVal
			case "tcp-established":
				result.TCPEstablished = boolVal
			}

		case OptionTypeInt:
			var intVal int64
			if _, err := fmt.Sscanf(value, "%d", &intVal); err != nil {
				errs = append(errs, fmt.Sprintf("option %q: invalid integer value: %q", key, value))
				continue
			}
			switch key {
			case "ghost-limit":
				result.GhostLimit = intVal
			}

		case OptionTypeString:
			switch key {
			case "network-lock":
				result.NetworkLock = value
			}
		}
	}

	if len(errs) > 0 {
		return result, fmt.Errorf("validation errors: %v", errs)
	}

	return result, nil
}
