package utils

import corev1 "k8s.io/api/core/v1"

const (
	// LogicalVolumeFinalizer is the name of LogicalVolume finalizer
	LogicVolumeFinalizer = "carina.storage.io/logicvolume"
	// DefaultCSISocket is the default path of the CSI socket file.
	DefaultCSISocket = "/run/carina/csi-carina.sock"
	// Default space hold
	DefaultReservedSpace = 10 << 30
)

// CapacityKeyPrefix is the key prefix of Node annotation that represents VG free space.
const CapacityKeyPrefix = "capacity.carina.storage.io/"

// CapacityResource is the resource name of carina capacity.
const CapacityResource = corev1.ResourceName("carina.storage.io/capacity")

// PluginName is the name of the CSI plugin.
const PluginName = "carina.storage.io"

// TopologyNodeKey is the key of topology that represents node name.
const TopologyNodeKey = "topology.carina.storage.io/node"

// DeviceClassKey is the key used in CSI volume create requests to specify a device-class.
const DeviceClassKey = "carina.storage.io/device-class"

// ResizeRequestedAtKey is the key of LogicalVolume that represents the timestamp of the resize request.
const ResizeRequestedAtKey = "carina.storage.io/resize-requested-at"

// NodeFinalizer is the name of Node finalizer of carina
const NodeFinalizer = "carina.storage.io/node"

// PVCFinalizer is the name of PVC finalizer of carina
const PVCFinalizer = "carina.storage.io/pvc"

// EphemeralVolumeSizeKey is the key used to obtain ephemeral inline volume size
// from the volume context
const EphemeralVolumeSizeKey = "carina.storage.io/size"

// DefaultDeviceClassAnnotationName is the part of annotation name for the default device-class.
const DefaultDeviceClassAnnotationName = "00default"

// DefaultDeviceClassName is the name for the default device-class.
const DefaultDeviceClassName = ""

// DefaultSizeGb is the default size in GiB for  volumes (PVC or inline ephemeral volumes) w/o capacity requests.
const DefaultSizeGb = 1

// DefaultSize is DefaultSizeGb in bytes
const DefaultSize = DefaultSizeGb << 30
