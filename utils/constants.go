package utils

const (
	Version = "beta"

	// PluginName is the name of the CSI plugin.
	CSIPluginName = "carina.storage.io"
	// DefaultCSISocket is the default path of the CSI socket file.
	DefaultCSISocket     = "/tmp/csi/csi-provisioner.sock"
	LogicVolumeNamespace = "default"
	// LogicalVolumeFinalizer is the name of LogicalVolume finalizer
	LogicVolumeFinalizer = "carina.storage.io/logicvolume"
	// Default disk space hold
	DefaultReservedSpace = 10 << 30

	// DefaultSizeGb is the default size in GiB for  volumes (PVC or inline ephemeral volumes) w/o capacity requests.
	MinRequestSizeGb = 1
	// DeviceDiskKey is the key used in CSI volume create requests to specify a DeviceDiskKey support carina-vg-ssd carina-vg-hdd
	DeviceDiskKey = "carina.storage.io/disk"
	// k8s default key Device FileSystem eg. xfs ext4
	DeviceFileSystem = "csi.storage.k8s.io/fstype"
	// ResizeRequestedAtKey is the key of LogicalVolume that represents the timestamp of the resize request.
	ResizeRequestedAtKey = "carina.storage.io/resize-requested-at"
	// TopologyNodeKey is the key of topology that represents node name.
	TopologyNodeKey = "topology.carina.storage.io/node"
	// volume path
	VolumeDevicePath = "carina.storage.io/path"
	// volume node
	VolumeDeviceNode = "carina.storage.io/node"

	// Kubernetes label
	KubernetesHostName = "kubernetes.io/hostname"

	// device plugin
	DeviceCapacityKeyPrefix = "carina.storage.io/"
)

// EphemeralVolumeSizeKey is the key used to obtain ephemeral inline volume size
// from the volume context
const EphemeralVolumeSizeKey = "carina.storage.io/size"
