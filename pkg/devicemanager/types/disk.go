package types

import (
	deviceManager "carina/pkg/devicemanager"
	"carina/utils/exec"
)

// Context for loading or applying the configuration state of a service.
type Context struct {

	// The implementation of executing a console command
	Executor exec.Executor
	// The root configuration directory used by services
	ConfigDir string

	// The local devices detected on the node
	Devices []*deviceManager.LocalDisk
}
