package utils

const (
	// project
	Version = "beta"
	// PluginName is the name of the CSI plugin.
	CSIPluginName = "carina.storage.io"
	// DefaultCSISocket is the default path of the CSI socket file.
	DefaultCSISocket = "/tmp/csi/csi-provisioner.sock"
	// Default disk space hold
	DefaultReservedSpace = 10 << 30

	// logicVolume
	LogicVolumeNamespace = "default"
	// LogicalVolumeFinalizer is the name of LogicalVolume finalizer
	LogicVolumeFinalizer = "carina.storage.io/logicvolume"
	// ResizeRequestedAtKey is the key of LogicalVolume that represents the timestamp of the resize request.
	ResizeRequestedAtKey = "carina.storage.io/resize-requested-at"

	// storage class
	// DeviceDiskKey is the key used in CSI volume create requests to specify a DeviceDiskKey support carina-vg-ssd carina-vg-hdd
	DeviceDiskKey = "carina.storage.io/disk-type"
	// k8s default key Device FileSystem eg. xfs ext4
	DeviceFileSystem = "csi.storage.k8s.io/fstype"

	// pvc
	// default size in GiB for volumes (PVC or inline ephemeral volumes) w/o capacity requests.
	MinRequestSizeGb = 1
	// This annotation is added to a PVC that has been triggered by scheduler to
	// be dynamically provisioned. Its value is the name of the selected node.
	AnnSelectedNode = "volume.kubernetes.io/selected-node"

	// pv csi VolumeAttributes
	VolumeDevicePath  = "carina.storage.io/path"
	VolumeDeviceNode  = "carina.storage.io/node"
	VolumeDeviceMajor = "carina.storage.io/major"
	VolumeDeviceMinor = "carina.storage.io/minor"



	// topology
	// TopologyZoneKey is the key of topology that represents zone name.
	TopologyNodeKey = "topology.carina.storage.io/node"

	// device plugin
	DeviceCapacityKeyPrefix = "carina.storage.io/"
	// support disk type
	DeviceVGSSD = "carina-vg-ssd"
	DeviceVGHDD = "carina-vg-hdd"

	// custom schedule
	CarinaSchedule = "carina-scheduler"
)
