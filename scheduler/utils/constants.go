package utils

const (
	// PluginName is the name of the CSI plugin.
	CSIPluginName = "carina.storage.io"
	// storage class disk group
	DeviceDiskKey = "carina.storage.io/disk"
	// pv csi VolumeAttributes
	VolumeDeviceNode = "carina.storage.io/node"
	// device plugin
	DeviceCapacityKeyPrefix = "carina.storage.io/"
)
